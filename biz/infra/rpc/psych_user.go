package rpc

import (
	"github.com/google/wire"
	"github.com/xh-polaris/gopkg/kitex/client"
	"github.com/xh-polaris/psych-core-api/biz/infra/config"
	user "github.com/xh-polaris/psych-idl/kitex_gen/user/psychuserservice"
	"sync"
)

var puOnce sync.Once
var puClnt user.Client

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
	puOnce.Do(func() {
		puClnt = client.NewClient(config.Name, "psych.user", user.NewClient)
	})
	return puClnt
}

func GetPsychUser() user.Client {
	if puClnt == nil {
		puOnce.Do(func() { // optimize 如果获取不到, 就会一直是空
			puClnt = client.NewClient(config.GetConfig().Name, "psych.user", user.NewClient)
		})
	}
	return puClnt
}
