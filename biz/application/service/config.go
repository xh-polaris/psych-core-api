package service

import (
	"context"
	"time"

	"github.com/xh-polaris/psych-core-api/biz/application/dto/basic"
	"github.com/xh-polaris/psych-core-api/biz/application/dto/core_api"
	"github.com/xh-polaris/psych-core-api/biz/cst"
	"github.com/xh-polaris/psych-core-api/biz/domain/auth"
	"github.com/xh-polaris/psych-core-api/biz/infra/mapper/config"
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
	ConfigGetCharacters(ctx context.Context, req *core_api.ConfigGetCharacterReq) (resp *core_api.ConfigGetCharacterResp, err error)
}

type ConfigService struct {
	AuthDomain   auth.IAuthDomain
	ConfigMapper config.IMongoMapper
}

var ConfigServiceSet = wire.NewSet(
	wire.Struct(new(ConfigService), "*"),
	wire.Bind(new(IConfigService), new(*ConfigService)),
)

func (c *ConfigService) ConfigCreate(ctx context.Context, req *core_api.ConfigCreateOrUpdateReq) (resp *basic.Response, err error) {
	// 参数合法性校验
	unitOID, err := bson.ObjectIDFromHex(req.Config.UnitId)
	if err != nil {
		return nil, errorx.New(errno.ErrInvalidParams, errorx.KV("field", "UnitID"), errorx.KV("value", "单位ID"))
	}

	// 鉴权
	usrMeta, err := c.AuthDomain.ExtraUserMeta(ctx)
	if err != nil {
		return nil, err
	}
	if !usrMeta.HasUnitAdminAuth(req.Config.UnitId) {
		return nil, errorx.New(errno.ErrInsufficientAuth)
	}

	// 构造并插入数据库
	now := time.Now()
	confDAO := &config.Config{
		ID:         bson.NewObjectID(),
		Type:       int(req.Config.Type),
		UnitID:     unitOID,
		Characters: characterReq2DB(req.Config.Characters),
		Scene:      req.Config.Scene,
		AlertPhone: req.Config.AlertPhone,
		Chat: &config.Chat{
			Name:        req.Config.Chat.Name,
			Description: req.Config.Chat.Description,
			Provider:    req.Config.Chat.Provider,
			AppID:       req.Config.Chat.AppId,
		},
		TTS: &config.TTS{
			Name:        req.Config.Tts.Name,
			Description: req.Config.Tts.Description,
			Provider:    req.Config.Tts.Provider,
			AppID:       req.Config.Tts.AppId,
			Speaker:     req.Config.Tts.Speaker,
		},
		Report: &config.Report{
			Name:        req.Config.Report.Name,
			Description: req.Config.Report.Description,
			Provider:    req.Config.Report.Provider,
			AppID:       req.Config.Report.AppId,
		},
		Status:     enum.ConfigStatusActive,
		CreateTime: now,
		UpdateTime: now,
	}
	// 插入数据库
	if err = c.ConfigMapper.Insert(ctx, confDAO); err != nil {
		logs.Errorf("insert config error: %s", errorx.ErrorWithoutStack(err))
		return nil, err
	}
	// 构造返回结果
	return &basic.Response{
		Code: 0,
		Msg:  "success",
	}, nil
}

// ConfigUpdate 更改单位配置
func (c *ConfigService) ConfigUpdate(ctx context.Context, req *core_api.ConfigCreateOrUpdateReq) (resp *basic.Response, err error) {
	// 参数校验
	unitOid, err := bson.ObjectIDFromHex(req.Config.UnitId)
	if err != nil {
		return nil, errorx.New(errno.ErrInvalidParams, errorx.KV("field", "单位ID"))
	}

	// 鉴权
	usrMeta, err := c.AuthDomain.ExtraUserMeta(ctx)
	if err != nil {
		return nil, err
	}
	if !usrMeta.HasUnitAdminAuth(req.Config.UnitId) {
		return nil, errorx.New(errno.ErrInsufficientAuth)
	}

	// 存在性验证
	oldConf, err := c.ConfigMapper.FindOneByUnitID(ctx, unitOid)
	if err != nil || oldConf == nil {
		// 若不存在，当成create处理
		return c.ConfigCreate(ctx, req)
	}

	// 若存在，执行更新逻辑
	// 提取req中的非空字段，构造bson
	update := extractUpdateBSON(req)

	err = c.ConfigMapper.UpdateFields(ctx, oldConf.ID, update)
	if err != nil {
		logs.Errorf("update config error: %s", errorx.ErrorWithoutStack(err))
		return nil, err
	}

	return &basic.Response{
		Code: 0,
		Msg:  "success",
	}, nil
}

