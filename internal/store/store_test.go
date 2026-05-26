package store

import (
	"fmt"
	"sync"
	"testing"
	"time"
)

func TestStoreSetCopiesInput(t *testing.T) {
	t.Parallel()

	db := New()
	payload := []byte("hello")
	db.Set("greeting", payload)

	payload[0] = 'j'

	value, ok := db.Get("greeting")
	if !ok {
		t.Fatal("expected key to exist")
	}
	if got := string(value.Data); got != "hello" {
		t.Fatalf("unexpected stored value: got %q want %q", got, "hello")
	}
}

func TestStoreGetReturnsCopy(t *testing.T) {
	t.Parallel()

	db := New()
	db.Set("greeting", []byte("hello"))

	value, ok := db.Get("greeting")
	if !ok {
		t.Fatal("expected key to exist")
	}

	value.Data[0] = 'j'

	again, ok := db.Get("greeting")
	if !ok {
		t.Fatal("expected key to exist")
	}
	if got := string(again.Data); got != "hello" {
		t.Fatalf("unexpected stored value after mutation: got %q want %q", got, "hello")
	}
}

func TestStoreSetPreservesMetadataOnUpdate(t *testing.T) {
	t.Parallel()

	db := New()
	first := db.Set("versioned", []byte("one"))
	second := db.Set("versioned", []byte("two"))

	if second.CreatedAt != first.CreatedAt {
		t.Fatalf("expected CreatedAt to be preserved: got %d want %d", second.CreatedAt, first.CreatedAt)
	}
	if second.Version != first.Version+1 {
		t.Fatalf("expected version increment: got %d want %d", second.Version, first.Version+1)
	}
	if second.UpdatedAt < first.UpdatedAt {
		t.Fatalf("expected UpdatedAt to move forward: got %d want >= %d", second.UpdatedAt, first.UpdatedAt)
	}
}

func TestStoreSetClearsExistingExpiration(t *testing.T) {
	t.Parallel()

	now := int64(1_000)
	db := New(WithClock(func() int64 { return now }))

	db.SetWithTTL("session", []byte("v1"), 5*time.Second)
	db.Set("session", []byte("v2"))

	now += 10_000

	value, ok := db.Get("session")
	if !ok {
		t.Fatal("expected key to exist after plain SET cleared the TTL")
	}
	if got := string(value.Data); got != "v2" {
		t.Fatalf("unexpected value: got %q want %q", got, "v2")
	}
	if got := db.TTL("session"); got != -1 {
		t.Fatalf("unexpected TTL after plain SET: got %d want -1", got)
	}
}

func TestStoreDeleteExistsAndStats(t *testing.T) {
	t.Parallel()

	db := New()
	db.Set("a", []byte("1"))
	db.Set("b", []byte("2"))
	db.RecordCommand()
	db.RecordCommand()

	if got := db.Exists("a", "missing", "b"); got != 2 {
		t.Fatalf("unexpected EXISTS count: got %d want 2", got)
	}
	if got := db.Del("b", "missing"); got != 1 {
		t.Fatalf("unexpected DEL count: got %d want 1", got)
	}
	if got := db.DBSize(); got != 1 {
		t.Fatalf("unexpected DBSIZE: got %d want 1", got)
	}

	stats := db.Stats()
	if stats.KeyCount != 1 {
		t.Fatalf("unexpected stats key count: got %d want 1", stats.KeyCount)
	}
	if stats.CommandCount != 2 {
		t.Fatalf("unexpected stats command count: got %d want 2", stats.CommandCount)
	}
}

