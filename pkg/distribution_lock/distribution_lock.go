package distribution_lock

import "github.com/xh-polaris/psych-core-api/biz/infra/cache"

// DistributionLock 不可重入的分布式锁
type DistributionLock struct {
	cache cache.Cmdable // 缓存客户端
	name  string        // 名称
	// watchDog 调度器
}

func NewDistributionLock(name string, cache cache.Cmdable) *DistributionLock {
	return &DistributionLock{
		cache: cache,
		name:  name,
	}
}
