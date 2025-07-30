package workflow

import (
	"encoding/json"
	retry "github.com/avast/retry-go"
	"github.com/xh-polaris/psych-core-api/biz/infra/config"
	rs "github.com/xh-polaris/psych-core-api/biz/infra/redis"
	"github.com/xh-polaris/psych-pkg/core"
	"github.com/xh-polaris/psych-pkg/util/logx"
	"github.com/zeromicro/go-zero/core/stores/redis"
	"sync"
	"time"
)

// HisEntry 历史记录条目
type HisEntry struct {
	Role      string
	Content   string
	Timestamp time.Time
}

type HistoryPipe struct {
	session string
	rs      *HisRedis
	in      *core.Channel[*HisEntry]
}

func NewHistoryPipe(close chan struct{}, session string) *HistoryPipe {
	return &HistoryPipe{
		session: session,
		rs:      GetHisRedis(),
		in:      core.NewChannel[*HisEntry](3, close),
	}
}

func (p *HistoryPipe) In() {
	var err error
	for entry := range p.in.C {
		if err = p.rs.add(p.session, entry); err != nil {
			logx.Error("[history pipe] add err:%v", err)
		}
	}
}

func (p *HistoryPipe) Run() {
	go p.In()
}

var (
	rsInstance *HisRedis
	rsOnce     sync.Once
	retries    = 3
)

type HisRedis struct {
	rs *redis.Redis
}

func GetHisRedis() *HisRedis {
	rsOnce.Do(func() {
		c := config.GetConfig()
		rsInstance = &HisRedis{
			rs: rs.NewRedis(c),
		}
	})
	return rsInstance
}

// add 将对话记录添加到队列尾部
func (r *HisRedis) add(session string, entry *HisEntry) (err error) {
	// 序列化
	var data []byte
	if data, err = json.Marshal(entry); err != nil {
		return err
	}

	return r.retry(session, "add", func() error {
		_, err = r.rs.Rpush(session, string(data))
		return err
	})
}

// Load 获取session对应的所有对话记录
func (r *HisRedis) Load(session string) (history []*HisEntry, err error) {
	var data []string
	if err = r.retry(session, "load", func() error {
		data, err = r.rs.Lrange(session, 0, -1)
		return err
	}); err != nil {
		return nil, err
	}

	for _, v := range data {
		var his HisEntry
		if err = json.Unmarshal([]byte(v), &his); err != nil {
			return nil, err
		}
		history = append(history, &his)
	}
	return history, nil
}

// Remove 删除Session对应的记录
func (r *HisRedis) Remove(session string) error {
	return r.retry(session, "del", func() error {
		_, err := r.rs.Del(session)
		return err
	})
}

func (r *HisRedis) retry(session, action string, operation func() error) error {
	// 配置重试策略
	opts := []retry.Option{
		retry.Attempts(uint(retries)),       // 最大重试次数
		retry.DelayType(retry.BackOffDelay), // 指数退避策略
		retry.MaxDelay(3 * time.Second),     // 最大退避间隔
		retry.OnRetry(func(n uint, err error) { // 重试日志
			logx.Info("[his redis] [%s] retry #%d times for session %s with err:%v", action, n+1, session, err)
		}),
	}
	return retry.Do(operation, opts...)
}
