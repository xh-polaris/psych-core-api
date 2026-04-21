package synapse

import (
	"context"
	"fmt"
	"net/http"

	"github.com/xh-polaris/psych-core-api/biz/conf"
	"github.com/xh-polaris/psych-core-api/biz/infra/util"
	"github.com/xh-polaris/psych-core-api/pkg/errorx"
	"github.com/xh-polaris/psych-core-api/pkg/httpcli"
	"github.com/xh-polaris/psych-core-api/types/errno"
)

const appName = "Psych"

type synapseClient struct {
	baseURL string
	state   string // "prod" | "test"，非 prod 时加 X-Xh-Env: test header
	client  *httpcli.Client
}

func New(config *conf.Config) Client {
	c := config.Synapse
	return &synapseClient{
		baseURL: c.BaseURL,
		state:   c.State,
		client:  httpcli.New(httpcli.WithBaseTransport(util.NewDebugTransport())),
	}
}

func (c *synapseClient) baseHeader() http.Header {
	h := http.Header{}
	h.Set("content-type", "application/json")
	if c.state != "prod" {
		h.Set("X-Xh-Env", "test")
	}
	return h
}

// synapseResp is the common response envelope for all synapse API endpoints.
type synapseResp struct {
	Code      float64           `json:"code"`
	Msg       string            `json:"msg"`
	Token     string            `json:"token"`
	Verify    bool              `json:"verify"`
	New       bool              `json:"new"`
	BasicUser *synapseBasicUser `json:"basicUser"`
	Unit      *UnitResult       `json:"unit"`
}

type synapseBasicUser struct {
	BasicUserID string `json:"basicUserId"`
	UnitID      string `json:"unitId"`
	Phone       string `json:"phone"`
	Email       string `json:"email"`
	Code        string `json:"code"`
	Name        string `json:"name"`
}

func (c *synapseClient) Login(ctx context.Context, authType, authId, extraAuthId, verify string) (*LoginResult, error) {
	body := map[string]any{
		"authType": authType,
		"authId":   authId,
		"verify":   verify,
		"app":      map[string]any{"name": appName},
	}
	if extraAuthId != "" {
		body["extraAuthId"] = extraAuthId
	}
	resp, err := httpcli.PostJSON[synapseResp](ctx, c.client, c.baseURL+"/basic_user/login", c.baseHeader(), body)
	if err != nil {
		return nil, err
	}
	if resp.Code != 0 {
		return nil, fmt.Errorf("synapse login error: code=%.0f msg=%s", resp.Code, resp.Msg)
	}
	if resp.BasicUser == nil || resp.BasicUser.BasicUserID == "" {
		return nil, fmt.Errorf("synapse login: missing basicUserId in response")
	}
	if resp.Token == "" {
		return nil, fmt.Errorf("synapse login: missing token in response")
	}
	return &LoginResult{
		BasicUserID: resp.BasicUser.BasicUserID,
		Token:       resp.Token,
		IsNew:       resp.Verify,
		UnitID:      resp.BasicUser.UnitID,
		Phone:       resp.BasicUser.Phone,
		Email:       resp.BasicUser.Email,
		StudentID:   resp.BasicUser.Code,
		Name:        resp.BasicUser.Name,
	}, nil
}

func (c *synapseClient) Register(ctx context.Context, authType, authId, extraAuthId, verify, password string) (*RegisterResult, error) {
	body := map[string]any{
		"authType": authType,
		"authId":   authId,
		"verify":   verify,
		"password": password,
		"app":      map[string]any{"name": appName},
	}
	if extraAuthId != "" {
		body["extraAuthId"] = extraAuthId
	}
	resp, err := httpcli.PostJSON[synapseResp](ctx, c.client, c.baseURL+"/basic_user/register", c.baseHeader(), body)
	if err != nil {
		return nil, err
	}
	if resp.Code != 0 {
		return nil, fmt.Errorf("synapse register error: code=%.0f msg=%s", resp.Code, resp.Msg)
	}
	if resp.Token == "" {
		return nil, fmt.Errorf("synapse register: missing token in response")
	}
	return &RegisterResult{Token: resp.Token}, nil
}

