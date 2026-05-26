package command

import (
	"testing"
	"time"

	"tiny-redis-go/internal/resp"
	"tiny-redis-go/internal/store"
)

func TestRegistryStoreCommands(t *testing.T) {
	t.Parallel()

	db := store.New()
	registry := NewRegistry(db)

	if got := registry.Dispatch([]string{"SET", "name", "redis"}); got != resp.SimpleString("OK") {
		t.Fatalf("unexpected SET response: %#v", got)
	}

	getResponse := registry.Dispatch([]string{"GET", "name"})
	bulk, ok := getResponse.(resp.BulkString)
	if !ok {
		t.Fatalf("expected bulk string, got %T", getResponse)
	}
	if bulk.Value != "redis" || bulk.Null {
		t.Fatalf("unexpected GET response: %#v", bulk)
	}

	if got := registry.Dispatch([]string{"EXISTS", "name", "missing"}); got != resp.Integer(1) {
		t.Fatalf("unexpected EXISTS response: %#v", got)
	}
	if got := registry.Dispatch([]string{"DBSIZE"}); got != resp.Integer(1) {
		t.Fatalf("unexpected DBSIZE response: %#v", got)
	}
	if got := registry.Dispatch([]string{"DEL", "name", "missing"}); got != resp.Integer(1) {
		t.Fatalf("unexpected DEL response: %#v", got)
	}

	missing := registry.Dispatch([]string{"GET", "name"})
	bulk, ok = missing.(resp.BulkString)
	if !ok {
		t.Fatalf("expected bulk string, got %T", missing)
	}
	if !bulk.Null {
		t.Fatalf("expected null bulk string for missing key, got %#v", bulk)
	}
}

func TestRegistryValidationAndUnknownCommand(t *testing.T) {
	t.Parallel()

	registry := NewRegistry(store.New())

	tests := []struct {
		name    string
		command []string
		want    resp.ErrorString
	}{
		{
			name:    "set wrong arity",
			command: []string{"SET", "key"},
			want:    resp.ErrorString("ERR wrong number of arguments for 'set' command"),
		},
		{
			name:    "get empty key",
			command: []string{"GET", ""},
			want:    resp.ErrorString("ERR empty key"),
		},
		{
			name:    "unknown command",
			command: []string{"NOPE"},
			want:    resp.ErrorString("ERR unknown command 'nope'"),
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := registry.Dispatch(tt.command); got != tt.want {
				t.Fatalf("unexpected response: got %#v want %#v", got, tt.want)
			}
		})
	}
}

func TestRegistryCommandCount(t *testing.T) {
	t.Parallel()

	db := store.New()
	registry := NewRegistry(db)

	registry.Dispatch([]string{"PING"})
	registry.Dispatch([]string{"SET", "a", "1"})
	registry.Dispatch([]string{"GET", "a"})

	stats := db.Stats()
	if stats.CommandCount != 3 {
		t.Fatalf("unexpected command count: got %d want 3", stats.CommandCount)
	}
}

func TestRegistryTTLCommands(t *testing.T) {
	now := int64(1_000)
	db := store.New(store.WithClock(func() int64 { return now }))
	registry := NewRegistry(db)

	if got := registry.Dispatch([]string{"SET", "session", "abc", "PX", "1500"}); got != resp.SimpleString("OK") {
		t.Fatalf("unexpected SET PX response: %#v", got)
	}
	if got := registry.Dispatch([]string{"PTTL", "session"}); got != resp.Integer(1500) {
		t.Fatalf("unexpected PTTL response: %#v", got)
	}
	if got := registry.Dispatch([]string{"PERSIST", "session"}); got != resp.Integer(1) {
		t.Fatalf("unexpected PERSIST response: %#v", got)
	}
	if got := registry.Dispatch([]string{"TTL", "session"}); got != resp.Integer(-1) {
		t.Fatalf("unexpected TTL response after PERSIST: %#v", got)
	}
	if got := registry.Dispatch([]string{"EXPIRE", "session", "2"}); got != resp.Integer(1) {
		t.Fatalf("unexpected EXPIRE response: %#v", got)
	}

	now += 2_100

	missing := registry.Dispatch([]string{"GET", "session"})
	bulk, ok := missing.(resp.BulkString)
	if !ok || !bulk.Null {
		t.Fatalf("expected null bulk string after expiration, got %#v", missing)
	}
	if got := registry.Dispatch([]string{"TTL", "session"}); got != resp.Integer(-2) {
		t.Fatalf("unexpected TTL response for expired key: %#v", got)
	}
	if got := registry.Dispatch([]string{"EXPIRE", "missing", "1"}); got != resp.Integer(0) {
		t.Fatalf("unexpected EXPIRE response for missing key: %#v", got)
	}
}

func TestRegistrySetExpirationValidation(t *testing.T) {
	db := store.New()
	registry := NewRegistry(db)

	tests := []struct {
		name    string
		command []string
		want    resp.ErrorString
	}{
		{
			name:    "set invalid option",
			command: []string{"SET", "k", "v", "NX", "1"},
			want:    resp.ErrorString("ERR syntax error"),
		},
		{
			name:    "set invalid expire time",
			command: []string{"SET", "k", "v", "EX", "0"},
			want:    resp.ErrorString("ERR invalid expire time in 'set' command"),
		},
		{
			name:    "pexpire invalid integer",
			command: []string{"PEXPIRE", "k", "abc"},
			want:    resp.ErrorString("ERR value is not an integer or out of range"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := registry.Dispatch(tt.command); got != tt.want {
				t.Fatalf("unexpected response: got %#v want %#v", got, tt.want)
			}
		})
	}

	if got := db.Expire("k", time.Second); got != 0 {
		t.Fatalf("unexpected direct store expire result for missing key: got %d want 0", got)
	}
}
