package provider

import (
	"github.com/google/wire"
	"github.com/xh-polaris/psych-core-api/biz/application/service"
	"github.com/xh-polaris/psych-core-api/biz/conf"
	"github.com/xh-polaris/psych-core-api/biz/infra/mapper/alarm"
	"github.com/xh-polaris/psych-core-api/biz/infra/mapper/config"
	"github.com/xh-polaris/psych-core-api/biz/infra/mapper/conversation"
	"github.com/xh-polaris/psych-core-api/biz/infra/mapper/message"
	"github.com/xh-polaris/psych-core-api/biz/infra/mapper/report"
	"github.com/xh-polaris/psych-core-api/biz/infra/mapper/unit"
	"github.com/xh-polaris/psych-core-api/biz/infra/mapper/user"
)

var provider *Provider

func Init() {
	var err error
	provider, err = NewProvider()
	if err != nil {
		panic(err)
	}
}

// Provider 依赖的对象
type Provider struct {
	Config              *conf.Config
	AlarmService        service.AlarmService
	DashboardService    service.DashboardService
	ConfigService       service.ConfigService
	UserService         service.UserService
	UnitService         service.UnitService
	ConversationService service.ConversationService
	MessageMapper       message.IMongoMapper
	ConversationMapper  conversation.IMongoMapper
	ReportMapper        report.IMongoMapper
}

func Get() *Provider {
	return provider
}

var RpcSet = wire.NewSet()

var ApplicationSet = wire.NewSet(
	service.AlarmServiceSet,
	service.DashboardServiceSet,
	service.ConfigServiceSet,
	service.UserServiceSet,
	service.UnitServiceSet,
	service.ConversationServiceSet,
)

var InfrastructureSet = wire.NewSet(
	conf.NewConfig,
	message.NewMessageMongoMapper,
	user.NewUserMongoMapper,
	unit.NewUnitMongoMapper,
	config.NewConfigMongoMapper,
	conversation.NewConversationMongoMapper,
	alarm.NewAlarmMongoMapper,
	report.NewReportMongoMapper,
	RpcSet,
)

var AllProvider = wire.NewSet(
	ApplicationSet,
	InfrastructureSet,
)
