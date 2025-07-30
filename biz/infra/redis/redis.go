package redis

import (
	"github.com/xh-polaris/psych-core-api/biz/infra/config"
	"github.com/zeromicro/go-zero/core/stores/redis"
)

func NewRedis(config *config.Config) *redis.Redis {
	return redis.MustNewRedis(*config.Redis)
}
