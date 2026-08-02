package service

import (
	"context"
	"time"

	"github.com/xh-polaris/psych-core-api/biz/application/dto/basic"
	"github.com/xh-polaris/psych-core-api/biz/application/dto/core_api"
	"github.com/xh-polaris/psych-core-api/biz/cst"
	"github.com/xh-polaris/psych-core-api/biz/domain/auth"
	"github.com/xh-polaris/psych-core-api/biz/infra/mapper/config"
	"github.com/xh-polaris/psych-core-api/biz/infra/mapper/voice"
	"github.com/xh-polaris/psych-core-api/biz/infra/util"
	"github.com/xh-polaris/psych-core-api/pkg/errorx"
	"github.com/xh-polaris/psych-core-api/pkg/logs"
	"github.com/xh-polaris/psych-core-api/types/enum"
	"github.com/xh-polaris/psych-core-api/types/errno"
	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/google/wire"
)

var _ IConfigService = (*ConfigService)(nil)

type IConfigService interface {
	ConfigCreate(ctx context.Context, req *core_api.ConfigCreateOrUpdateReq) (resp *basic.Response, err error)
	ConfigUpdate(ctx context.Context, req *core_api.ConfigCreateOrUpdateReq) (resp *basic.Response, err error)
	ConfigGetByUnitID(ctx context.Context, req *core_api.ConfigGetByUnitIdReq) (resp *core_api.ConfigGetByUnitIdResp, err error)
	ConfigGetByUnitID4Engine(ctx context.Context, unitId string) (*core_api.ConfigVO, error)
	ConfigGetCharacters(ctx context.Context, req *core_api.ConfigGetCharacterReq) (resp *core_api.ConfigGetCharacterResp, err error)
	ConfigListVoice(ctx context.Context, req *core_api.ConfigListVoiceReq) (*core_api.ConfigListVoiceResp, error)
	AddCharacter(ctx context.Context, unitId string, ch *AddCharacterReq) (*AddCharacterResp, error)
	UpdateCharacter(ctx context.Context, unitId string, ch *UpdateCharacterReq) (*basic.Response, error)
	DeleteCharacter(ctx context.Context, unitId, characterId string) (*basic.Response, error)
}

type ConfigService struct {
	AuthDomain   auth.IAuthDomain
	ConfigMapper config.IMongoMapper
	VoiceMapper  voice.IMongoMapper
}

var ConfigServiceSet = wire.NewSet(
	wire.Struct(new(ConfigService), "*"),
	wire.Bind(new(IConfigService), new(*ConfigService)),
)

func (c *ConfigService) ConfigCreate(ctx context.Context, req *core_api.ConfigCreateOrUpdateReq) (resp *basic.Response, err error) {
	unitOID, err := bson.ObjectIDFromHex(req.Config.UnitId)
	if err != nil {
		return nil, errorx.New(errno.ErrInvalidParams, errorx.KV("field", "UnitID"), errorx.KV("value", "单位ID"))
	}
	_, isStaff, err := c.authCfg(ctx, req.Config.UnitId)
	if err != nil {
		return nil, err
	}
	if isStaff {
		strip4Unit(req)
	}

	now := time.Now()
	confDAO := &config.Config{
		ID:         bson.NewObjectID(),
		Type:       int(req.Config.Type),
		UnitID:     unitOID,
		Characters: characterReq2DB(req.Config.Characters),
		Scene:      req.Config.Scene,
		AlertPhone: req.Config.AlertPhone,
		Status:     enum.ConfigStatusActive,
		CreateTime: now,
		UpdateTime: now,
	}
	if cht := req.Config.GetChat(); cht != nil {
		confDAO.Chat = &config.Chat{
			Name: cht.Name, Description: cht.Description,
			Provider: cht.Provider, AppID: cht.AppId,
		}
	}
	if tts := req.Config.GetTts(); tts != nil {
		confDAO.TTS = &config.TTS{
			Name: tts.Name, Description: tts.Description,
			Provider: tts.Provider, AppID: tts.AppId, Speaker: tts.Speaker,
		}
	}
	if rpt := req.Config.GetReport(); rpt != nil {
		confDAO.Report = &config.Report{
			Name: rpt.Name, Description: rpt.Description,
			Provider: rpt.Provider, AppID: rpt.AppId,
		}
	}
	if err = c.ConfigMapper.Insert(ctx, confDAO); err != nil {
		logs.Errorf("insert config error: %s", errorx.ErrorWithoutStack(err))
		return nil, err
	}
	return &basic.Response{Code: 0, Msg: "success"}, nil
}

