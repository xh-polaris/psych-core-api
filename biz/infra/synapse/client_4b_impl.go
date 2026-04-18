package synapse

import (
	"context"
	"fmt"

	"github.com/xh-polaris/psych-core-api/biz/conf"
	"github.com/xh-polaris/psych-core-api/biz/infra/util"
	"github.com/xh-polaris/psych-core-api/pkg/errorx"
	"github.com/xh-polaris/psych-core-api/pkg/httpcli"
	"github.com/xh-polaris/psych-core-api/types/errno"
)

// synapse4bClient 是面向 ToB 场景的 synapse4b 客户端。
// 通过嵌入 synapseClient 复用所有公共方法，仅覆盖行为不同的接口：
//   - Login：要求所有 authType 必须携带 unitId（extraAuthId）
//   - Register：ToB 模式不支持自助注册，直接返回 ErrMethodNotSupported
type synapse4bClient struct {
	synapseClient
}

// New4b 创建面向 ToB 的 synapse4b 客户端
func New4b(config *conf.Config) Client {
	c := config.Synapse
	return &synapse4bClient{
		synapseClient: synapseClient{
			baseURL: c.BaseURL,
			state:   "test",
			client:  httpcli.New(httpcli.WithBaseTransport(util.NewDebugTransport())),
		},
	}
}

// Login 在 ToB 模式下要求所有认证类型（手机、邮箱、code）均提供 unitId
func (c *synapse4bClient) Login(ctx context.Context, authType, authId, extraAuthId, verify string) (*LoginResult, error) {
	if extraAuthId == "" {
		return nil, errorx.New(errno.ErrMissingParams, errorx.KV("msg", "ToB 模式登录必须提供 unitId"))
	}
	return c.synapseClient.Login(ctx, authType, authId, extraAuthId, verify)
}

// Register ToB 模式不支持自助注册，账号需由管理员在后台创建
func (c *synapse4bClient) Register(_ context.Context, _, _, _, _, _ string) (*RegisterResult, error) {
	return nil, errorx.New(errno.ErrUnsupported, errorx.KV("msg", "ToB 模式不支持自助注册，请联系管理员创建账号"))
}

// CreateBasicUser 管理员创建账号
func (c *synapse4bClient) CreateBasicUser(ctx context.Context, unitID, code, phone, email, password string, encryptType int64) (*synapseBasicUser, error) {
	body := map[string]any{
		"unitID":      unitID,
		"code":        code,
		"phone":       phone,
		"email":       email,
		"password":    password,
		"encryptType": encryptType,
		"createKey":   conf.GetConfig().Synapse.CreateKey,
		"app":         map[string]any{"name": appName},
	}
	resp, err := httpcli.PostJSON[synapseResp](ctx, c.client, c.baseURL+"/basic_user/create", c.baseHeader(), body)
	if err != nil {
		return nil, err
	}
	if resp.Code != 0 {
		return nil, fmt.Errorf("synapse basic_user_create error: code=%.0f msg=%s", resp.Code, resp.Msg)
	}
	if resp.BasicUser == nil || resp.BasicUser.BasicUserID == "" {
		return nil, fmt.Errorf("synapse basic_user_create: missing basicUserId in response")
	}
	return resp.BasicUser, nil
}

func (c *synapse4bClient) CreateUnit(ctx context.Context, name string) (*UnitResult, error) {
	body := map[string]any{
		"name":      name,
		"createKey": conf.GetConfig().Synapse.CreateKey,
		"app":       map[string]any{"name": appName},
	}
	resp, err := httpcli.PostJSON[synapseResp](ctx, c.client, c.baseURL+"/unit/create", c.baseHeader(), body)
	if err != nil {
		return nil, err
	}
	if resp.Code != 0 {
		return nil, fmt.Errorf("synapse unit_create error: code=%.0f msg=%s", resp.Code, resp.Msg)
	}
	if resp.Unit == nil || resp.Unit.UnitID == "" {
		return nil, fmt.Errorf("synapse unit_create: missing unitId in response")
	}
	return resp.Unit, nil
}

func (c *synapse4bClient) GetUnit(ctx context.Context, unitID string) (*UnitResult, error) {
	body := map[string]any{
		"unitID": unitID,
		"app":    map[string]any{"name": appName},
	}
	resp, err := httpcli.PostJSON[synapseResp](ctx, c.client, c.baseURL+"/unit/get", c.baseHeader(), body)
	if err != nil {
		return nil, err
	}
	if resp.Code != 0 {
		return nil, fmt.Errorf("synapse unit_get error: code=%.0f msg=%s", resp.Code, resp.Msg)
	}
	if resp.Unit == nil || resp.Unit.UnitID == "" {
		return nil, fmt.Errorf("synapse unit_get: missing unitId in response")
	}
	return resp.Unit, nil
}
