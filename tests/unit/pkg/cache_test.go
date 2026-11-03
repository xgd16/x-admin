package pkg_test

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"x-admin/internal/pkg/cache"
)

func TestMemoryStore(t *testing.T) {
	ctx := context.Background()
	m := cache.NewMemory()
	require.NoError(t, m.Set(ctx, "k", "v", time.Minute))
	v, err := m.Get(ctx, "k")
	require.NoError(t, err)
	assert.Equal(t, "v", v)

	ok, err := m.Exists(ctx, "k")
	require.NoError(t, err)
	assert.True(t, ok)

	_, err = m.Get(ctx, "missing")
	require.ErrorIs(t, err, cache.ErrNotFound)

	require.NoError(t, m.Set(ctx, "short", "v", 10*time.Millisecond))
	time.Sleep(20 * time.Millisecond)
	_, err = m.Get(ctx, "short")
	require.ErrorIs(t, err, cache.ErrNotFound)
	ok, _ = m.Exists(ctx, "short")
	assert.False(t, ok)

	n, err := m.Incr(ctx, "counter", time.Minute)
	require.NoError(t, err)
	assert.Equal(t, int64(1), n)
	n, err = m.Incr(ctx, "counter", time.Minute)
	require.NoError(t, err)
	assert.Equal(t, int64(2), n)

	require.NoError(t, m.Set(ctx, "take", "one", time.Minute))
	v, err = m.Take(ctx, "take")
	require.NoError(t, err)
	assert.Equal(t, "one", v)
	_, err = m.Take(ctx, "take")
	require.ErrorIs(t, err, cache.ErrNotFound)

	require.NoError(t, m.Del(ctx, "counter"))
	ok, _ = m.Exists(ctx, "counter")
	assert.False(t, ok)

	require.NoError(t, m.Ping(ctx))
	require.NoError(t, m.Close())
}

func TestMemoryStoreExpiredCounterRestarts(t *testing.T) {
	ctx := context.Background()
	m := cache.NewMemory()
	n, _ := m.Incr(ctx, "c", 5*time.Millisecond)
	assert.Equal(t, int64(1), n)
	time.Sleep(10 * time.Millisecond)
	n, _ = m.Incr(ctx, "c", time.Minute)
	assert.Equal(t, int64(1), n)
}

func TestRedisStore(t *testing.T) {
	ctx := context.Background()
	mr := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	store := cache.NewRedis(client)

	require.NoError(t, store.Set(ctx, "k", "v", time.Minute))
	v, err := store.Get(ctx, "k")
	require.NoError(t, err)
	assert.Equal(t, "v", v)

	_, err = store.Get(ctx, "missing")
	require.ErrorIs(t, err, cache.ErrNotFound)

	ok, err := store.Exists(ctx, "k")
	require.NoError(t, err)
	assert.True(t, ok)

	n, err := store.Incr(ctx, "counter", time.Minute)
	require.NoError(t, err)
	assert.Equal(t, int64(1), n)
	n, err = store.Incr(ctx, "counter", time.Minute)
	require.NoError(t, err)
	assert.Equal(t, int64(2), n)
	assert.Greater(t, mr.TTL("counter"), time.Duration(0))

	require.NoError(t, store.Set(ctx, "take", "one", time.Minute))
	v, err = store.Take(ctx, "take")
	require.NoError(t, err)
	assert.Equal(t, "one", v)
	_, err = store.Take(ctx, "take")
	require.ErrorIs(t, err, cache.ErrNotFound)

	require.NoError(t, store.Del(ctx, "k", "counter"))
	ok, _ = store.Exists(ctx, "k")
	assert.False(t, ok)

	require.NoError(t, store.Ping(ctx))
	require.NoError(t, store.Close())
}