func TestStoreExpirePersistAndTTL(t *testing.T) {
	t.Parallel()

	now := int64(10_000)
	db := New(WithClock(func() int64 { return now }))

	db.Set("job", []byte("queued"))

	if got := db.TTL("job"); got != -1 {
		t.Fatalf("unexpected TTL for key without expiration: got %d want -1", got)
	}
	if got := db.TTL("missing"); got != -2 {
		t.Fatalf("unexpected TTL for missing key: got %d want -2", got)
	}
	if got := db.Expire("job", 3*time.Second); got != 1 {
		t.Fatalf("unexpected EXPIRE result: got %d want 1", got)
	}
	if got := db.TTL("job"); got != 3 {
		t.Fatalf("unexpected TTL after EXPIRE: got %d want 3", got)
	}
	if got := db.Persist("job"); got != 1 {
		t.Fatalf("unexpected PERSIST result: got %d want 1", got)
	}
	if got := db.TTL("job"); got != -1 {
		t.Fatalf("unexpected TTL after PERSIST: got %d want -1", got)
	}
	if got := db.Persist("job"); got != 0 {
		t.Fatalf("unexpected PERSIST result without expiration: got %d want 0", got)
	}
	if got := db.Expire("missing", time.Second); got != 0 {
		t.Fatalf("unexpected EXPIRE result for missing key: got %d want 0", got)
	}
}

func TestStoreLazyExpirationOnGetAndPTTL(t *testing.T) {
	t.Parallel()

	now := int64(50_000)
	db := New(WithClock(func() int64 { return now }))

	db.SetWithTTL("token", []byte("abc"), 1500*time.Millisecond)

	if got := db.PTTL("token"); got != 1500 {
		t.Fatalf("unexpected initial PTTL: got %d want 1500", got)
	}

	now += 1200
	if got := db.TTL("token"); got != 0 {
		t.Fatalf("unexpected TTL after partial advance: got %d want 0", got)
	}

	now += 400
	if _, ok := db.Get("token"); ok {
		t.Fatal("expected key to be lazily deleted on GET after expiration")
	}
	if got := db.PTTL("token"); got != -2 {
		t.Fatalf("unexpected PTTL after expiration: got %d want -2", got)
	}

	stats := db.Stats()
	if stats.ExpiredKeysDeleted != 1 {
		t.Fatalf("unexpected expired delete count: got %d want 1", stats.ExpiredKeysDeleted)
	}
}

func TestStoreActiveExpirationWorker(t *testing.T) {
	now := int64(100_000)
	db := New(
		WithClock(func() int64 { return now }),
		WithActiveExpiration(5*time.Millisecond, 2),
	)
	defer db.Close()

	db.SetWithTTL("a", []byte("1"), time.Second)
	db.SetWithTTL("b", []byte("2"), time.Second)
	db.SetWithTTL("c", []byte("3"), time.Second)

	now += 1_500

	deadline := time.Now().Add(250 * time.Millisecond)
	for time.Now().Before(deadline) {
		if db.DBSize() == 0 {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}

	if got := db.DBSize(); got != 0 {
		t.Fatalf("expected active expiration to remove all keys, got DB size %d", got)
	}

	stats := db.Stats()
	if stats.ActiveCleanupRuns == 0 {
		t.Fatal("expected active cleanup worker to run at least once")
	}
	if stats.ExpiredKeysDeleted != 3 {
		t.Fatalf("unexpected expired delete count: got %d want 3", stats.ExpiredKeysDeleted)
	}
}

func TestStoreConcurrentSetGet(t *testing.T) {
	t.Parallel()

	db := New()

	const workers = 16
	const iterations = 200

	var wg sync.WaitGroup
	wg.Add(workers)

	for worker := 0; worker < workers; worker++ {
		go func(worker int) {
			defer wg.Done()
			for i := 0; i < iterations; i++ {
				key := fmt.Sprintf("key:%d:%d", worker, i)
				value := []byte(fmt.Sprintf("value:%d:%d", worker, i))
				db.Set(key, value)

				stored, ok := db.Get(key)
				if !ok {
					t.Errorf("expected key %q to exist", key)
					return
				}
				if got := string(stored.Data); got != string(value) {
					t.Errorf("unexpected value for %q: got %q want %q", key, got, string(value))
					return
				}
			}
		}(worker)
	}

	wg.Wait()

	if got := db.DBSize(); got != workers*iterations {
		t.Fatalf("unexpected DB size: got %d want %d", got, workers*iterations)
	}
}