// ConfigUpdate 更改单位配置
func (c *ConfigService) ConfigUpdate(ctx context.Context, req *core_api.ConfigCreateOrUpdateReq) (resp *basic.Response, err error) {
	unitOid, err := bson.ObjectIDFromHex(req.Config.UnitId)
	if err != nil {
		return nil, errorx.New(errno.ErrInvalidParams, errorx.KV("field", "单位ID"))
	}
	_, isStaff, err := c.authCfg(ctx, req.Config.UnitId)
	if err != nil {
		return nil, err
	}
	if isStaff {
		strip4Unit(req)
	}

	oldConf, err := c.ConfigMapper.FindOneByUnitID(ctx, unitOid)
	if err != nil || oldConf == nil {
		return c.ConfigCreate(ctx, req)
	}
	update := extractUpdateBSON(req)
	err = c.ConfigMapper.UpdateFields(ctx, oldConf.ID, update)
	if err != nil {
		logs.Errorf("update config error: %s", errorx.ErrorWithoutStack(err))
		return nil, err
	}
	return &basic.Response{Code: 0, Msg: "success"}, nil
}

func (c *ConfigService) ConfigGetByUnitID(ctx context.Context, req *core_api.ConfigGetByUnitIdReq) (resp *core_api.ConfigGetByUnitIdResp, err error) {
	if req.UnitId == "" {
		return nil, errorx.New(errno.ErrMissingParams, errorx.KV("field", "单位ID"))
	}
	uOid, err := bson.ObjectIDFromHex(req.UnitId)
	if err != nil {
		return nil, errorx.New(errno.ErrInvalidParams, errorx.KV("field", "单位ID"))
	}
	cfg, err := c.ConfigMapper.FindOneByUnitID(ctx, uOid)
	if err != nil {
		logs.Errorf("find config error: %s", errorx.ErrorWithoutStack(err))
		return nil, err
	}
	vo := configDB2VO(cfg)
	_, isStaff, err := c.authCfg(ctx, req.UnitId)
	if err != nil {
		return nil, err
	}
	if isStaff {
		mask4Unit(vo)
	}
	return &core_api.ConfigGetByUnitIdResp{Config: vo, Code: 0, Msg: "success"}, nil
}

