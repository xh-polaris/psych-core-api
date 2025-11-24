package distribution_lock

import (
	"context"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/xh-polaris/psych-core-api/biz/infra/cache"
)

const prefix = "dis_lock:"

// DistributionLock 不可重入的分布式锁
type DistributionLock struct {
	cache cache.Cmdable
	id    string
	name  string

	cancelWatchDog context.CancelFunc // 用于停止续期 goroutine
	wg             sync.WaitGroup     // 等待 watchDog 退出
}

// NewDistributionLock 创建分布式锁
func NewDistributionLock(name string, cache cache.Cmdable) *DistributionLock {
	return &DistributionLock{
		cache: cache,
		name:  prefix + name,
		id:    uuid.New().String(),
	}
}

// TryLock 尝试获取锁, 无等待, 如果 leaseTime>0 会启动续期 watchdog
func (l *DistributionLock) TryLock(ctx context.Context, leaseTime, interval, extendTime time.Duration) (bool, error) {
	if leaseTime <= 0 {
		leaseTime = 0
	}

	ok, err := l.cache.SetNX(ctx, l.name, l.id, leaseTime).Result()
	if err != nil || !ok {
		return ok, err
	}

	// 如果有 leaseTime 且不是永久锁，启动 watchDog
	if leaseTime > 0 {
		watchCtx, cancel := context.WithCancel(context.Background())
		l.cancelWatchDog = cancel
		l.wg.Add(1)
		go l.watchDog(watchCtx, interval, extendTime)
	}

	return ok, err
}

// TryUnlock 释放锁，并停止 watchDog
func (l *DistributionLock) TryUnlock(ctx context.Context) error {
	// 停止 watchdog
	if l.cancelWatchDog != nil {
		l.cancelWatchDog()
		l.wg.Wait()
		l.cancelWatchDog = nil
	}

	script := `
    if redis.call("get", KEYS[1]) == ARGV[1] then
        return redis.call("del", KEYS[1])
    else
        return 0
    end`
	_, err := l.cache.Eval(ctx, script, []string{l.name}, l.id).Result()
	return err
}

func (l *DistributionLock) watchDog(ctx context.Context, interval, extendTime time.Duration) {
	defer l.wg.Done()
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			script := `
            if redis.call("get", KEYS[1]) == ARGV[1] then
                return redis.call("expire", KEYS[1], ARGV[2])
            else
                return 0
            end`
			res, err := l.cache.Eval(ctx, script, []string{l.name}, l.id, extendTime.Seconds()).Result()
			if err != nil { // 出错就暂时跳过一次续期，等待下一个 tick
				continue
			}
			if val, ok := res.(int64); ok && val == 0 { // 锁不是自己持有或已被释放，退出 watchdog
				return
			}
		}
	}
}

func (l *DistributionLock) ID() string {
	return l.id
}
