package sms

import (
	"context"
	"fmt"

	"github.com/xh-polaris/psych-core-api/biz/conf"
	"github.com/xh-polaris/psych-core-api/biz/infra/sms/tencent"
)

var Mgr Provider

type Provider interface {
	Send(ctx context.Context, cause, phone string, params []string) error
}

func New(cfg *conf.Config) {
	switch cfg.SMS.Provider {
	case "tencent":
		p, err := tencent.New(cfg.SMS.Account, cfg.SMS.Token)
		if err != nil {
			panic(fmt.Sprintf("init SMS tencent provider: %v", err))
		}
		Mgr = p
	default:
		panic("no such SMS provider: " + cfg.SMS.Provider)
	}
}
