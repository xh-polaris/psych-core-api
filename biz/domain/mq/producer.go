package mq

import (
	"encoding/json"
	"github.com/avast/retry-go"
	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/xh-polaris/psych-core-api/biz/infra/config"
	"github.com/xh-polaris/psych-pkg/core"
	"github.com/xh-polaris/psych-pkg/util/logx"
	"golang.org/x/net/context"
	"sync"
	"time"
)

// optimize 目前mq断连直接panic, 应该优化成降级本地缓存, 直至重连成功

// conn 采用单例模式, 复用连接
var (
	conn     *amqp.Connection
	once     sync.Once
	url      string
	maxRetry = 5
)

// getConn 获取连接单例
func getConn() *amqp.Connection {
	once.Do(func() {
		var err error
		url = config.GetConfig().RabbitMQ.Url
		if conn, err = amqp.Dial(url); err != nil {
			panic("[mq] connect failed:" + err.Error())
		}
		go monitor() // 自动重连监听
	})
	return conn
}

// monitor 监听健康状态并重连
func monitor() {
	opts := []retry.Option{
		retry.Attempts(uint(maxRetry)),      // 最大重试次数
		retry.DelayType(retry.BackOffDelay), // 指数退避策略
		retry.MaxDelay(64 * time.Second),    // 最大退避间隔
		retry.OnRetry(func(n uint, err error) { // 重试日志
			logx.Info("[mq produce] retry #%d times with err:%v", n+1, err)
		}),
	}

	operation := func() (err error) {
		if conn, err = amqp.Dial(url); err == nil {
			logx.Info("[mq producer] reconnect")
		}
		return err
	}

	for {
		reason := <-conn.NotifyClose(make(chan *amqp.Error))
		logx.Info("[mq producer] connection closed , reason: ", reason)
		if err := retry.Do(operation, opts...); err != nil {
			panic("[mq produce] retry too many times:" + err.Error())
		}
		conn.NotifyClose(make(chan *amqp.Error)) // 重新监听
	}
}

var (
	producer     *PostProducer
	producerOnce sync.Once
)

// PostProducer 对话后处理
type PostProducer struct {
	mu      sync.Mutex
	conn    *amqp.Connection
	channel *amqp.Channel
}

// GetHistoryProducer 获取历史记录生产者
func GetHistoryProducer() *PostProducer {
	producerOnce.Do(func() {
		var err error
		producer = new(PostProducer)
		producer.conn = getConn()
		if producer.channel, err = producer.conn.Channel(); err != nil {
			panic("[mq producer] create channel failed" + err.Error())
		}
	})
	return producer
}

// Produce 创建历史记录消息
func (p *PostProducer) Produce(ctx context.Context, session string, info map[string]any, start, end time.Time) (err error) {
	var payload []byte
	// 构造消息体
	msg := &core.PostNotify{Session: session, Info: info, Start: start.Unix(), End: end.Unix()}
	if payload, err = json.Marshal(msg); err != nil {
		logx.Error("[mq producer] marshal post notify failed, err:%v", err.Error())
		return err
	}

	p.mu.Lock()
	defer p.mu.Unlock()
	// 发布持久化消息
	err = p.channel.PublishWithContext(ctx, "psych_his", "psych_his.end", false, false,
		amqp.Publishing{
			DeliveryMode: amqp.Persistent,
			ContentType:  "application/json",
			Body:         payload,
		})
	return err
}
