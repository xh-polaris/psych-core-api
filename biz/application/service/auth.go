package service

import (
	"context"

	"github.com/google/wire"
	"github.com/xh-polaris/psych-core-api/biz/application/dto/core_api"
	"github.com/xh-polaris/psych-core-api/biz/cst"
	"github.com/xh-polaris/psych-core-api/biz/domain/usr"
	"github.com/xh-polaris/psych-core-api/biz/infra/rpc"
	"github.com/xh-polaris/psych-core-api/biz/infra/util"
	"github.com/xh-polaris/psych-core-api/pkg/errorx"
	"github.com/xh-polaris/psych-core-api/types/errno"
	"github.com/xh-polaris/psych-idl/kitex_gen/user"
	u "github.com/xh-polaris/psych-idl/kitex_gen/user"
	"github.com/xh-polaris/psych-pkg/util/logx"
	"google.golang.org/protobuf/types/known/structpb"
)

type IAuthService interface {
	UserSignIn(ctx context.Context, req *core_api.UserSignInReq) (resp *core_api.UserSignInResp, err error)
	UserGetInfo(ctx context.Context, _ *core_api.UserGetInfoReq) (resp *core_api.UserGetInfoResp, err error)
}

type AuthService struct {
}

var AuthServiceSet = wire.NewSet(
	wire.Struct(new(AuthService), "*"),
	wire.Bind(new(IAuthService), new(*AuthService)),
)

func (s AuthService) UserSignIn(ctx context.Context, req *core_api.UserSignInReq) (resp *core_api.UserSignInResp, err error) {
	// 调用接口
	client := rpc.GetPsychUser()
	userResp, err := client.UserSignIn(ctx, &u.UserSignInReq{
		UnitId:     req.UnitId,
		AuthType:   req.AuthType,
		AuthId:     req.AuthId,
		VerifyCode: req.VerifyCode,
	})
	if err != nil {
		return nil, errorx.New(errno.InvalidAuth)
	}

	jwt, err := util.GenerateJwt(map[string]any{
		cst.JWTUnitId:    userResp.UnitId,
		cst.JWTUserId:    userResp.UserId,
		cst.JWTStudentId: userResp.StudentId,
	})

	resp = &core_api.UserSignInResp{
		Code:      0,
		Msg:       "success",
		UnitId:    userResp.UnitId,
		UserId:    userResp.UserId,
		StudentId: userResp.StudentId,
		Strong:    userResp.Strong,
		Token:     jwt,
	}

	return resp, nil
}

func (s AuthService) UserGetInfo(ctx context.Context, _ *core_api.UserGetInfoReq) (resp *core_api.UserGetInfoResp, err error) {
	var meta *usr.Meta
	if meta, err = util.ExtraUserMeta(ctx); err != nil {
		return nil, errorx.WrapByCode(err, errno.ExpireAuth)
	}

	// 获取用户信息
	get := &user.UserGetInfoReq{UserId: meta.UserId, UnitId: &meta.UnitId}
	getResp, err := rpc.GetPsychUser().UserGetInfo(ctx, get)
	if err != nil {
		logx.Error("[auth service] get user %s info err:", meta.UserId, err)
		return nil, errorx.WrapByCode(err, errno.ExpireAuth)
	}
	// 构造响应
	r := &core_api.UserGetInfoResp{
		User: &core_api.User{
			Id:         getResp.User.Id,
			Phone:      getResp.User.Phone,
			Name:       getResp.User.Name,
			Birth:      getResp.User.Birth,
			Gender:     getResp.User.Gender,
			Status:     getResp.User.Status,
			CreateTime: getResp.User.CreateTime,
			UpdateTime: getResp.User.UpdateTime,
		},
		Code: 0,
		Msg:  "success",
	}
	data, err := util.Anypb2Any(getResp.Form)
	if err != nil {
		return nil, err
	}
	if r.Info, err = structpb.NewStruct(data); err != nil {
		return nil, err
	}
	return r, nil
}
