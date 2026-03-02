package application

import (
	"github.com/xh-polaris/psych-core-api/biz/domain/his"
	"github.com/xh-polaris/psych-core-api/biz/infra/cache"
	"github.com/xh-polaris/psych-core-api/biz/infra/cache/redis"
	"github.com/xh-polaris/psych-core-api/biz/infra/lock"
	"github.com/xh-polaris/psych-core-api/biz/infra/mapper/conversation"
	"github.com/xh-polaris/psych-core-api/biz/infra/mapper/message"
	"github.com/xh-polaris/psych-core-api/provider"
)

type AppDependency struct {
	// infra
	Cache              cache.Cmdable             // Cache 缓存
	MessageMapper      message.MongoMapper       // MessageMapper 消息持久层
	ConversationMapper conversation.IMongoMapper // ConversationMapper 对话元信息持久层
}

func InitApplication() {
	app := &AppDependency{}
	InitInfra(app)
	InitDomain(app)
}

func InitInfra(app *AppDependency) {
	app.Cache = redis.New()
	app.MessageMapper = provider.Get().MessageMapper
	app.ConversationMapper = provider.Get().ConversationMapper
	lock.New(app.Cache) // 初始化 DistributionLockManager 分布式锁管理
}

func InitDomain(app *AppDependency) {
	his.New(app.Cache, app.MessageMapper, app.ConversationMapper) // 初始化 HistoryManager 历史记录管理
}
