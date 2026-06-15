package tencent

import (
	"context"
	"errors"

	"github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/common"
	tencenterrors "github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/common/errors"
	"github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/common/profile"
	tencentsms "github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/sms/v20210111"
	"github.com/xh-polaris/psych-core-api/pkg/logs"
)

type Client struct {
	client   *tencentsms.Client
	appId    string
	signName string
}

func New(_ context.Context, secretId, secretKey string, extra map[string]string) (*Client, error) {
	credential := common.NewCredential(secretId, secretKey)
	client, err := tencentsms.NewClient(credential, "ap-guangzhou", profile.NewClientProfile())
	if err != nil {
		return nil, err
	}
	return &Client{
		client:   client,
		appId:    extra["AppId"],
		signName: extra["Sign"],
	}, nil
}

func (c *Client) Send(_ context.Context, phone, templateId string, params []string) error {
	req := tencentsms.NewSendSmsRequest()
	req.SmsSdkAppId = common.StringPtr(c.appId)
	req.SignName = common.StringPtr(c.signName)
	req.TemplateId = common.StringPtr(templateId)
	req.TemplateParamSet = common.StringPtrs(params)
	req.PhoneNumberSet = common.StringPtrs([]string{phone})

	_, err := c.client.SendSms(req)

	var tencentCloudSDKError *tencenterrors.TencentCloudSDKError
	if errors.As(err, &tencentCloudSDKError) {
		logs.Errorf("Tencent SMS API error: %s", err)
	}
	return err
}
