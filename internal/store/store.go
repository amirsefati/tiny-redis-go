package store

import (
	"sync"
	"sync/atomic"
	"time"
)

type ValueType string

const (
	ValueTypeString       ValueType = "string"
	NoExpirationUnixMilli int64     = 0
)

type Value struct {
	Type      ValueType
	Data      []byte
	CreatedAt int64
	UpdatedAt int64
	Version   uint64
	ExpiresAt int64
}

func (v Value) HasExpiration() bool {
	return v.ExpiresAt != NoExpirationUnixMilli
}

func (v Value) IsExpired(now int64) bool {
	return v.HasExpiration() && now >= v.ExpiresAt
}

type Stats struct {
	KeyCount            int
	CommandCount        uint64
	ExpiredKeysDeleted  uint64
	ActiveCleanupRuns   uint64
	ActiveCleanupSample int
}

type Store struct {
	mu                   sync.RWMutex
	values               map[string]Value
	commandCount         atomic.Uint64
	expiredKeysDeleted   atomic.Uint64
	activeCleanupRuns    atomic.Uint64
	now                  func() int64
	activeCleanupEvery   time.Duration
	activeCleanupSample  int
	stopActiveCleanupCh  chan struct{}
	stopActiveCleanupMux sync.Once
}

type Option func(*Store)

func WithClock(now func() int64) Option {
	return func(s *Store) {
		if now != nil {
			s.now = now
		}
	}
}

func WithActiveExpiration(interval time.Duration, sampleSize int) Option {
	return func(s *Store) {
		if interval > 0 {
			s.activeCleanupEvery = interval
		}
		if sampleSize > 0 {
			s.activeCleanupSample = sampleSize
		}
	}
}

func New(options ...Option) *Store {
	s := &Store{
		values: make(map[string]Value),
		now: func() int64 {
			return time.Now().UnixMilli()
		},
	}

	for _, option := range options {
		option(s)
	}

	if s.activeCleanupEvery > 0 && s.activeCleanupSample > 0 {
		s.stopActiveCleanupCh = make(chan struct{})
		go s.runActiveExpiration()
	}

	return s
}

func (s *Store) Set(key string, data []byte) Value {
	return s.set(key, data, NoExpirationUnixMilli)
}

func (s *Store) SetWithTTL(key string, data []byte, ttl time.Duration) Value {
	now := s.now()
	expiresAt := now + ttl.Milliseconds()
	return s.set(key, data, expiresAt)
}

func (s *Store) set(key string, data []byte, expiresAt int64) Value {
	now := s.now()
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
		ExpiresAt: expiresAt,
	}
	if exists {
		value.CreatedAt = current.CreatedAt
		value.Version = current.Version + 1
	}

	s.values[key] = value
	return cloneValue(value)
}

func (s *Store) Get(key string) (Value, bool) {
	now := s.now()

	s.mu.RLock()
	value, ok := s.values[key]
	s.mu.RUnlock()
	if !ok {
		return Value{}, false
	}
	if value.IsExpired(now) {
		s.deleteIfExpired(key, now)
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
	now := s.now()

	s.mu.RLock()
	count := 0
	expired := make([]string, 0)
	for _, key := range keys {
		value, ok := s.values[key]
		if !ok {
			continue
		}
		if value.IsExpired(now) {
			expired = append(expired, key)
			continue
		}
		count++
	}
	s.mu.RUnlock()

	for _, key := range expired {
		if !s.deleteIfExpired(key, now) && s.isLiveKey(key, now) {
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
		KeyCount:            keyCount,
		CommandCount:        s.commandCount.Load(),
		ExpiredKeysDeleted:  s.expiredKeysDeleted.Load(),
		ActiveCleanupRuns:   s.activeCleanupRuns.Load(),
		ActiveCleanupSample: s.activeCleanupSample,
	}
}

func (s *Store) Expire(key string, ttl time.Duration) int {
	now := s.now()

	s.mu.Lock()
	defer s.mu.Unlock()

	value, ok := s.values[key]
	if !ok {
		return 0
	}
	if value.IsExpired(now) {
		delete(s.values, key)
		s.expiredKeysDeleted.Add(1)
		return 0
	}

	expiresAt := now + ttl.Milliseconds()
	if expiresAt <= now {
		delete(s.values, key)
		s.expiredKeysDeleted.Add(1)
		return 1
	}

	value.ExpiresAt = expiresAt
	value.UpdatedAt = now
	value.Version++
	s.values[key] = value
	return 1
}

func (s *Store) TTL(key string) int64 {
	pttl := s.PTTL(key)
	if pttl < 0 {
		return pttl
	}
	return pttl / 1000
}

func (s *Store) PTTL(key string) int64 {
	now := s.now()

	s.mu.RLock()
	value, ok := s.values[key]
	s.mu.RUnlock()
	if !ok {
		return -2
	}
	if value.IsExpired(now) {
		s.deleteIfExpired(key, now)
		return -2
	}
	if !value.HasExpiration() {
		return -1
	}

	remaining := value.ExpiresAt - now
	if remaining < 0 {
		s.deleteIfExpired(key, now)
		return -2
	}
	return remaining
}

func (s *Store) Persist(key string) int {
	now := s.now()

	s.mu.Lock()
	defer s.mu.Unlock()

	value, ok := s.values[key]
	if !ok {
		return 0
	}
	if value.IsExpired(now) {
		delete(s.values, key)
		s.expiredKeysDeleted.Add(1)
		return 0
	}
	if !value.HasExpiration() {
		return 0
	}

	value.ExpiresAt = NoExpirationUnixMilli
	value.UpdatedAt = now
	value.Version++
	s.values[key] = value
	return 1
}

func (s *Store) Close() {
	if s.stopActiveCleanupCh == nil {
		return
	}

	s.stopActiveCleanupMux.Do(func() {
		close(s.stopActiveCleanupCh)
	})
}

func (s *Store) runActiveExpiration() {
	ticker := time.NewTicker(s.activeCleanupEvery)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			s.activeCleanupRuns.Add(1)
			s.runActiveExpirationCycle()
		case <-s.stopActiveCleanupCh:
			return
		}
	}
}

func (s *Store) runActiveExpirationCycle() int {
	now := s.now()

	s.mu.RLock()
	sampleKeys := make([]string, 0, s.activeCleanupSample)
	for key := range s.values {
		sampleKeys = append(sampleKeys, key)
		if len(sampleKeys) == s.activeCleanupSample {
			break
		}
	}
	s.mu.RUnlock()

	deleted := 0
	for _, key := range sampleKeys {
		if s.deleteIfExpired(key, now) {
			deleted++
		}
	}
	return deleted
}

func (s *Store) deleteIfExpired(key string, now int64) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	value, ok := s.values[key]
	if !ok || !value.IsExpired(now) {
		return false
	}

	delete(s.values, key)
	s.expiredKeysDeleted.Add(1)
	return true
}

func (s *Store) isLiveKey(key string, now int64) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	value, ok := s.values[key]
	return ok && !value.IsExpired(now)
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
