package service

import (
	"context"
	"github.com/google/wire"
	core_api "github.com/xh-polaris/psych-core-api/biz/application/dto/core_api"
	"github.com/xh-polaris/psych-core-api/biz/infra/config"
	cst "github.com/xh-polaris/psych-core-api/biz/infra/consts"
	"github.com/xh-polaris/psych-core-api/biz/infra/rpc"
	"github.com/xh-polaris/psych-core-api/biz/infra/utils"
	u "github.com/xh-polaris/psych-idl/kitex_gen/user"
)

type IAuthService interface {
	SignIn(ctx context.Context, req *u.UserSignInReq) (resp *u.UserSignInResp, err error)
}

type AuthService struct {
}

var AuthServiceSet = wire.NewSet(
	wire.Struct(new(AuthService), "*"),
	wire.Bind(new(IAuthService), new(*AuthService)),
)

func (s AuthService) SignIn(ctx context.Context, req *core_api.UserSignInReq) (resp *core_api.UserSignInResp, err error) {
	// 调用接口
	client := rpc.GetPsychUser()
	userResp, err := client.UserSignIn(ctx, &u.UserSignInReq{
		UnitId:     req.UnitId,
		AuthType:   req.AuthType,
		AuthId:     req.AuthId,
		VerifyCode: req.VerifyCode,
	})
	if err != nil {
		return nil, cst.InvalidAuth
	}

	jwt, err := utils.GenerateJwt(config.GetConfig().Auth.SecretKey, map[string]any{
		cst.UnitId:    userResp.UnitId,
		cst.UserId:    userResp.UserId,
		cst.StudentId: userResp.StudentId,
		cst.Strong:    userResp.Strong,
	})

	resp = &core_api.UserSignInResp{
		UnitId:    userResp.UnitId,
		UserId:    userResp.UserId,
		StudentId: userResp.StudentId,
		Strong:    userResp.Strong,
		Token:     jwt,
	}

	return resp, nil
}
