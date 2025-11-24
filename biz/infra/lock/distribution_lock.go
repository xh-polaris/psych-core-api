package lock

import (
	"context"
	"time"

	"github.com/xh-polaris/psych-core-api/biz/infra/cache"
	"github.com/xh-polaris/psych-core-api/pkg/distribution_lock"
)

var Mgr *DistributionLockManager

func New(cache cache.Cmdable) {
	Mgr = &DistributionLockManager{cache: cache}
	return
}

// DistributionLockManager 分布式锁管理器
type DistributionLockManager struct {
	cache cache.Cmdable
}

func (d *DistributionLockManager) NewLock(key string) *distribution_lock.DistributionLock {
	return distribution_lock.NewDistributionLock(key, d.cache)
}

type DistributionLock interface {
	TryLock(ctx context.Context, leaseTime, interval, expend time.Duration) (bool, error)
	TryUnlock(ctx context.Context) error
}
