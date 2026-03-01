package engine

import (
	"github.com/xh-polaris/psych-core-api/biz/conf"
	"github.com/xh-polaris/psych-core-api/biz/cst"
	"github.com/xh-polaris/psych-core-api/pkg/app"
	_ "github.com/xh-polaris/psych-core-api/pkg/app/volc/asr"
	_ "github.com/xh-polaris/psych-core-api/pkg/app/volc/tts"
	"github.com/xh-polaris/psych-core-api/pkg/core"
	"github.com/xh-polaris/psych-core-api/pkg/logs"
	"github.com/xh-polaris/psych-idl/kitex_gen/profile"

	"github.com/xh-polaris/psych-core-api/biz/infra/rpc"
	"github.com/xh-polaris/psych-core-api/biz/infra/util"
	"github.com/xh-polaris/psych-core-api/pkg/errorx"
	"github.com/xh-polaris/psych-core-api/types/errno"
)

// config 配置app与workflow
func (e *Engine) config() error {
	var err error
	var cf *core.Config
	var wfc *core.WorkFlowConfig
	var configResp *profile.ConfigGetByUnitIdResp
	pm := rpc.GetPsychProfile()

	// 获取配置
	req := &profile.ConfigGetByUnitIdReq{UnitId: e.info[cst.UnitId].(string), Admin: true}
	if configResp, err = pm.ConfigGetByUnitID(e.ctx, req); err != nil {
		logs.Error("[engine] [%s] UnitAppConfigGetByUnitId err: %v", core.AConfig, err)
		return e.MWrite(core.MErr, core.ToErr(errorx.WrapByCode(err, errno.GetConfigErr)))
	}
	// 构造配置
	if cf, wfc, err = e.buildConfig(configResp); err != nil {
		logs.Error("[workflow] [config] build config err: %v", err)
		return errorx.WrapByCode(err, errno.AppConfigErr, errorx.KV("app", "llm"))
	}

	// 构造llm
	if e.llm, err = app.NewChatApp(e.ctx, e.uSession, wfc.ChatConfig); err != nil {
		logs.Error("[workflow] [config] new chatApp err: %v", err)
		return errorx.WrapByCode(err, errno.AppConfigErr, errorx.KV("app", "llm"))
	}
	util.DPrint("llm: %+v", e.llm)
	// 构造asr
	if e.asr, err = app.NewASRApp(e.uSession, wfc.ASRConfig); err != nil {
		logs.Error("[workflow] [config] new asrApp err: %v", err)
		return errorx.WrapByCode(err, errno.AppConfigErr, errorx.KV("app", "asr"))
	}
	// 构造tts
	if e.tts, err = app.NewTTSApp(e.uSession, wfc.TTSConfig); err != nil {
		logs.Error("[workflow] [config] new asrApp err: %v", err)
		return errorx.WrapByCode(err, errno.AppConfigErr, errorx.KV("app", "tts"))
	}
	util.DPrint("tts: %+v", e.tts)
	// 返回前端
	util.DPrint("[engine] [config] workflow config: %+v\n conf: %+v\n", wfc, cf)
	return e.MWrite(core.MConfig, cf)
}

// 构造配置
func (e *Engine) buildConfig(resp *profile.ConfigGetByUnitIdResp) (c *core.Config, wfc *core.WorkFlowConfig, err error) {
	wfc = &core.WorkFlowConfig{}
	if wfc.ChatConfig, err = conf.GetConfig().ChatConf(resp.Config.Chat); err != nil {
		return
	}
	wfc.ChatConfig.UserId = e.info[cst.UserId].(string)
	if wfc.TTSConfig, err = conf.GetConfig().TTSConf(resp.Config.Tts); err != nil {
		return
	}
	if wfc.ReportConfig, err = conf.GetConfig().ReportConf(resp.Config.Report); err != nil {
		return
	}
	if wfc.ASRConfig, err = conf.GetConfig().ASRConf(); err != nil {
		return
	}
	c = &core.Config{Type: resp.Config.Type, ModelName: "", ModelView: "", ChatConfig: core.ChatConfig{},
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
