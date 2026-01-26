package provider

import (
	"github.com/google/wire"
	"github.com/xh-polaris/psych-core-api/biz/application/service"
	"github.com/xh-polaris/psych-core-api/biz/conf"
	"github.com/xh-polaris/psych-core-api/biz/infra/mapper/alarm"
	"github.com/xh-polaris/psych-core-api/biz/infra/mapper/config"
	"github.com/xh-polaris/psych-core-api/biz/infra/mapper/message"
	"github.com/xh-polaris/psych-core-api/biz/infra/mapper/unit"
	"github.com/xh-polaris/psych-core-api/biz/infra/mapper/user"
	"github.com/xh-polaris/psych-core-api/biz/infra/rpc"
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
	Config           *conf.Config
	AuthService      service.AuthService
	AlarmService     service.AlarmService
	DashboardService service.DashboardService
	MessageMapper    message.MongoMapper
}

func Get() *Provider {
	return provider
}

var RpcSet = wire.NewSet(
	rpc.NewPsychProfile,
)

var ApplicationSet = wire.NewSet(
	service.AuthServiceSet,
	service.AlarmServiceSet,
	service.DashboardServiceSet,
)

var InfrastructureSet = wire.NewSet(
	conf.NewConfig,
	message.NewMessageMongoMapper,
	user.NewUserMongoMapper,
	unit.NewUnitMongoMapper,
	config.NewConfigMongoMapper,

	alarm.NewAlarmMongoMapper,
	RpcSet,
)

var AllProvider = wire.NewSet(
	ApplicationSet,
	InfrastructureSet,
)
