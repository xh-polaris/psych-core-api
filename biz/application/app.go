package application

import (
	"github.com/xh-polaris/psych-core-api/biz/domain/alert"
	"github.com/xh-polaris/psych-core-api/biz/domain/his"
	"github.com/xh-polaris/psych-core-api/biz/domain/prompt"
	"github.com/xh-polaris/psych-core-api/biz/infra/cache"
	"github.com/xh-polaris/psych-core-api/biz/infra/cache/redis"
	"github.com/xh-polaris/psych-core-api/biz/infra/lock"
	"github.com/xh-polaris/psych-core-api/biz/infra/mapper/config"
	"github.com/xh-polaris/psych-core-api/biz/infra/mapper/conversation"
	"github.com/xh-polaris/psych-core-api/biz/infra/mapper/message"
	pmapper "github.com/xh-polaris/psych-core-api/biz/infra/mapper/prompt"
	"github.com/xh-polaris/psych-core-api/biz/infra/mapper/report"
	"github.com/xh-polaris/psych-core-api/biz/infra/mapper/sms_alert"
	"github.com/xh-polaris/psych-core-api/biz/infra/mapper/unit"
	"github.com/xh-polaris/psych-core-api/biz/infra/mapper/user"
	"github.com/xh-polaris/psych-core-api/biz/infra/sms"
	"github.com/xh-polaris/psych-core-api/pkg/httpx"
	"github.com/xh-polaris/psych-core-api/provider"
)

type AppDependency struct {
	Cache              cache.Cmdable
	MessageMapper      message.IMongoMapper
	ConversationMapper conversation.IMongoMapper
	ReportMapper       report.IMongoMapper
	UserMapper         user.IMongoMapper
	UnitMapper         unit.IMongoMapper
	ConfigMapper       config.IMongoMapper
	SmsAlertMapper     sms_alert.IMongoMapper
	PromptMapper       pmapper.IMongoMapper
}

func InitApplication() {
	// 初始化带追踪的 Mongo Client 注入到 mon 管理中
	config := provider.Get().Config
	if _, err := httpx.NewTracedClient(config.Mongo.URL); err != nil {
		panic(err)
	}

	app := &AppDependency{}
	InitInfra(app)
	InitDomain(app)
}

func InitInfra(app *AppDependency) {
	app.Cache = redis.New()
	app.MessageMapper = provider.Get().MessageMapper
	app.ConversationMapper = provider.Get().ConversationMapper
	app.ReportMapper = provider.Get().ReportMapper
	app.UserMapper = provider.Get().UserMapper
	app.UnitMapper = provider.Get().UnitMapper
	app.ConfigMapper = provider.Get().ConfigMapper
	app.SmsAlertMapper = provider.Get().SmsAlertMapper
	app.PromptMapper = provider.Get().PromptMapper
	lock.New(app.Cache)
	sms.New(provider.Get().Config)
}

func InitDomain(app *AppDependency) {
	his.New(app.Cache, app.MessageMapper, app.ConversationMapper)
	alert.New(app.Cache, app.UserMapper, app.ConfigMapper, app.UnitMapper, app.SmsAlertMapper)
	prompt.New(app.Cache, app.PromptMapper)
}