func (c *synapseClient) ResetPassword(ctx context.Context, authorization, newPassword, resetKey, basicUserId string) error {
	h := c.baseHeader()
	h.Set("Authorization", authorization)
	body := map[string]any{
		"newPassword": newPassword,
		"resetKey":    resetKey,
		"basicUserId": basicUserId,
		"app":         map[string]any{"name": appName},
	}
	resp, err := httpcli.PostJSON[synapseResp](ctx, c.client, c.baseURL+"/basic_user/reset_password", h, body)
	if err != nil {
		return err
	}
	if resp.Code != 0 {
		return fmt.Errorf("synapse reset_password error: code=%.0f msg=%s", resp.Code, resp.Msg)
	}
	return nil
}

func (c *synapseClient) SendVerifyCode(ctx context.Context, authType, authId, cause string) error {
	if cause == "" {
		cause = "passport"
	}
	body := map[string]any{
		"authType": authType,
		"authId":   authId,
		"expire":   300,
		"cause":    cause,
		"app":      map[string]any{"name": appName},
	}
	resp, err := httpcli.PostJSON[synapseResp](ctx, c.client, c.baseURL+"/system/send_verify_code", c.baseHeader(), body)
	if err != nil {
		return err
	}
	if resp.Code != 0 {
		return fmt.Errorf("synapse send_verify_code error: code=%.0f msg=%s", resp.Code, resp.Msg)
	}
	return nil
}

func (c *synapseClient) CheckVerifyCode(ctx context.Context, authType, authId, cause, verify string) error {
	body := map[string]any{
		"authType": authType,
		"authId":   authId,
		"cause":    cause,
		"verify":   verify,
		"app":      map[string]any{"name": appName},
	}
	resp, err := httpcli.PostJSON[synapseResp](ctx, c.client, c.baseURL+"/system/check_verify_code", c.baseHeader(), body)
	if err != nil {
		return err
	}
	if resp.Code != 0 {
		return fmt.Errorf("synapse check_verify_code error: code=%.0f msg=%s", resp.Code, resp.Msg)
	}
	if !resp.Verify {
		return fmt.Errorf("synapse check_verify_code error: verify failed")
	}
	return nil
}

func (c *synapseClient) ThirdPartyLogin(ctx context.Context, thirdparty, ticket string) (*LoginResult, error) {
	body := map[string]any{
		"thirdparty": thirdparty,
		"ticket":     ticket,
	}
	resp, err := httpcli.PostJSON[synapseResp](ctx, c.client, c.baseURL+"/thirdparty/login", c.baseHeader(), body)
	if err != nil {
		return nil, err
	}
	if resp.Code != 0 {
		return nil, fmt.Errorf("synapse thirdparty_login error: code=%.0f msg=%s", resp.Code, resp.Msg)
	}
	if resp.BasicUser == nil || resp.BasicUser.BasicUserID == "" {
		return nil, fmt.Errorf("synapse thirdparty_login: missing basicUserId in response")
	}
	if resp.Token == "" {
		return nil, fmt.Errorf("synapse thirdparty_login: missing token in response")
	}
	return &LoginResult{
		BasicUserID: resp.BasicUser.BasicUserID,
		Token:       resp.Token,
		IsNew:       resp.New,
	}, nil
}

func (c *synapseClient) CreateBasicUser(ctx context.Context, unitID, code, phone, email, password string, encryptType int64) (*synapseBasicUser, error) {
	return nil, errorx.New(errno.ErrUnImplement)
}

func (c *synapseClient) CreateUnit(ctx context.Context, name string) (*UnitResult, error) {
	return nil, errorx.New(errno.ErrUnImplement)
}

func (c *synapseClient) GetUnit(ctx context.Context, unitID string) (*UnitResult, error) {
	return nil, errorx.New(errno.ErrUnImplement)
}
