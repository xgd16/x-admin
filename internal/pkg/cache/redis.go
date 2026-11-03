package cache

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"
)

// Redis implements Store on top of go-redis.
type Redis struct {
	client *redis.Client
	incr   *redis.Script
}

func NewRedis(client *redis.Client) *Redis {
	return &Redis{
		client: client,
		incr: redis.NewScript(`
local v = redis.call('INCR', KEYS[1])
if v == 1 and tonumber(ARGV[1]) > 0 then
  redis.call('PEXPIRE', KEYS[1], ARGV[1])
end
return v
`),
	}
}

func (r *Redis) Set(ctx context.Context, key, value string, ttl time.Duration) error {
	return r.client.Set(ctx, key, value, ttl).Err()
}

func (r *Redis) Get(ctx context.Context, key string) (string, error) {
	v, err := r.client.Get(ctx, key).Result()
	if err == redis.Nil {
		return "", ErrNotFound
	}
	return v, err
}

func (r *Redis) Del(ctx context.Context, keys ...string) error {
	if len(keys) == 0 {
		return nil
	}
	return r.client.Del(ctx, keys...).Err()
}

func (r *Redis) Exists(ctx context.Context, key string) (bool, error) {
	n, err := r.client.Exists(ctx, key).Result()
	return n > 0, err
}

func (r *Redis) Incr(ctx context.Context, key string, ttl time.Duration) (int64, error) {
	return r.incr.Run(ctx, r.client, []string{key}, ttl.Milliseconds()).Int64()
}

func (r *Redis) Ping(ctx context.Context) error { return r.client.Ping(ctx).Err() }
func (r *Redis) Close() error                   { return r.client.Close() }
