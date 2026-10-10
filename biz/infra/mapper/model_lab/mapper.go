package model_lab

import (
	"context"
	"time"

	"github.com/google/wire"
	"github.com/xh-polaris/psych-core-api/biz/infra/cache"
)

type IMapper interface {
	Set(ctx context.Context, key string, value []byte, ttl time.Duration) error
	Get(ctx context.Context, key string) ([]byte, error)
	TryLock(ctx context.Context, key, value string, ttl time.Duration) (bool, error)
	Unlock(ctx context.Context, key, value string) error
}

type RedisMapper struct {
	Cache cache.Cmdable
}

var Set = wire.NewSet(
	wire.Struct(new(RedisMapper), "*"),
	wire.Bind(new(IMapper), new(*RedisMapper)),
)

func (m *RedisMapper) Set(ctx context.Context, key string, value []byte, ttl time.Duration) error {
	return m.Cache.Set(ctx, key, value, ttl).Err()
}

func (m *RedisMapper) Get(ctx context.Context, key string) ([]byte, error) {
	return m.Cache.Get(ctx, key).Bytes()
}

func (m *RedisMapper) TryLock(ctx context.Context, key, value string, ttl time.Duration) (bool, error) {
	return m.Cache.SetNX(ctx, key, value, ttl).Result()
}

// Unlock 仅删除当前调用持有的锁，避免过期后误删其他请求新取得的锁。
func (m *RedisMapper) Unlock(ctx context.Context, key, value string) error {
	const script = `
if redis.call("get", KEYS[1]) == ARGV[1] then
    return redis.call("del", KEYS[1])
end
return 0`
	_, err := m.Cache.Eval(ctx, script, []string{key}, value).Result()
	return err
}
