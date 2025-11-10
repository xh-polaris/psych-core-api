package provider

import (
	"github.com/google/wire"
	"github.com/xh-polaris/psych-core-api/biz/application/service"
	"github.com/xh-polaris/psych-core-api/biz/infra/conf"
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

// Provider 提供controller依赖的对象
type Provider struct {
	Config      *conf.Config
	AuthService service.AuthService
}

func Get() *Provider {
	return provider
}

var RpcSet = wire.NewSet(
	rpc.NewPsychUser,
)

var ApplicationSet = wire.NewSet()

var InfrastructureSet = wire.NewSet(
	conf.NewConfig,
	RpcSet,
)

var AllProvider = wire.NewSet(
	ApplicationSet,
	InfrastructureSet,
)
