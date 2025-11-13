package rpc

import (
	"github.com/xh-polaris/psych-core-api/biz/infra/conf"

	"github.com/google/wire"
	"github.com/xh-polaris/gopkg/kitex/client"
	profile "github.com/xh-polaris/psych-idl/kitex_gen/profile/psychprofileservice"
	"sync"
)

var ppOnce sync.Once
var ppClnt profile.Client

type IProfileService interface {
	profile.Client
}
type PsychProfile struct {
	profile.Client
}

var PsychProfileSet = wire.NewSet()

func NewPsychProfile(config conf.Config) profile.Client {
	ppOnce.Do(func() {
		ppClnt = client.NewClient(config.Name, "psych.profile", profile.NewClient)
	})
	return ppClnt
}

func GetPsychProfile() profile.Client {
	if ppClnt == nil {
		ppOnce.Do(func() {
			ppClnt = client.NewClient(conf.GetConfig().Name, "psych.profile", profile.NewClient)
		})
	}
	return ppClnt
}