// ConfigGetByUnitID4Engine 供对话引擎内部使用：返回引擎所需的最小字段
// （Type / Chat.Provider+AppId / TTS.Provider+AppId+Speaker / Report.Provider+AppId / Characters 子集），
// 不做鉴权、不填 AlertPhone/Scene 等敏感字段。
func (c *ConfigService) ConfigGetByUnitID4Engine(ctx context.Context, unitId string) (*core_api.ConfigVO, error) {
	if unitId == "" {
		return nil, errorx.New(errno.ErrMissingParams, errorx.KV("field", "单位ID"))
	}
	uOid, err := bson.ObjectIDFromHex(unitId)
	if err != nil {
		return nil, errorx.New(errno.ErrInvalidParams, errorx.KV("field", "单位ID"))
	}
	cfg, err := c.ConfigMapper.FindOneByUnitID(ctx, uOid)
	if err != nil {
		logs.Errorf("find config error: %s", errorx.ErrorWithoutStack(err))
		return nil, err
	}
	vo := &core_api.ConfigVO{
		Type: int32(cfg.Type),
	}
	if cfg.Chat != nil {
		vo.Chat = &core_api.ChatApp{
			Provider: cfg.Chat.Provider,
			AppId:    cfg.Chat.AppID,
		}
	}
	if cfg.TTS != nil {
		vo.Tts = &core_api.TTSApp{
			Provider: cfg.TTS.Provider,
			AppId:    cfg.TTS.AppID,
			Speaker:  cfg.TTS.Speaker,
		}
	}
	if cfg.Report != nil {
		vo.Report = &core_api.ReportApp{
			Provider: cfg.Report.Provider,
			AppId:    cfg.Report.AppID,
		}
	}
	vo.Characters = make([]*core_api.Character, len(cfg.Characters))
	for i, ch := range cfg.Characters {
		vo.Characters[i] = &core_api.Character{
			Id:     ch.ID.Hex(),
			Name:   ch.Name,
			Voice:  ch.Voice,
			Image:  ch.Image,
			Status: int32(ch.Status),
		}
	}
	return vo, nil
}

// authCfg 鉴权并返回角色：isSA=超管, isStaff=单位管理员/教师
func (c *ConfigService) authCfg(ctx context.Context, uID string) (isSA bool, isStaff bool, err error) {
	m, err := c.AuthDomain.ExtraUserMeta(ctx)
	if err != nil {
		return false, false, err
	}
	if m.HasSuperAdminAuth() {
		return true, false, nil
	}
	if m.Role >= enum.UserRoleTeacher && m.UnitId == uID {
		return false, true, nil
	}
	return false, false, errorx.New(errno.ErrInsufficientAuth)
}

// strip4Unit 移除单位管理员/教师无权配置的字段
func strip4Unit(req *core_api.ConfigCreateOrUpdateReq) {
	req.Config.Chat = nil
	req.Config.Tts = nil
	req.Config.Report = nil
}

// ConfigGetCharacters 获取心理老师虚拟形象
func (c *ConfigService) ConfigGetCharacters(ctx context.Context, req *core_api.ConfigGetCharacterReq) (resp *core_api.ConfigGetCharacterResp, err error) {
	if req.UnitId == "" {
		return nil, errorx.New(errno.ErrMissingParams, errorx.KV("field", "单位ID"))
	}

	unitOid, err := bson.ObjectIDFromHex(req.UnitId)
	if err != nil {
		return nil, errorx.New(errno.ErrInvalidParams, errorx.KV("field", "单位ID"))
	}

	uConf, err := c.ConfigMapper.FindOneByUnitID(ctx, unitOid)
	if err != nil {
		logs.Errorf("find config error: %s", errorx.ErrorWithoutStack(err))
		return nil, errorx.WrapByCode(err, errno.ErrConfigNotFound, errorx.KV("unitId", req.UnitId))
	}

	return &core_api.ConfigGetCharacterResp{
		Characters: characterDB2Resp(uConf.Characters),
		Scene:      uConf.Scene,
		Code:       0,
		Msg:        "success",
	}, nil
}

