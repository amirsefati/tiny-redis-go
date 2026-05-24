package store

import (
	"fmt"
	"testing"
)

func BenchmarkStoreSet(b *testing.B) {
	db := New()
	payload := []byte("benchmark-value")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		db.Set(fmt.Sprintf("key:%d", i), payload)
	}
}

func BenchmarkStoreGet(b *testing.B) {
	db := New()
	for i := 0; i < 1024; i++ {
		db.Set(fmt.Sprintf("key:%d", i), []byte("benchmark-value"))
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, ok := db.Get(fmt.Sprintf("key:%d", i%1024)); !ok {
			b.Fatal("expected key to exist")
		}
	}
}

func BenchmarkStoreMixedWorkload(b *testing.B) {
	db := New()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		key := fmt.Sprintf("key:%d", i%2048)
		if i%3 == 0 {
			db.Set(key, []byte("benchmark-value"))
			continue
		}
		db.Get(key)
	}
}
