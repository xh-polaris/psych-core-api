package engine

import (
	"github.com/xh-polaris/psych-core-api/biz/application/dto/core_api"
	"github.com/xh-polaris/psych-core-api/biz/cst"
	"github.com/xh-polaris/psych-core-api/biz/infra/util"
	"github.com/xh-polaris/psych-core-api/pkg/core"
	"github.com/xh-polaris/psych-core-api/pkg/errorx"
	"github.com/xh-polaris/psych-core-api/pkg/logs"
	"github.com/xh-polaris/psych-core-api/types/errno"
	"go.mongodb.org/mongo-driver/v2/bson"
)

// auth 验证用户信息 [engine]
func (e *Engine) auth(auth *core.Auth) (bool, error) {
	var merr *core.Err
	var alreadyAuth *core.Auth // 返回额外信息

	switch auth.AuthType {
	case int(core.AlreadyAuth): // 已经在其他环节登录过
		alreadyAuth, merr = e.already(auth)
	default:
		alreadyAuth, merr = e.unAuth(auth)
	}

	if merr != nil {
		return false, e.MWrite(core.MErr, merr)
	}
	e.info = alreadyAuth.Info
	if cid, ok := alreadyAuth.Info[cst.JsonConversationID]; ok {
		e.uSession = cid.(string)
	} else {
		e.uSession = bson.NewObjectID().Hex()
	}
	util.DPrint("[engine] [auth] info: %+v, merr: %+v, uSession: %s\n", alreadyAuth, merr, e.uSession) // debug
	return true, e.MWrite(core.MAuth, alreadyAuth)                                                     // 前端收到Auth响应后, 需要显示配置中
}

// 已登录
func (e *Engine) already(auth *core.Auth) (alreadyAuth *core.Auth, merr *core.Err) {
	alreadyAuth = &core.Auth{}
	claims, err := util.ParseJwt(auth.VerifyCode)
	if err != nil {
		// ParseJwt 已返回带 code 的错误（如 ErrUnAuth），这里直接透传
		return nil, core.ToErr(err)
	}
	// 提取字段
	alreadyAuth.Info = auth.Info
	e.info = alreadyAuth.Info
	e.info[cst.JsonUserID] = claims[cst.JsonUnitID].(string)
	e.info[cst.JsonCode] = claims[cst.JsonUserID].(string)
	e.info[cst.JsonCode] = claims[cst.JsonCode].(string)
	return alreadyAuth, nil
}

// 通过注入的 UserService 进行未登录校验
func (e *Engine) unAuth(auth *core.Auth) (alreadyAuth *core.Auth, merr *core.Err) {
	var err error
	alreadyAuth = &core.Auth{}

	// 调用注入的 UserService 做登录校验
	if e.usrSvc == nil {
		logs.Error("[engine] [unAuth] user service is nil")
		return nil, core.ToErr(errorx.New(errno.ErrInternalError))
	}
	signReq := &core_api.UserSignInReq{
		UnitId:     auth.Info[cst.JsonUnitID].(string),
		AuthType:   int32(auth.AuthType),
		AuthId:     auth.AuthID,
		VerifyCode: auth.VerifyCode,
	}
	signResp, err := e.usrSvc.UserSignIn(e.ctx, signReq)
	if err != nil {
		logs.Errorf("[engine] [%s] UserSignIn err: %v", core.AAuth, err)
		// UserService 已返回带业务 code 的错误，直接透传
		merr = core.ToErr(err)
		return
	}

	alreadyAuth.Info = auth.Info
	if alreadyAuth.Info == nil {
		alreadyAuth.Info = make(map[string]any)
	}
	alreadyAuth.Info[cst.JsonUnitID] = signResp.UnitId
	alreadyAuth.Info[cst.JsonUserID] = signResp.UserId
	alreadyAuth.Info[cst.JsonCode] = signResp.CodeValue
	e.info = alreadyAuth.Info
	return
}
