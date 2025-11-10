package redis

import (
	"github.com/xh-polaris/psych-core-api/biz/infra/conf"
	"github.com/zeromicro/go-zero/core/stores/redis"
)

func NewRedis(config *conf.Config) *redis.Redis {
	return redis.MustNewRedis(*config.Redis)
}
