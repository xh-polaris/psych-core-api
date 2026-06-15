package tencent

import (
	"context"
	"errors"

	"github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/common"
	tencenterrors "github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/common/errors"
	"github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/common/profile"
	tencentsms "github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/sms/v20210111"
	"github.com/xh-polaris/psych-core-api/biz/conf"
	"github.com/xh-polaris/psych-core-api/pkg/logs"
)

type TencentSMS struct {
	client *tencentsms.Client
}

func New(account, token string) (*TencentSMS, error) {
	credential := common.NewCredential(account, token)
	client, err := tencentsms.NewClient(credential, "ap-guangzhou", profile.NewClientProfile())
	if err != nil {
		return nil, err
	}
	return &TencentSMS{client: client}, nil
}

func (t *TencentSMS) Send(_ context.Context, cause, phone string, params []string) error {
	cfg := conf.GetConfig().SMS
	templateId := cfg.CauseTemplate[cause]
	if templateId == "" {
		logs.Errorf("[sms] no template configured for cause %s", cause)
		return errors.New("sms: no template for cause: " + cause)
	}

	req := tencentsms.NewSendSmsRequest()
	req.SmsSdkAppId = common.StringPtr(cfg.Extra["AppId"])
	req.SignName = common.StringPtr(cfg.Extra["Sign"])
	req.TemplateId = common.StringPtr(templateId)
	req.TemplateParamSet = common.StringPtrs(params)
	req.PhoneNumberSet = common.StringPtrs([]string{phone})

	_, err := t.client.SendSms(req)

	var tencentCloudSDKError *tencenterrors.TencentCloudSDKError
	if errors.As(err, &tencentCloudSDKError) {
		logs.Errorf("Tencent SMS API error: %s", err)
	}
	return err
}
