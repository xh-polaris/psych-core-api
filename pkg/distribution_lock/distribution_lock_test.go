package distribution_lock

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/xh-polaris/psych-core-api/biz/infra/cache"
	"github.com/xh-polaris/psych-core-api/biz/infra/cache/redis"
)

// Helper: 生成 Redis Cmdable
func newRedisCmdable() cache.Cmdable {
	return redis.NewWithAddrAndPassword("localhost:6379", "123456")
}

func TestDistributionLock(t *testing.T) {
	ctx := context.Background()
	redisClient := newRedisCmdable()

	// 清理测试 key
	redisClient.Del(ctx, "test_lock")

	t.Run("Basic Lock and Unlock", func(t *testing.T) {
		lock := NewDistributionLock("test_lock", redisClient)
		ok, err := lock.TryLock(ctx, 3*time.Second, 1*time.Second, 2*time.Second)
		t.Log("TryLock result:", ok, "err:", err)
		if err != nil || !ok {
			t.Fatalf("Expected lock acquired, got ok=%v err=%v", ok, err)
		}

		val, _ := redisClient.Get(ctx, "test_lock").Result()
		t.Log("lock key value =", val)

		err = lock.TryUnlock(ctx)
		t.Log("Unlock err:", err)
		if err != nil {
			t.Fatalf("Unlock failed: %v", err)
		}
	})

	t.Run("Lock Failure When Already Locked", func(t *testing.T) {
		redisClient.Del(ctx, "test_lock")
		lock := NewDistributionLock("test_lock", redisClient)
		lock.TryLock(ctx, 5*time.Second, 1*time.Second, 2*time.Second)

		lock2 := NewDistributionLock("test_lock", redisClient)
		ok, _ := lock2.TryLock(ctx, 5*time.Second, 1*time.Second, 2*time.Second)
		t.Log("Second lock attempt ok:", ok)
		if ok {
			t.Fatalf("Expected second lock attempt to fail")
		}

		lock.TryUnlock(ctx)
	})

	t.Run("WatchDog Keeps Lock Alive", func(t *testing.T) {
		redisClient.Del(ctx, "test_lock")

		lock := NewDistributionLock("test_lock", redisClient)
		lock.TryLock(ctx, 3*time.Second, 1*time.Second, 3*time.Second)

		t.Log("Sleeping 4s to allow watchdog to extend lock")
		time.Sleep(4 * time.Second)

		val, _ := redisClient.Get(ctx, "test_lock").Result()
		t.Log("lock key=", val, "expected=", lock.ID())

		if val != lock.ID() {
			t.Fatalf("Watchdog failed to renew lock")
		}

		lock.TryUnlock(ctx)
	})

	t.Run("WatchDog Stops After Unlock", func(t *testing.T) {
		redisClient.Del(ctx, "test_lock")

		lock := NewDistributionLock("test_lock", redisClient)
		lock.TryLock(ctx, 3*time.Second, 1*time.Second, 3*time.Second)

		t.Log("Unlock now")
		lock.TryUnlock(ctx)

		t.Log("Sleeping 4s to make sure watchdog won't renew expired lock")
		time.Sleep(4 * time.Second)

		val, _ := redisClient.Get(ctx, "test_lock").Result()
		t.Log("Final redis key:", val)

		if val != "" {
			t.Fatalf("Watchdog should NOT renew lock after unlock")
		}
	})

	t.Run("WatchDog Stops When Lock Stolen or Modified", func(t *testing.T) {
		redisClient.Del(ctx, "test_lock")

		lock := NewDistributionLock("test_lock", redisClient)
		lock.TryLock(ctx, 3*time.Second, 1*time.Second, 3*time.Second)

		t.Log("Manually overriding key to simulate stolen lock")
		redisClient.Set(ctx, "test_lock", "another-owner", 10*time.Second)

		time.Sleep(2 * time.Second)

		val, _ := redisClient.Get(ctx, "test_lock").Result()
		t.Log("Redis key after override:", val)
		if val != "another-owner" {
			t.Fatalf("Override failed")
		}

		// watchdog should stop automatically
		t.Log("Sleep and check watchdog didn't restore original owner")
		time.Sleep(3 * time.Second)

		val2, _ := redisClient.Get(ctx, "test_lock").Result()
		t.Log("Redis key final:", val2)

		if val2 == lock.ID() {
			t.Fatalf("Watchdog should STOP after lock is stolen")
		}
	})

	t.Run("Concurrent Lock Attempts", func(t *testing.T) {
		redisClient.Del(ctx, "test_lock")

		var wg sync.WaitGroup
		successCount := 0
		mu := sync.Mutex{}

		for i := 0; i < 10; i++ {
			wg.Add(1)
			go func(idx int) {
				defer wg.Done()
				lock := NewDistributionLock("test_lock", redisClient)
				ok, _ := lock.TryLock(ctx, 3*time.Second, 1*time.Second, 3*time.Second)
				if ok {
					mu.Lock()
					successCount++
					mu.Unlock()
					t.Logf("Goroutine %d acquired lock", idx)
					lock.TryUnlock(ctx)
				} else {
					t.Logf("Goroutine %d failed", idx)
				}
			}(i)
		}

		wg.Wait()
		t.Logf("Total successful locks: %d", successCount)

		if successCount == 0 {
			t.Fatalf("No goroutine acquired lock")
		}
		if successCount > 3 { // 因为 lock 会释放，所以可能多次成功，但不会超过 3 次
			t.Fatalf("Unexpected too many lock acquisitions: %d", successCount)
		}
	})
}
