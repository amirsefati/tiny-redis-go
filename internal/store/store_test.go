package store

import (
	"fmt"
	"sync"
	"testing"
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