// DEPRECATED
func validateCreateConfigReq(req *core_api.ConfigCreateOrUpdateReq) error {
	if req.Config == nil {
		return errorx.New(errno.ErrMissingParams, errorx.KV("field", "配置内容"))
	}
	// 基础字段
	if req.Config.UnitId == "" {
		return errorx.New(errno.ErrMissingParams, errorx.KV("field", "单位ID"))
	}

	// chat配置
	if req.Config.Chat != nil {
		if req.Config.Chat.Name == "" {
			return errorx.New(errno.ErrMissingParams, errorx.KV("field", "Chat名称"))
		}
		if req.Config.Chat.Description == "" {
			return errorx.New(errno.ErrMissingParams, errorx.KV("field", "Chat描述"))
		}
		if req.Config.Chat.Provider == "" {
			return errorx.New(errno.ErrMissingParams, errorx.KV("field", "Chat模型平台"))
		}
		if req.Config.Chat.AppId == "" {
			return errorx.New(errno.ErrMissingParams, errorx.KV("field", "Chat平台模型标识符"))
		}
	}

	// tts配置
	if req.Config.Tts != nil {
		if req.Config.Tts.Name == "" {
			return errorx.New(errno.ErrMissingParams, errorx.KV("field", "TTS名称"))
		}
		if req.Config.Tts.Description == "" {
			return errorx.New(errno.ErrMissingParams, errorx.KV("field", "TTS描述"))
		}
		if req.Config.Tts.Provider == "" {
			return errorx.New(errno.ErrMissingParams, errorx.KV("field", "TTS模型平台"))
		}
		if req.Config.Tts.AppId == "" {
			return errorx.New(errno.ErrMissingParams, errorx.KV("field", "TTS平台模型标识符"))
		}
		if req.Config.Tts.Speaker == "" {
			return errorx.New(errno.ErrMissingParams, errorx.KV("field", "语音形象"))
		}
	}

	// report配置
	if req.Config.Report != nil {
		if req.Config.Report.Name == "" {
			return errorx.New(errno.ErrMissingParams, errorx.KV("field", "Report名称"))
		}
		if req.Config.Report.Description == "" {
			return errorx.New(errno.ErrMissingParams, errorx.KV("field", "Report描述"))
		}
		if req.Config.Report.Provider == "" {
			return errorx.New(errno.ErrMissingParams, errorx.KV("field", "Report模型平台"))
		}
		if req.Config.Report.AppId == "" {
			return errorx.New(errno.ErrMissingParams, errorx.KV("field", "Report平台模型标识符"))
		}
	}

	return nil
}

func extractUpdateBSON(req *core_api.ConfigCreateOrUpdateReq) bson.M {
	setUpdate := bson.M{}
	now := time.Now().Unix()
	conf := req.GetConfig()

	if conf.Type == enum.ConfigTypeEnd2End || conf.Type == enum.ConfigTypeChain {
		setUpdate[cst.Type] = conf.Type
	}
	if conf.Status == enum.ConfigStatusActive || conf.Status == enum.ConfigStatusDeleted {
		setUpdate[cst.Status] = conf.Status
	}
	// chat配置
	if chat := conf.GetChat(); chat != nil {
		if chat.GetName() != "" {
			setUpdate["chat.name"] = chat.GetName()
		}
		if chat.GetDescription() != "" {
			setUpdate["chat.description"] = chat.GetDescription()
		}
		if chat.GetProvider() != "" {
			setUpdate["chat.provider"] = chat.GetProvider()
		}
		if chat.GetAppId() != "" {
			setUpdate["chat.app_id"] = chat.GetAppId()
		}
		setUpdate["chat.update_time"] = now
	}

	// characters配置
	if chars := conf.GetCharacters(); len(chars) > 0 {
		setUpdate["character"] = characterReq2DB(chars)
	}
	if scene := conf.GetScene(); len(scene) > 0 {
		setUpdate["scene"] = scene
	}
	if alertPhone := conf.GetAlertPhone(); len(alertPhone) > 0 {
		setUpdate["alert_phone"] = alertPhone
	}

	// tts配置
	if tts := conf.GetTts(); tts != nil {
		if tts.GetName() != "" {
			setUpdate["tts.name"] = tts.GetName()
		}
		if tts.GetDescription() != "" {
			setUpdate["tts.description"] = tts.GetDescription()
		}
		if tts.GetProvider() != "" {
			setUpdate["tts.provider"] = tts.GetProvider()
		}
		if tts.GetAppId() != "" {
			setUpdate["tts.app_id"] = tts.GetAppId()
		}
		if tts.GetSpeaker() != "" {
			setUpdate["tts.speaker"] = tts.GetSpeaker()
		}
		setUpdate["tts.update_time"] = now
	}

	// report配置
	if report := conf.GetReport(); report != nil {
		if report.GetName() != "" {
			setUpdate["report.name"] = report.GetName()
		}
		if report.GetDescription() != "" {
			setUpdate["report.description"] = report.GetDescription()
		}
		if report.GetProvider() != "" {
			setUpdate["report.provider"] = report.GetProvider()
		}
		if report.GetAppId() != "" {
			setUpdate["report.app_id"] = report.GetAppId()
		}
		setUpdate["report.update_time"] = now
	}

	// 文档级更新时间
	setUpdate["update_time"] = now

	return setUpdate
}

