package command

import (
	"testing"

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
