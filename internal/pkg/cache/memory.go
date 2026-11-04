// Package cache 定义应用依赖的最小缓存接口及两种实现。
package cache

import (
	"context"
	"errors"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

// ErrNotFound 表示缓存键不存在或已过期。
var ErrNotFound = errors.New("cache: key not found")

// Store 是应用依赖的 Redis 能力子集。
// 接口保持狭窄，便于用内存实现做单元测试。
type Store interface {
	Set(ctx context.Context, key, value string, ttl time.Duration) error
	Get(ctx context.Context, key string) (string, error)
	// Take 原子地读取并删除键，用于 refresh token 的一次性轮换。
	Take(ctx context.Context, key string) (string, error)
	Del(ctx context.Context, keys ...string) error
	Exists(ctx context.Context, key string) (bool, error)
	// Keys 返回匹配前缀且未过期的键，供在线会话列表等场景使用。
	Keys(ctx context.Context, prefix string) ([]string, error)
	// SetNX 仅在键不存在时写入，用于分布式锁。
	SetNX(ctx context.Context, key, value string, ttl time.Duration) (bool, error)
	Incr(ctx context.Context, key string, ttl time.Duration) (int64, error)
	Ping(ctx context.Context) error
	Close() error
}

// entry 是内存缓存中的一条记录；expiresAt 为零值表示永不过期。
type entry struct {
	value     string
	expiresAt time.Time
}

// Memory 是并发安全的进程内缓存，用于测试或 cache.driver=memory。
type Memory struct {
	mu   sync.Mutex
	data map[string]entry
	now  func() time.Time
}

// NewMemory 创建内存缓存。
func NewMemory() *Memory {
	return &Memory{data: make(map[string]entry), now: time.Now}
}

// expired 判断记录是否已过期。
func (m *Memory) expired(e entry) bool {
	return !e.expiresAt.IsZero() && !m.now().Before(e.expiresAt)
}

// Set 写入键值，ttl <= 0 表示不过期。
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

// Get 读取键值；不存在或过期时返回 ErrNotFound 并顺手清理。
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

// Take 原子地读取并删除键。
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

// Del 删除一个或多个键。
func (m *Memory) Del(_ context.Context, keys ...string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, key := range keys {
		delete(m.data, key)
	}
	return nil
}

// Exists 判断键是否存在且未过期。
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

// Keys 返回匹配前缀且未过期的键。内存实现直接遍历，数据量受会话数限制。
func (m *Memory) Keys(_ context.Context, prefix string) ([]string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	keys := make([]string, 0)
	for key, e := range m.data {
		if m.expired(e) {
			delete(m.data, key)
			continue
		}
		if strings.HasPrefix(key, prefix) {
			keys = append(keys, key)
		}
	}
	sort.Strings(keys)
	return keys, nil
}

// SetNX 仅在键不存在或已过期时写入，返回是否成功获取。
func (m *Memory) SetNX(_ context.Context, key, value string, ttl time.Duration) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if e, ok := m.data[key]; ok && !m.expired(e) {
		return false, nil
	}
	e := entry{value: value}
	if ttl > 0 {
		e.expiresAt = m.now().Add(ttl)
	}
	m.data[key] = e
	return true, nil
}

// Incr 原子自增。键不存在时从 1 开始并按 ttl 设置过期时间；
// 已存在的键保持原过期时间，从而形成固定计数窗口。
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

// Ping 对内存实现始终成功。
func (m *Memory) Ping(context.Context) error { return nil }

// Close 对内存实现无需释放资源。
func (m *Memory) Close() error { return nil }

func parseInt(s string) (int64, error) { return strconv.ParseInt(s, 10, 64) }
func formatInt(n int64) string         { return strconv.FormatInt(n, 10) }