// 将数据库Config对象字段转化为DTO对象
func configDB2VO(cfg *config.Config) *core_api.ConfigVO {
	vo := &core_api.ConfigVO{
		UnitId:     cfg.UnitID.Hex(),
		Type:       int32(cfg.Type),
		Characters: characterDB2Resp(cfg.Characters),
		Scene:      cfg.Scene,
		AlertPhone: cfg.AlertPhone,
		Status:     int32(cfg.Status),
		CreateTime: cfg.CreateTime.Unix(),
		UpdateTime: cfg.UpdateTime.Unix(),
	}
	if cfg.Chat != nil {
		vo.Chat = &core_api.ChatApp{
			Name: cfg.Chat.Name, Description: cfg.Chat.Description,
			Provider: cfg.Chat.Provider, AppId: cfg.Chat.AppID,
		}
	}
	if cfg.TTS != nil {
		vo.Tts = &core_api.TTSApp{
			Name: cfg.TTS.Name, Description: cfg.TTS.Description,
			Provider: cfg.TTS.Provider, AppId: cfg.TTS.AppID,
			Speaker: cfg.TTS.Speaker,
		}
	}
	if cfg.Report != nil {
		vo.Report = &core_api.ReportApp{
			Name: cfg.Report.Name, Description: cfg.Report.Description,
			Provider: cfg.Report.Provider, AppId: cfg.Report.AppID,
		}
	}
	return vo
}

// mask4Unit 对单位管理员隐藏 chat/tts/report
func mask4Unit(vo *core_api.ConfigVO) {
	vo.Chat = nil
	vo.Tts = nil
	vo.Report = nil
}

func characterDB2Resp(in []*config.Character) []*core_api.Character {
	if in == nil {
		return nil
	}
	out := make([]*core_api.Character, len(in))
	for i, c := range in {
		out[i] = &core_api.Character{
			Id:       c.ID.Hex(),
			Name:     c.Name,
			Voice:    c.Voice,
			Image:    c.Image,
			Status:   int32(c.Status),
			Identity: c.Identity,
			Style:    c.Style,
			Greeting: c.Greeting,
		}
	}
	return out
}

func characterReq2DB(in []*core_api.Character) []*config.Character {
	if in == nil {
		return nil
	}
	out := make([]*config.Character, len(in))
	for i, c := range in {
		id := c.Id
		if id == "" {
			id = bson.NewObjectID().Hex()
		}
		oid, _ := bson.ObjectIDFromHex(id)
		out[i] = &config.Character{
			ID:       oid,
			Name:     c.Name,
			Voice:    c.Voice,
			Image:    c.Image,
			Status:   int(c.Status),
			Identity: c.Identity,
			Style:    c.Style,
			Greeting: c.Greeting,
		}
	}
	return out
}

type AddCharacterReq struct {
	Name     string `json:"name"`
	Voice    string `json:"voice"`
	Image    string `json:"image"`
	Identity string `json:"identity"`
	Style    string `json:"style"`
	Greeting string `json:"greeting"`
}

type AddCharacterResp struct {
	Code        int32  `json:"code"`
	Msg         string `json:"msg"`
	CharacterId string `json:"characterId"`
}

