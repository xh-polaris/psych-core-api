package distribution_lock

import "github.com/xh-polaris/psych-core-api/biz/infra/cache"

var Mgr *DistributionLockManager

func New(cache cache.Cmdable) {
	Mgr = &DistributionLockManager{cache: cache}
	return
}

// DistributionLockManager 分布式锁管理器
type DistributionLockManager struct {
	cache cache.Cmdable
}
