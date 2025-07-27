package rpc

import (
	"github.com/google/wire"
	"github.com/xh-polaris/psych-core-api/biz/infra/config"
	user "github.com/xh-polaris/psych-idl/kitex_gen/user/psychuserservice"
	"sync"
)

var once sync.Once
var client user.Client

type IPsychUser interface {
	user.Client
}

type PsychUser struct {
	user.Client
}

var PsychUserSet = wire.NewSet(
	NewPsychUser,
	wire.Struct(new(PsychUser), "*"),
	wire.Bind(new(IPsychUser), new(*PsychUser)),
)

func NewPsychUser(config *config.Config) user.Client {
	once.Do(func() {
		client = client.NewClient(config.Name, "psych.user", user.NewClient)
	})
	return client
}

func GetPsychUser() user.Client {
	return client
}
