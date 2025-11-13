package service

import (
	"context"

	"github.com/google/wire"
	"github.com/xh-polaris/psych-core-api/biz/domain/usr"
	"github.com/xh-polaris/psych-core-api/biz/infra/cst"
	"github.com/xh-polaris/psych-core-api/biz/infra/rpc"
	"github.com/xh-polaris/psych-core-api/biz/infra/utils"
	"github.com/xh-polaris/psych-idl/kitex_gen/core_api"
	"github.com/xh-polaris/psych-idl/kitex_gen/profile"
	"github.com/xh-polaris/psych-pkg/util/logx"
)

type IAuthService interface {
	SignIn(ctx context.Context, req *profile.UserSignInReq) (resp *profile.UserSignInResp, err error)
}

type AuthService struct {
}

var AuthServiceSet = wire.NewSet(
	wire.Struct(new(AuthService), "*"),
	wire.Bind(new(IAuthService), new(*AuthService)),
)

func (s AuthService) UserSignIn(ctx context.Context, req *core_api.UserSignInReq) (resp *core_api.UserSignInResp, err error) {
	// 调用接口
	client := rpc.GetPsychProfile()
	userResp, err := client.UserSignIn(ctx, &profile.UserSignInReq{
		UnitId:    req.UnitId,
		AuthType:  req.AuthType,
		AuthId:    req.AuthId,
		AuthValue: req.AuthValue,
	})
	if err != nil {
		return nil, cst.InvalidAuth
	}

	jwt, err := utils.GenerateJwt(map[string]any{
		cst.UnitId: userResp.UnitId,
		cst.UserId: userResp.UserId,
		cst.Code:   req.AuthId,
	})

	resp = &core_api.UserSignInResp{
		Code:      0,
		Msg:       "success",
		UnitId:    userResp.UnitId,
		UserId:    userResp.UserId,
		CodeValue: req.AuthId,
		Token:     jwt,
	}

	return resp, nil
}

func (s AuthService) UserGetInfo(ctx context.Context, _ *core_api.UserGetInfoReq) (resp *core_api.UserGetInfoResp, err error) {
	var meta *usr.Meta
	if meta, err = utils.ExtraUserMeta(ctx); err != nil {
		return nil, cst.ExpireAuth
	}

	// 获取用户信息
	get := &profile.UserGetInfoReq{UserId: meta.UserId}
	getResp, err := rpc.GetPsychProfile().UserGetInfo(ctx, get)
	if err != nil {
		logx.Error("[auth service] get user %s info err:", meta.UserId, err)
		return nil, cst.ExpireAuth
	}
	// 构造响应
	r := &core_api.UserGetInfoResp{
		User: &profile.User{
			Id:         getResp.User.Id,
			CodeType:   getResp.User.CodeType,
			Code:       getResp.User.Code,
			UnitId:     getResp.User.UnitId,
			Name:       getResp.User.Name,
			Birth:      getResp.User.Birth,
			Gender:     getResp.User.Gender,
			Status:     getResp.User.Status,
			EnrollYear: getResp.User.EnrollYear,
			Grade:      getResp.User.Grade,
			Class:      getResp.User.Class,
			CreateTime: getResp.User.CreateTime,
			UpdateTime: getResp.User.UpdateTime,
		},
		Code: 0,
		Msg:  "success",
	}

	return r, nil
}
