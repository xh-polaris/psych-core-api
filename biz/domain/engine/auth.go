package engine

import (
	"github.com/xh-polaris/psych-core-api/biz/infra/consts"
	"github.com/xh-polaris/psych-core-api/biz/infra/rpc"
	"github.com/xh-polaris/psych-idl/kitex_gen/user"
	"github.com/xh-polaris/psych-pkg/core"
	"github.com/xh-polaris/psych-pkg/util/logx"
)

// buildAuth
func buildAuth(e *Engine) {
	e.info = make(map[string]string)
}

// auth 验证用户信息, 串行
func (e *Engine) auth(auth *core.Auth) bool {
	var merr *core.Err
	var alreadyAuth *core.Auth // 返回额外信息

	switch auth.AuthType {
	case core.AlreadyAuth: // 已经在其他环节登录过
		alreadyAuth, merr = e.already(auth)
	default:
		alreadyAuth, merr = e.unAuth(auth)
	}

	if merr != nil {
		e.MWrite(core.MErr, merr)
		return false
	}
	e.info = alreadyAuth.Info
	e.MWrite(core.MAuth, alreadyAuth) // 前端收到Auth响应后, 需要显示配置中
	return true
}

// 已登录
func (e *Engine) already(auth *core.Auth) (alreadyAuth *core.Auth, merr *core.Err) {
	alreadyAuth = &core.Auth{}
	// TODO 校验JWT正确性
	merr = consts.Err(consts.InvalidAuth)
	alreadyAuth.Info = auth.Info
	e.info = alreadyAuth.Info
	return alreadyAuth, merr
}

func (e *Engine) unAuth(auth *core.Auth) (alreadyAuth *core.Auth, merr *core.Err) {
	var err error
	var signResp *user.UserSignInResp
	var getResp *user.UserGetInfoResp
	pu, alreadyAuth := rpc.GetPsychUser(), &core.Auth{}
	// 用户登录
	sign := &user.UserSignInReq{UnitId: auth.Info[consts.UnitId],
		AuthType: auth.AuthType, AuthId: auth.AuthID, VerifyCode: auth.VerifyCode}
	if signResp, err = pu.UserSignIn(e.ctx, sign); err != nil {
		logx.Error("[engine] [%s] UserSignIn err: %v", core.AAuth, err)
		merr = consts.Err(consts.InvalidAuth)
		return
	}

	// 获取用户信息
	get := &user.UserGetInfoReq{UserId: signResp.UserId, UnitId: &signResp.UnitId}
	if getResp, err = pu.UserGetInfo(e.ctx, get); err != nil {
		logx.Error("[engine] [%s] UserGetInfo err: %v", core.AAuth, err)
		merr = consts.Err(consts.InvalidAuth)
		return
	}
	alreadyAuth.Info = getResp.Form
	e.info = alreadyAuth.Info
	return
}