type UpdateCharacterReq struct {
	CharacterId string `json:"characterId"`
	Name        string `json:"name"`
	Voice       string `json:"voice"`
	Image       string `json:"image"`
	Identity    string `json:"identity"`
	Style       string `json:"style"`
	Greeting    string `json:"greeting"`
}

func (c *ConfigService) ConfigListVoice(ctx context.Context, req *core_api.ConfigListVoiceReq) (*core_api.ConfigListVoiceResp, error) {
	m, err := c.AuthDomain.ExtraUserMeta(ctx)
	if err != nil {
		return nil, err
	}

	// 暂仅支持超管在配置心理老师角色时查看音色列表
	if err := c.AuthDomain.VerifySuperAdmin(ctx, m); err != nil {
		return nil, err
	}

	pg := util.EnsurePaginationOptions(req.GetPaginationOptions())
	opt := util.PagedFindOpt(pg).SetSort(bson.D{{cst.VoiceType, 1}})

	voices, err := c.VoiceMapper.FindManyWithOption(ctx, bson.M{}, opt)
	if err != nil {
		logs.CtxErrorf(ctx, "[ConfigListVoice] mongo error: %v", err)
		return nil, errorx.New(errno.ErrInternalError, errorx.KV("field", "获取音色列表失败"))
	}

	total, err := c.VoiceMapper.CountByFields(ctx, bson.M{})
	if err != nil {
		logs.CtxErrorf(ctx, "[ConfigListVoice] count error: %v", err)
		return nil, errorx.New(errno.ErrInternalError, errorx.KV("field", "获取音色总数失败"))
	}

	res := make([]*core_api.VoiceItemVO, len(voices))
	for i, v := range voices {
		res[i] = &core_api.VoiceItemVO{
			Id:          v.ID.Hex(),
			VoiceType:   v.VoiceType,
			Name:        v.Name,
			Avatar:      v.Avatar,
			Gender:      v.Gender,
			Age:         v.Age,
			Description: v.Description,
			TrialUrl:    v.TrialURL,
			VolcanoId:   v.VolcanoID,
			ResourceId:  v.ResourceID,
			CreateTime:  v.CreateTime.Unix(),
			UpdateTime:  v.UpdateTime.Unix(),
		}
	}

	return &core_api.ConfigListVoiceResp{
		Code:       0,
		Msg:        "success",
		Voices:     res,
		Pagination: util.PaginationRes(int32(total), pg),
	}, nil
}

func (c *ConfigService) AddCharacter(ctx context.Context, unitId string, req *AddCharacterReq) (*AddCharacterResp, error) {
	m, err := c.AuthDomain.ExtraUserMeta(ctx)
	if err != nil {
		return nil, err
	}
	if err := c.AuthDomain.VerifySuperAdmin(ctx, m); err != nil {
		return nil, err
	}

	unitOID, err := bson.ObjectIDFromHex(unitId)
	if err != nil {
		return nil, errorx.New(errno.ErrInvalidParams, errorx.KV("field", "unitId"))
	}
	if req.Name == "" {
		return nil, errorx.New(errno.ErrMissingParams, errorx.KV("field", "角色名称"))
	}
	if req.Voice == "" {
		return nil, errorx.New(errno.ErrMissingParams, errorx.KV("field", "音色"))
	}

	cfg, err := c.ConfigMapper.FindOneByUnitID(ctx, unitOID)
	if err != nil || cfg == nil {
		return nil, errorx.New(errno.ErrNotFound, errorx.KV("field", "单位配置"))
	}

	chId := bson.NewObjectID()
	ch := &config.Character{
		ID:       chId,
		Name:     req.Name,
		Voice:    req.Voice,
		Image:    req.Image,
		Status:   enum.ConfigStatusActive,
		Identity: req.Identity,
		Style:    req.Style,
		Greeting: req.Greeting,
	}

	if err := c.ConfigMapper.PushCharacter(ctx, cfg.ID, ch); err != nil {
		logs.CtxErrorf(ctx, "[AddCharacter] mongo error: %v", err)
		return nil, errorx.New(errno.ErrInternalError, errorx.KV("field", "添加角色失败"))
	}

	return &AddCharacterResp{Code: 0, Msg: "success", CharacterId: chId.Hex()}, nil
}

