package provider

import (
	"github.com/google/wire"
	"github.com/xh-polaris/psych-core-api/biz/application/service"
	"github.com/xh-polaris/psych-core-api/biz/conf"
	"github.com/xh-polaris/psych-core-api/biz/infra/mapper/message"
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
	Config        *conf.Config
	AuthService   service.AuthService
	MessageMapper message.MongoMapper
}

func Get() *Provider {
	return provider
}

var RpcSet = wire.NewSet(
	rpc.NewPsychUser,
)

var ApplicationSet = wire.NewSet(
	service.AuthServiceSet,
)

var InfrastructureSet = wire.NewSet(
	conf.NewConfig,
	message.NewMessageMongoMapper,
	RpcSet,
)

var AllProvider = wire.NewSet(
	ApplicationSet,
	InfrastructureSet,
)
