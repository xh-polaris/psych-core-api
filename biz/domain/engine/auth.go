package engine

import (
	"github.com/xh-polaris/psych-core-api/biz/cst"
	"github.com/xh-polaris/psych-core-api/biz/infra/rpc"
	"github.com/xh-polaris/psych-core-api/biz/infra/util"
	"github.com/xh-polaris/psych-core-api/pkg/errorx"
	"github.com/xh-polaris/psych-core-api/types/errno"
	"github.com/xh-polaris/psych-idl/kitex_gen/user"
	"github.com/xh-polaris/psych-pkg/core"
	"github.com/xh-polaris/psych-pkg/util/logx"
)

// auth 验证用户信息 [engine]
func (e *Engine) auth(auth *core.Auth) (bool, error) {
	var merr *core.Err
	var alreadyAuth *core.Auth // 返回额外信息

	switch auth.AuthType {
	case core.AlreadyAuth: // 已经在其他环节登录过
		alreadyAuth, merr = e.already(auth)
	default:
		alreadyAuth, merr = e.unAuth(auth)
	}

	if merr != nil {
		return false, e.MWrite(core.MErr, merr)
	}
	e.info = alreadyAuth.Info
	util.DPrint("[engine] [auth] info: %+v, merr: %+v\n", alreadyAuth, merr) // debug
	return true, e.MWrite(core.MAuth, alreadyAuth)                           // 前端收到Auth响应后, 需要显示配置中
}

// 已登录
func (e *Engine) already(auth *core.Auth) (alreadyAuth *core.Auth, merr *core.Err) {
	alreadyAuth = &core.Auth{}
	claims, err := util.ParseJwt(auth.VerifyCode)
	if err != nil {
		return nil, util.Err(errorx.WrapByCode(err, errno.InvalidAuth))
	}
	// 提取字段
	alreadyAuth.Info = auth.Info
	e.info = alreadyAuth.Info
	e.info[cst.JWTUnitId] = claims[cst.JWTUnitId].(string)
	e.info[cst.UserId] = claims[cst.UserId].(string)
	e.info[cst.StudentId] = claims[cst.StudentId].(string)
	e.info[cst.Strong] = claims[cst.Strong].(bool)
	return alreadyAuth, nil
}

func (e *Engine) unAuth(auth *core.Auth) (alreadyAuth *core.Auth, merr *core.Err) {
	var err error
	var signResp *user.UserSignInResp
	var getResp *user.UserGetInfoResp
	pu, alreadyAuth := rpc.GetPsychUser(), &core.Auth{}
	// 用户登录
	sign := &user.UserSignInReq{UnitId: auth.Info[cst.JWTUnitId].(string),
		AuthType: auth.AuthType, AuthId: auth.AuthID, VerifyCode: auth.VerifyCode}
	if signResp, err = pu.UserSignIn(e.ctx, sign); err != nil {
		logx.Error("[engine] [%s] UserSignIn err: %v", core.AAuth, err)
		merr = util.Err(errorx.WrapByCode(err, errno.InvalidAuth))
		return
	}

	// 获取用户信息
	get := &user.UserGetInfoReq{UserId: signResp.UserId, UnitId: &signResp.UnitId}
	if getResp, err = pu.UserGetInfo(e.ctx, get); err != nil {
		logx.Error("[engine] [%s] UserGetInfo err: %v", core.AAuth, err)
		merr = util.Err(errorx.WrapByCode(err, errno.InvalidAuth))
		return
	}
	form, err := util.Anypb2Any(getResp.Form)
	if err != nil {
		logx.Error("[engine] [%s] UserGetInfo err: %v", core.AAuth, err)
		merr = util.Err(errorx.WrapByCode(err, errno.InvalidAuth))
		return
	}
	alreadyAuth.Info = form
	alreadyAuth.Info[cst.JWTUnitId] = signResp.UnitId
	alreadyAuth.Info[cst.UserId] = signResp.UserId
	alreadyAuth.Info[cst.StudentId] = *signResp.StudentId
	alreadyAuth.Info[cst.Strong] = signResp.Strong
	e.info = alreadyAuth.Info
	return
}
