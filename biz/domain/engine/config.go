package engine

import (
	"time"

	"github.com/xh-polaris/psych-core-api/biz/application/dto/core_api"
	"github.com/xh-polaris/psych-core-api/biz/conf"
	"github.com/xh-polaris/psych-core-api/biz/cst"
	"github.com/xh-polaris/psych-core-api/biz/infra/util"
	"github.com/xh-polaris/psych-core-api/pkg/app"
	_ "github.com/xh-polaris/psych-core-api/pkg/app/volc/asr"
	_ "github.com/xh-polaris/psych-core-api/pkg/app/volc/tts"
	"github.com/xh-polaris/psych-core-api/pkg/core"
	"github.com/xh-polaris/psych-core-api/pkg/errorx"
	"github.com/xh-polaris/psych-core-api/pkg/logs"
	"github.com/xh-polaris/psych-core-api/types/enum"
	"github.com/xh-polaris/psych-core-api/types/errno"
	"go.mongodb.org/mongo-driver/v2/bson"
)

// config 配置app与workflow
func (e *Engine) config() error {
	var (
		err error
		cf  *core.Config
		wfc *core.WorkFlowConfig
	)

	vo, err := e.cfgSvc.ConfigGetByUnitID4Engine(e.ctx, e.info[cst.JsonUnitID].(string))
	if err != nil {
		logs.Errorf("[engine] [%s] UnitAppConfigGetByUnitId err: %v", core.AConfig, err)
		return e.MWrite(core.MErr, core.ToErr(errorx.WrapByCode(err, errno.GetConfigErr)))
	}

	if cf, wfc, err = e.buildConfig(vo); err != nil {
		logs.Error("[workflow] [config] build config err: %v", err)
		return errorx.WrapByCode(err, errno.AppConfigErr, errorx.KV("app", "llm"))
	}
	// character_id 仅在会话创建时确定（CreateConversation）
	// e.Character 以「会话已绑定 character」为准，前端本次携带的 characterId 仅在新建会话时生效
	e.loadConvMeta()
	e.Character = pickCharacterByID(vo.Characters, e.characterID)
	logs.Infof("[engine] [config] chat=%s/%s tts=%s/%s asr=%s type=%d",
		wfc.ChatConfig.Provider, wfc.ChatConfig.BotId,
		wfc.TTSConfig.Provider, wfc.TTSConfig.Speaker,
		wfc.ASRConfig.Provider, cf.Type)

	if err = e.buildDialogueApp(wfc.ChatConfig); err != nil {
		logs.Error("[workflow] [config] new dialogueApp err: %v", err)
		return errorx.WrapByCode(err, errno.AppConfigErr, errorx.KV("app", "llm"))
	}
	if err = e.buildStrategyApp(wfc.ChatConfig); err != nil {
		logs.Error("[workflow] [config] new strategy agent err: %v", err)
		return errorx.WrapByCode(err, errno.AppConfigErr, errorx.KV("app", "strategy"))
	}
	if e.asr, err = app.NewASRApp(e.uSession, wfc.ASRConfig); err != nil {
		logs.Error("[workflow] [config] new asrApp err: %v", err)
		return errorx.WrapByCode(err, errno.AppConfigErr, errorx.KV("app", "asr"))
	}
	if e.tts, err = app.NewTTSApp(e.uSession, wfc.TTSConfig); err != nil {
		logs.Error("[workflow] [config] new asrApp err: %v", err)
		return errorx.WrapByCode(err, errno.AppConfigErr, errorx.KV("app", "tts"))
	}
	return e.MWrite(core.MConfig, cf)
}

// 构造配置
func (e *Engine) buildConfig(vo *core_api.ConfigVO) (c *core.Config, wfc *core.WorkFlowConfig, err error) {
	wfc = &core.WorkFlowConfig{}
	if wfc.ChatConfig, err = conf.GetConfig().ChatConf(vo.Chat); err != nil {
		return
	}
	wfc.ChatConfig.UserId = e.info[cst.JsonUserID].(string)
	if wfc.TTSConfig, err = conf.GetConfig().TTSConf(vo.Tts); err != nil {
		return
	}
	if wfc.ReportConfig, err = conf.GetConfig().ReportConf(vo.Report); err != nil {
		return
	}
	if wfc.ASRConfig, err = conf.GetConfig().ASRConf(); err != nil {
		return
	}
	c = &core.Config{Type: int(vo.Type), ModelName: "", ModelView: "", ChatConfig: core.ChatConfig{},
		ASRConfig: core.ASRConfig{Format: wfc.ASRConfig.Format, Codec: wfc.ASRConfig.Codec, Rate: wfc.ASRConfig.Rate,
			Bits: wfc.ASRConfig.Bits, Channels: wfc.ASRConfig.Channels, ResultType: wfc.ASRConfig.ResultType},
		TTSConfig: core.TTSConfig{Format: wfc.TTSConfig.AudioParams.Format, Codec: wfc.TTSConfig.AudioParams.Codec,
			Rate: int(wfc.TTSConfig.AudioParams.Rate), Bits: int(wfc.TTSConfig.AudioParams.Bits),
			Channels: wfc.TTSConfig.AudioParams.Channels, ResultType: wfc.TTSConfig.AudioParams.ResultType,
			SpeechRate: float32(wfc.TTSConfig.AudioParams.SpeechRate), LoudnessRate: float32(wfc.TTSConfig.AudioParams.LoudnessRate),
			Lang: wfc.TTSConfig.AudioParams.Lang},
		ReportConfig: core.ReportConfig{},
	}
	return
}

// loadConvMeta 回查当前会话绑定的 character_id 与 chat_date，
// 供当日同角色上下文聚合（GetDailyMessages）使用。
func (e *Engine) loadConvMeta() {
	if convOID, err := bson.ObjectIDFromHex(e.uSession); err == nil {
		if conv, err := e.convMapper.FindOneById(e.ctx, convOID); err == nil && conv != nil {
			if !conv.CharacterID.IsZero() {
				e.characterID = conv.CharacterID.Hex()
			}
			e.chatDate = conv.ChatDate
		}
	}
	if e.characterID == "" {
		e.characterID, _ = e.info[cst.JsonCharacterID].(string)
	}
	if e.chatDate == "" {
		e.chatDate = util.FormatDateUTC8(time.Now())
	}
}

// pickCharacterByID 确定本次会话绑定的老师形象。
// conversationId 在 auth() 中已强制要求必填，characterID 由 loadConvMeta 从会话 DB 回查，
// 前端本次携带的 characterId 仅在会话 character_id 缺失时兜底；为空时返回首个启用角色
// （与历史 pickCharacter 语义保持一致）。
func pickCharacterByID(characters []*core_api.Character, characterID string) *core.CharacterInfo {
	if len(characters) == 0 {
		return nil
	}
	for _, ch := range characters {
		if int(ch.Status) != enum.ConfigStatusActive {
			continue
		}
		if characterID != "" && ch.Id != characterID {
			continue
		}
		return &core.CharacterInfo{Id: ch.Id, Name: ch.Name, Voice: ch.Voice, Image: ch.Image}
	}
	// 指定 ID 未匹配到启用角色时，允许回退到该 ID 本身（可能已停用但会话历史仍引用）
	if characterID != "" {
		for _, ch := range characters {
			if ch.Id == characterID {
				return &core.CharacterInfo{Id: ch.Id, Name: ch.Name, Voice: ch.Voice, Image: ch.Image}
			}
		}
	}
	return nil
}
