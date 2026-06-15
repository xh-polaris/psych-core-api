package sms

import (
	"context"
	"fmt"

	"github.com/xh-polaris/psych-core-api/biz/conf"
	"github.com/xh-polaris/psych-core-api/biz/infra/sms/tencent"
)

var Mgr Service

// Param 短信模板参数
type Param struct {
	TemplateId string   // 模板ID
	Params     []string // 按顺序对应短信模板中的变量
}

// Service 短信发送接口
type Service interface {
	Send(ctx context.Context, phone string, param *Param) error
}

func New(cfg *conf.Config) {
	raw, err := tencent.New(context.Background(), cfg.SMS.Account, cfg.SMS.Token, cfg.SMS.Extra)
	if err != nil {
		panic(fmt.Sprintf("init SMS tencent client: %v", err))
	}
	Mgr = &tencentAdapter{client: raw}
}

// tencentAdapter 适配 tencent.Client 到 Service 接口
type tencentAdapter struct {
	client *tencent.Client
}

func (a *tencentAdapter) Send(ctx context.Context, phone string, param *Param) error {
	return a.client.Send(ctx, phone, param.TemplateId, param.Params)
}