func (c *ConfigService) UpdateCharacter(ctx context.Context, unitId string, req *UpdateCharacterReq) (*basic.Response, error) {
	m, err := c.AuthDomain.ExtraUserMeta(ctx)
	if err != nil {
		return nil, err
	}
	if err := c.AuthDomain.VerifySuperAdmin(ctx, m); err != nil {
		return nil, err
	}

	unitOID, err := bson.ObjectIDFromHex(unitId)
	if err != nil {
		return nil, errorx.New(errno.ErrInvalidParams, errorx.KV("field", "unitId"))
	}
	chOID, err := bson.ObjectIDFromHex(req.CharacterId)
	if err != nil {
		return nil, errorx.New(errno.ErrInvalidParams, errorx.KV("field", "characterId"))
	}

	cfg, err := c.ConfigMapper.FindOneByUnitID(ctx, unitOID)
	if err != nil || cfg == nil {
		return nil, errorx.New(errno.ErrNotFound, errorx.KV("field", "单位配置"))
	}

	setFields := bson.M{}
	if req.Name != "" {
		setFields["character.$.name"] = req.Name
	}
	if req.Voice != "" {
		setFields["character.$.voice"] = req.Voice
	}
	if req.Image != "" {
		setFields["character.$.image"] = req.Image
	}
	if req.Identity != "" {
		setFields["character.$.identity"] = req.Identity
	}
	if req.Style != "" {
		setFields["character.$.style"] = req.Style
	}
	if req.Greeting != "" {
		setFields["character.$.greeting"] = req.Greeting
	}
	if len(setFields) == 0 {
		return &basic.Response{Code: 0, Msg: "success"}, nil
	}

	if err := c.ConfigMapper.SetCharacter(ctx, cfg.ID, chOID, setFields); err != nil {
		logs.CtxErrorf(ctx, "[UpdateCharacter] mongo error: %v", err)
		return nil, errorx.New(errno.ErrInternalError, errorx.KV("field", "更新角色失败"))
	}

	return &basic.Response{Code: 0, Msg: "success"}, nil
}

func (c *ConfigService) DeleteCharacter(ctx context.Context, unitId, characterId string) (*basic.Response, error) {
	m, err := c.AuthDomain.ExtraUserMeta(ctx)
	if err != nil {
		return nil, err
	}
	if err := c.AuthDomain.VerifySuperAdmin(ctx, m); err != nil {
		return nil, err
	}

	unitOID, err := bson.ObjectIDFromHex(unitId)
	if err != nil {
		return nil, errorx.New(errno.ErrInvalidParams, errorx.KV("field", "unitId"))
	}
	chOID, err := bson.ObjectIDFromHex(characterId)
	if err != nil {
		return nil, errorx.New(errno.ErrInvalidParams, errorx.KV("field", "characterId"))
	}

	cfg, err := c.ConfigMapper.FindOneByUnitID(ctx, unitOID)
	if err != nil || cfg == nil {
		return nil, errorx.New(errno.ErrNotFound, errorx.KV("field", "单位配置"))
	}

	if err := c.ConfigMapper.SetCharacterStatus(ctx, cfg.ID, chOID, enum.ConfigStatusDeleted); err != nil {
		logs.CtxErrorf(ctx, "[DeleteCharacter] mongo error: %v", err)
		return nil, errorx.New(errno.ErrInternalError, errorx.KV("field", "删除角色失败"))
	}

	return &basic.Response{Code: 0, Msg: "success"}, nil
}
