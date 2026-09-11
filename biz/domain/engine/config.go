package engine

import (
	"github.com/xh-polaris/psych-core-api/biz/application/dto/core_api"
	"github.com/xh-polaris/psych-core-api/biz/conf"
	"github.com/xh-polaris/psych-core-api/biz/cst"
	"github.com/xh-polaris/psych-core-api/pkg/app"
	_ "github.com/xh-polaris/psych-core-api/pkg/app/volc/asr"
	_ "github.com/xh-polaris/psych-core-api/pkg/app/volc/tts"
	"github.com/xh-polaris/psych-core-api/pkg/core"
	"github.com/xh-polaris/psych-core-api/pkg/errorx"
	"github.com/xh-polaris/psych-core-api/pkg/logs"
	"github.com/xh-polaris/psych-core-api/types/enum"
	"github.com/xh-polaris/psych-core-api/types/errno"
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
	e.Character = e.pickCharacter(vo.Characters)
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

func (e *Engine) pickCharacter(characters []*core_api.Character) *core.CharacterInfo {
	if len(characters) == 0 {
		return nil
	}
	characterId, _ := e.info[cst.JsonCharacterID].(string)
	for _, ch := range characters {
		if int(ch.Status) != enum.ConfigStatusActive {
			continue
		}
		if characterId != "" && ch.Id != characterId {
			continue
		}
		return &core.CharacterInfo{Id: ch.Id, Name: ch.Name, Voice: ch.Voice, Image: ch.Image}
	}
	if characterId == "" {
		return nil
	}
	for _, ch := range characters {
		if ch.Id == characterId {
			return &core.CharacterInfo{Id: ch.Id, Name: ch.Name, Voice: ch.Voice, Image: ch.Image}
		}
	}
	return nil
}
