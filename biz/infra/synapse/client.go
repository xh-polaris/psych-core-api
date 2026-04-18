package synapse

import (
	"context"
)

type LoginResult struct {
	BasicUserID string
	Token       string
	IsNew       bool
	UnitID      string
	Phone       string
	Email       string
	StudentID   string
	Name        string
}

type RegisterResult struct {
	Token string
}

type UnitResult struct {
	UnitID     string
	Name       string
	CreateTime int64
	UpdateTime int64
}

type Client interface {
	Login(ctx context.Context, authType, authId, extraAuthId, verify string) (*LoginResult, error)
	Register(ctx context.Context, authType, authId, extraAuthId, verify, password string) (*RegisterResult, error)
	ResetPassword(ctx context.Context, authorization, newPassword, resetKey, basicUserId string) error
	SendVerifyCode(ctx context.Context, authType, authId, cause string) error
	CheckVerifyCode(ctx context.Context, authType, authId, cause, verify string) error
	ThirdPartyLogin(ctx context.Context, thirdparty, ticket string) (*LoginResult, error)
	CreateBasicUser(ctx context.Context, unitID, code, phone, email, password string, encryptType int64) (*synapseBasicUser, error)
	CreateUnit(ctx context.Context, name string) (*UnitResult, error)
	GetUnit(ctx context.Context, unitID string) (*UnitResult, error)
}
