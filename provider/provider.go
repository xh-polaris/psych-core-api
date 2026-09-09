package provider

import (
	"github.com/google/wire"
	"github.com/xh-polaris/psych-core-api/biz/application/service"
	"github.com/xh-polaris/psych-core-api/biz/conf"
	"github.com/xh-polaris/psych-core-api/biz/domain/auth"
	"github.com/xh-polaris/psych-core-api/biz/domain/dashboard"
	"github.com/xh-polaris/psych-core-api/biz/domain/usr"
	"github.com/xh-polaris/psych-core-api/biz/infra/cache/redis"
	"github.com/xh-polaris/psych-core-api/biz/infra/mapper/alarm"
	"github.com/xh-polaris/psych-core-api/biz/infra/mapper/config"
	"github.com/xh-polaris/psych-core-api/biz/infra/mapper/conversation"
	"github.com/xh-polaris/psych-core-api/biz/infra/mapper/message"
	"github.com/xh-polaris/psych-core-api/biz/infra/mapper/prompt"
	"github.com/xh-polaris/psych-core-api/biz/infra/mapper/report"
	"github.com/xh-polaris/psych-core-api/biz/infra/mapper/sms_alert"
	"github.com/xh-polaris/psych-core-api/biz/infra/mapper/unit"
	"github.com/xh-polaris/psych-core-api/biz/infra/mapper/user"
	"github.com/xh-polaris/psych-core-api/biz/infra/mapper/voice"
	"github.com/xh-polaris/psych-core-api/biz/infra/storage"
	"github.com/xh-polaris/psych-core-api/biz/infra/synapse"
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
	DashboardService    service.DashboardService
	ConfigService       service.ConfigService
	UserService         service.UserService
	UnitService         service.UnitService
	ConversationService service.ConversationService
	FileService         service.FileService
	ChatReportService   service.ChatReportService
	MessageMapper       message.IMongoMapper
	ConversationMapper  conversation.IMongoMapper
	ReportMapper        report.IMongoMapper
	UserMapper          user.IMongoMapper
	ConfigMapper        config.IMongoMapper
	UnitMapper          unit.IMongoMapper
	SmsAlertMapper      sms_alert.IMongoMapper
	PromptMapper        prompt.IMongoMapper
	VoiceMapper         voice.IMongoMapper
}

func Get() *Provider {
	return provider
}

var RpcSet = wire.NewSet()

var ApplicationSet = wire.NewSet(
	service.DashboardServiceSet,
	service.ConfigServiceSet,
	service.UserServiceSet,
	service.UnitServiceSet,
	service.ConversationServiceSet,
	service.FileServiceSet,
	service.ChatReportServiceSet,
)

var DomainSet = wire.NewSet(
	usr.UserDomainSet,
	auth.AuthDomainSet,
	dashboard.DashboardDomainSet,
)

var InfrastructureSet = wire.NewSet(
	conf.NewConfig,
	redis.New,
	message.NewMessageMongoMapper,
	user.NewUserMongoMapper,
	unit.NewUnitMongoMapper,
	config.NewConfigMongoMapper,
	conversation.NewConversationMongoMapper,
	alarm.NewAlarmMongoMapper,
	report.NewReportMongoMapper,
	sms_alert.NewSmsAlertMongoMapper,
	prompt.NewPromptMongoMapper,
	voice.NewVoiceMongoMapper,
	RpcSet,
	synapse.New4b,
	storage.NewCOS,
)

var AllProvider = wire.NewSet(
	ApplicationSet,
	DomainSet,
	InfrastructureSet,
)
