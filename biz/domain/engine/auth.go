package engine

import (
	"github.com/xh-polaris/psych-core-api/biz/application/dto/core_api"
	"github.com/xh-polaris/psych-core-api/biz/cst"
	"github.com/xh-polaris/psych-core-api/biz/domain/his"
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
	case core.AlreadyAuth: // 已经在其他环节登录过
		alreadyAuth, merr = e.already(auth)
	default:
		alreadyAuth, merr = e.unAuth(auth)
	}

	if merr != nil {
		return false, e.MWrite(core.MErr, merr)
	}
	e.info = alreadyAuth.Info
	// 前端契约：先 CreateConversation，再带返回的 conversationId 建 WS。
	// conversationId 为必填；不传直接报参数错误，避免生成临时 ID 导致首次写消息静默失败。
	cid, ok := alreadyAuth.Info[cst.JsonConversationID]
	if !ok {
		return false, e.MWrite(core.MErr, core.ToErr(errorx.New(errno.ErrInvalidParams, errorx.KV("field", "conversationId"))))
	}
	e.uSession = cid.(string)

	// 校验 conversationId 归属：前端透传的会话 ID 不可信，
	// 必须确认其属于当前登录用户，防止越权读取/污染他人会话。
	if err := e.verifyConversationOwnership(); err != nil {
		return false, e.MWrite(core.MErr, core.ToErr(err))
	}

	// 记录初始消息总数，用于后续对比是否有新消息产生
	if msgs, err := his.Mgr.GetConversationMessages(e.ctx, e.uSession, -1); err == nil {
		e.initialCount = len(msgs)
	}

	logs.Infof("[engine] [auth] info: %+v, merr: %+v, uSession: %s, initialCount: %d", alreadyAuth, merr, e.uSession, e.initialCount)
	return true, e.MWrite(core.MAuth, alreadyAuth) // 前端收到Auth响应后, 需要显示配置中
}

// verifyConversationOwnership 校验 e.uSession 对应的会话是否归属于当前登录用户
func (e *Engine) verifyConversationOwnership() error {
	convOID, err := bson.ObjectIDFromHex(e.uSession)
	if err != nil {
		// 非法会话 ID，直接拒绝
		return errorx.New(errno.ConvNotOwned)
	}

	userID := e.getID(e.info, cst.JsonUserID)
	if userID == "" {
		return errorx.New(errno.ErrUnAuth)
	}
	userOID, err := bson.ObjectIDFromHex(userID)
	if err != nil {
		return errorx.New(errno.ErrUnAuth)
	}

	owned, err := e.convMapper.IsOwnedBy(e.ctx, convOID, userOID)
	if err != nil {
		logs.Errorf("[engine] [auth] verify conversation ownership err: %s", errorx.ErrorWithoutStack(err))
		return errorx.WrapByCode(err, errno.ErrGetConversation)
	}
	if !owned {
		logs.Warnf("[engine] [auth] conversation %s does not belong to user %s, reject", e.uSession, userID)
		return errorx.New(errno.ConvNotOwned)
	}
	return nil
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
	e.info[cst.JsonUnitID] = claims[cst.JsonUnitID].(string)
	e.info[cst.JsonUserID] = claims[cst.JsonUserID].(string)
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
		AuthType:   auth.AuthType,
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
