package cache

import (
	"context"
	"errors"
	"time"

	"github.com/redis/go-redis/v9"
)

type RedisKV struct {
	client *redis.Client
}

func NewSentinel(masterName string, sentinelAddrs []string) *RedisKV {
	// Чтение и запись идут в мастер, который выбирает Sentinel.
	return &RedisKV{client: redis.NewFailoverClient(&redis.FailoverOptions{
		MasterName:    masterName,
		SentinelAddrs: sentinelAddrs,
	})}
}

func (r *RedisKV) Ping(ctx context.Context) error {
	return r.client.Ping(ctx).Err()
}

func (r *RedisKV) Close() error {
	return r.client.Close()
}

func (r *RedisKV) Get(ctx context.Context, key string) ([]byte, error) {
	value, err := r.client.Get(ctx, key).Bytes()
	if errors.Is(err, redis.Nil) {
		return nil, ErrMiss
	}
	return value, err
}

func (r *RedisKV) Set(ctx context.Context, key string, value []byte, ttl time.Duration) error {
	return r.client.Set(ctx, key, value, ttl).Err()
}

func (r *RedisKV) Del(ctx context.Context, keys ...string) error {
	return r.client.Del(ctx, keys...).Err()
}