func (c *ConfigService) ConfigGetByUnitID(ctx context.Context, req *core_api.ConfigGetByUnitIdReq) (resp *core_api.ConfigGetByUnitIdResp, err error) {
	// 参数校验和转化
	if req.UnitId == "" {
		return nil, errorx.New(errno.ErrMissingParams, errorx.KV("field", "单位ID"))
	}

	unitOid, err := bson.ObjectIDFromHex(req.UnitId)
	if err != nil {
		return nil, errorx.New(errno.ErrInvalidParams, errorx.KV("field", "单位ID"))
	}

	// 获得配置对象
	configDAO, err := c.ConfigMapper.FindOneByUnitID(ctx, unitOid)
	if err != nil {
		logs.Errorf("find config error: %s", errorx.ErrorWithoutStack(err))
		return nil, err
	}

	util.DPrint("configDAO: %+v\n", configDAO.Chat)
	return &core_api.ConfigGetByUnitIdResp{
		Config: configDB2VO(configDAO),
		Code:   0,
		Msg:    "success",
	}, nil
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
func configDB2VO(configDAO *config.Config) *core_api.ConfigVO {
	return &core_api.ConfigVO{
		UnitId:     configDAO.UnitID.Hex(),
		Type:       int32(configDAO.Type),
		Characters: characterDB2Resp(configDAO.Characters),
		Scene:      configDAO.Scene,
		AlertPhone: configDAO.AlertPhone,
		Chat: &core_api.ChatApp{
			Name:        configDAO.Chat.Name,
			Description: configDAO.Chat.Description,
			Provider:    configDAO.Chat.Provider,
			AppId:       configDAO.Chat.AppID,
		},

		Tts: &core_api.TTSApp{
			Name:        configDAO.TTS.Name,
			Description: configDAO.TTS.Description,
			Provider:    configDAO.TTS.Provider,
			AppId:       configDAO.TTS.AppID,
			Speaker:     configDAO.TTS.Speaker,
		},

		Report: &core_api.ReportApp{
			Name:        configDAO.Report.Name,
			Description: configDAO.Report.Description,
			Provider:    configDAO.Report.Provider,
			AppId:       configDAO.Report.AppID,
		},

		Status:     int32(configDAO.Status),
		CreateTime: configDAO.CreateTime.Unix(),
		UpdateTime: configDAO.UpdateTime.Unix(),
	}
}

// MaskConfig 隐藏Config的一些敏感字段
func MaskConfig(conf *core_api.ConfigVO) *core_api.ConfigVO {
	if conf.Chat != nil {
		conf.Chat.AppId = ""
	}
	if conf.Tts != nil {
		conf.Tts.AppId = ""
	}
	if conf.Report != nil {
		conf.Report.AppId = ""
	}
	return conf
}

func characterDB2Resp(in []*config.Character) []*core_api.Character {
	if in == nil {
		return nil
	}
	out := make([]*core_api.Character, len(in))
	for i, c := range in {
		out[i] = &core_api.Character{
			Id:     c.ID.Hex(),
			Name:   c.Name,
			Voice:  c.Voice,
			Image:  c.Image,
			Status: int32(c.Status),
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
			ID:     oid,
			Name:   c.Name,
			Voice:  c.Voice,
			Image:  c.Image,
			Status: int(c.Status),
		}
	}
	return out
}
