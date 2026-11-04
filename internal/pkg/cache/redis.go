package cache

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"
)

// Redis 基于 go-redis 实现 Store。
type Redis struct {
	client *redis.Client
	incr   *redis.Script
}

// NewRedis 创建 Redis 缓存。
// Incr 使用 Lua 脚本保证“自增 + 首次设置过期时间”的原子性，
// 避免并发下过期时间被反复刷新。
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

// Set 写入键值并设置过期时间。
func (r *Redis) Set(ctx context.Context, key, value string, ttl time.Duration) error {
	return r.client.Set(ctx, key, value, ttl).Err()
}

// Get 读取键值；Redis 的 nil 响应转换为统一的 ErrNotFound。
func (r *Redis) Get(ctx context.Context, key string) (string, error) {
	v, err := r.client.Get(ctx, key).Result()
	if err == redis.Nil {
		return "", ErrNotFound
	}
	return v, err
}

// Take 使用 GETDEL 原子地读取并删除键。
func (r *Redis) Take(ctx context.Context, key string) (string, error) {
	v, err := r.client.GetDel(ctx, key).Result()
	if err == redis.Nil {
		return "", ErrNotFound
	}
	return v, err
}

// Del 删除一个或多个键。
func (r *Redis) Del(ctx context.Context, keys ...string) error {
	if len(keys) == 0 {
		return nil
	}
	return r.client.Del(ctx, keys...).Err()
}

// Exists 判断键是否存在。
func (r *Redis) Exists(ctx context.Context, key string) (bool, error) {
	n, err := r.client.Exists(ctx, key).Result()
	return n > 0, err
}

// SetNX 仅在键不存在时写入，用于分布式锁。
func (r *Redis) SetNX(ctx context.Context, key, value string, ttl time.Duration) (bool, error) {
	return r.client.SetNX(ctx, key, value, ttl).Result()
}

// Keys 使用 SCAN 按前缀查找键，避免 KEYS 阻塞 Redis。
func (r *Redis) Keys(ctx context.Context, prefix string) ([]string, error) {
	var cursor uint64
	keys := make([]string, 0)
	for {
		batch, next, err := r.client.Scan(ctx, cursor, prefix+"*", 100).Result()
		if err != nil {
			return nil, err
		}
		keys = append(keys, batch...)
		cursor = next
		if cursor == 0 {
			break
		}
	}
	return keys, nil
}

// Incr 原子自增，并在键首次创建时设置过期时间（毫秒）。
func (r *Redis) Incr(ctx context.Context, key string, ttl time.Duration) (int64, error) {
	return r.incr.Run(ctx, r.client, []string{key}, ttl.Milliseconds()).Int64()
}

// Ping 探测 Redis 连通性。
func (r *Redis) Ping(ctx context.Context) error { return r.client.Ping(ctx).Err() }

// Close 关闭 Redis 客户端。
func (r *Redis) Close() error { return r.client.Close() }
