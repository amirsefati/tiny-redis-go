package store

import (
	"sync"
	"sync/atomic"
	"time"
)

type ValueType string

const (
	ValueTypeString ValueType = "string"
)

type Value struct {
	Type      ValueType
	Data      []byte
	CreatedAt int64
	UpdatedAt int64
	Version   uint64
}

type Stats struct {
	KeyCount     int
	CommandCount uint64
}

type Store struct {
	mu           sync.RWMutex
	values       map[string]Value
	commandCount atomic.Uint64
}

func New() *Store {
	return &Store{
		values: make(map[string]Value),
	}
}

func (s *Store) Set(key string, data []byte) Value {
	now := time.Now().UnixMilli()
	owned := cloneBytes(data)

	s.mu.Lock()
	defer s.mu.Unlock()

	current, exists := s.values[key]
	value := Value{
		Type:      ValueTypeString,
		Data:      owned,
		CreatedAt: now,
		UpdatedAt: now,
		Version:   1,
	}
	if exists {
		value.CreatedAt = current.CreatedAt
		value.Version = current.Version + 1
	}

	s.values[key] = value
	return cloneValue(value)
}

func (s *Store) Get(key string) (Value, bool) {
	s.mu.RLock()
	value, ok := s.values[key]
	s.mu.RUnlock()
	if !ok {
		return Value{}, false
	}
	return cloneValue(value), true
}

func (s *Store) Del(keys ...string) int {
	s.mu.Lock()
	defer s.mu.Unlock()

	deleted := 0
	for _, key := range keys {
		if _, ok := s.values[key]; ok {
			delete(s.values, key)
			deleted++
		}
	}

	return deleted
}

func (s *Store) Exists(keys ...string) int {
	s.mu.RLock()
	defer s.mu.RUnlock()

	count := 0
	for _, key := range keys {
		if _, ok := s.values[key]; ok {
			count++
		}
	}

	return count
}

func (s *Store) DBSize() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.values)
}

func (s *Store) RecordCommand() uint64 {
	return s.commandCount.Add(1)
}

func (s *Store) Stats() Stats {
	s.mu.RLock()
	keyCount := len(s.values)
	s.mu.RUnlock()

	return Stats{
		KeyCount:     keyCount,
		CommandCount: s.commandCount.Load(),
	}
}

func cloneValue(value Value) Value {
	value.Data = cloneBytes(value.Data)
	return value
}

func cloneBytes(data []byte) []byte {
	if data == nil {
		return nil
	}
	clone := make([]byte, len(data))
	copy(clone, data)
	return clone
}
