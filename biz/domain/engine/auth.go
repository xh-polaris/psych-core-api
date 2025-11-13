package engine

import (
	"github.com/xh-polaris/psych-core-api/biz/infra/cst"
	"github.com/xh-polaris/psych-core-api/biz/infra/rpc"
	"github.com/xh-polaris/psych-core-api/biz/infra/utils"
	"github.com/xh-polaris/psych-core-api/biz/infra/utils/enum"
	"github.com/xh-polaris/psych-idl/kitex_gen/profile"
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
	utils.DPrint("[engine] [auth] info: %+v, merr: %+v\n", alreadyAuth, merr) // debug
	return true, e.MWrite(core.MAuth, alreadyAuth)                            // 前端收到Auth响应后, 需要显示配置中
}

// 已登录
func (e *Engine) already(auth *core.Auth) (alreadyAuth *core.Auth, merr *core.Err) {
	alreadyAuth = &core.Auth{}
	claims, err := utils.ParseJwt(auth.VerifyCode)
	if err != nil {
		return nil, cst.Err(cst.JwtAuthErr)
	}
	// 提取字段
	alreadyAuth.Info = auth.Info
	e.info = alreadyAuth.Info
	e.info[cst.UnitId] = claims[cst.UnitId].(string)
	e.info[cst.UserId] = claims[cst.UserId].(string)
	e.info[cst.Code] = claims[cst.Code].(string)
	return alreadyAuth, nil
}

func (e *Engine) unAuth(auth *core.Auth) (alreadyAuth *core.Auth, merr *core.Err) {
	var err error
	var signResp *profile.UserSignInResp
	var getResp *profile.UserGetInfoResp
	pp, alreadyAuth := rpc.GetPsychProfile(), &core.Auth{}
	// 获得枚举值
	authTypeStr, ok := enum.GetAuthType(int(auth.AuthType))
	if !ok {
		logx.Error("[engine] [%s] AuthType not found: %d", core.AAuth, auth.AuthType)
		merr = cst.Err(cst.InvalidAuth)
		return
	}

	// 用户登录
	sign := &profile.UserSignInReq{
		UnitId:    auth.Info[cst.UnitId].(string),
		AuthType:  authTypeStr,
		AuthId:    auth.AuthID,
		AuthValue: auth.VerifyCode,
	}
	if signResp, err = pp.UserSignIn(e.ctx, sign); err != nil {
		logx.Error("[engine] [%s] UserSignIn err: %v", core.AAuth, err)
		merr = cst.Err(cst.InvalidAuth)
		return
	}

	// 获取用户信息
	get := &profile.UserGetInfoReq{
		UserId: signResp.UserId,
	}
	if getResp, err = pp.UserGetInfo(e.ctx, get); err != nil {
		logx.Error("[engine] [%s] UserGetInfo err: %v", core.AAuth, err)
		merr = cst.Err(cst.InvalidAuth)
		return
	}
	//form, err := utils.Anypb2Any(getResp.Form)
	//if err != nil {
	//	logx.Error("[engine] [%s] UserGetInfo err: %v", core.AAuth, err)
	//	merr = cst.Err(cst.InvalidAuth)
	//	return
	//}
	//alreadyAuth.Info = form
	alreadyAuth.Info[cst.UnitId] = signResp.UnitId
	alreadyAuth.Info[cst.UserId] = signResp.UserId
	alreadyAuth.Info[cst.Code] = getResp.User.Code
	e.info = alreadyAuth.Info
	return
}
