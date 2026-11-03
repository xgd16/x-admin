package cache

import (
	"context"
	"errors"
	"strconv"
	"sync"
	"time"
)

var ErrNotFound = errors.New("cache: key not found")

// Store is the small Redis surface the application depends on. Keeping it
// narrow makes services testable with the in-memory implementation.
type Store interface {
	Set(ctx context.Context, key, value string, ttl time.Duration) error
	Get(ctx context.Context, key string) (string, error)
	// Take atomically reads and deletes a key (refresh token rotation).
	Take(ctx context.Context, key string) (string, error)
	Del(ctx context.Context, keys ...string) error
	Exists(ctx context.Context, key string) (bool, error)
	Incr(ctx context.Context, key string, ttl time.Duration) (int64, error)
	Ping(ctx context.Context) error
	Close() error
}

type entry struct {
	value     string
	expiresAt time.Time
}

// Memory is a concurrency-safe Store used by tests and by `cache.driver=memory`.
type Memory struct {
	mu   sync.Mutex
	data map[string]entry
	now  func() time.Time
}

func NewMemory() *Memory {
	return &Memory{data: make(map[string]entry), now: time.Now}
}

func (m *Memory) expired(e entry) bool {
	return !e.expiresAt.IsZero() && !m.now().Before(e.expiresAt)
}

func (m *Memory) Set(_ context.Context, key, value string, ttl time.Duration) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	e := entry{value: value}
	if ttl > 0 {
		e.expiresAt = m.now().Add(ttl)
	}
	m.data[key] = e
	return nil
}

func (m *Memory) Get(_ context.Context, key string) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	e, ok := m.data[key]
	if !ok || m.expired(e) {
		delete(m.data, key)
		return "", ErrNotFound
	}
	return e.value, nil
}

func (m *Memory) Take(_ context.Context, key string) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	e, ok := m.data[key]
	if !ok || m.expired(e) {
		delete(m.data, key)
		return "", ErrNotFound
	}
	delete(m.data, key)
	return e.value, nil
}

func (m *Memory) Del(_ context.Context, keys ...string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, key := range keys {
		delete(m.data, key)
	}
	return nil
}

func (m *Memory) Exists(_ context.Context, key string) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	e, ok := m.data[key]
	if !ok || m.expired(e) {
		delete(m.data, key)
		return false, nil
	}
	return true, nil
}

func (m *Memory) Incr(_ context.Context, key string, ttl time.Duration) (int64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	e, ok := m.data[key]
	if !ok || m.expired(e) {
		e = entry{value: "1"}
		if ttl > 0 {
			e.expiresAt = m.now().Add(ttl)
		}
		m.data[key] = e
		return 1, nil
	}
	n := int64(1)
	if v, err := parseInt(e.value); err == nil {
		n = v + 1
	}
	e.value = formatInt(n)
	m.data[key] = e
	return n, nil
}

func (m *Memory) Ping(context.Context) error { return nil }
func (m *Memory) Close() error               { return nil }

func parseInt(s string) (int64, error) { return strconv.ParseInt(s, 10, 64) }
func formatInt(n int64) string         { return strconv.FormatInt(n, 10) }
