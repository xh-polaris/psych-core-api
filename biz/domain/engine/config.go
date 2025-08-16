package engine

import (
	"github.com/xh-polaris/psych-core-api/biz/infra/consts"
	"github.com/xh-polaris/psych-core-api/biz/infra/rpc"
	"github.com/xh-polaris/psych-core-api/biz/infra/utils"
	"github.com/xh-polaris/psych-idl/kitex_gen/model"
	"github.com/xh-polaris/psych-pkg/core"
	"github.com/xh-polaris/psych-pkg/util/logx"
)

// config 配置app与workflow
func (e *Engine) config() {
	var err error
	var conf *core.Config
	var wfConf *core.WorkFlowConfig
	var configResp *model.UnitAppConfigGetByUnitIdResp
	pm := rpc.GetPsychModel()

	// 获取配置
	req := &model.UnitAppConfigGetByUnitIdReq{UnitId: "", Admin: false}
	if configResp, err = pm.UnitAppConfigGetByUnitId(e.ctx, req); err != nil {
		logx.Error("[engine] [%s] UnitAppConfigGetByUnitId err: %v", core.AConfig, err)
		e.MWrite(core.MErr, consts.Err(consts.GetConfigFailed))
	}
	// 构造配置
	if conf, wfConf, err = e.buildConfig(configResp); err != nil {
		logx.Error("[engine] [%s] buildConfig err: %v", core.AConfig, err)
		e.MWrite(core.MErr, consts.Err(consts.GetConfigFailed))
	}
	// 配置workflow
	if err = e.workflow.Orchestrate(wfConf); err != nil {
		logx.Error("[engine] [%s] workflow orchestrate err: %v", core.AConfig, err)
		e.MWrite(core.MErr, consts.Err(consts.GetConfigFailed))
	}
	// 返回前端
	e.MWrite(core.MConfig, conf)
	utils.DPrint("[engine] [config] workflow config: %+v\n conf: %+v\n", conf)
}

// 构造返回给前端的配置
func (e *Engine) buildConfig(resp *model.UnitAppConfigGetByUnitIdResp) (config *core.Config, wfConf *core.WorkFlowConfig, err error) {
	var (
		chatConf   core.ChatConfig
		reportConf core.ReportConfig
		asrConf    core.ASRConfig
		ttsConf    core.TTSConfig
	)
	config.ModelName, config.ModelView = resp.UnitAppConfig.Name, resp.UnitAppConfig.View
	apps := resp.Apps
	for _, app := range apps {
		switch app.Type {
		case consts.ChatApp:
			chatApp := app.GetChatApp()
			chatConf = core.ChatConfig{Id: chatApp.App.Id}
		case consts.TtsApp:
			ttsApp := app.GetTtsApp()
			ttsConf = core.TTSConfig{
				Id:           ttsApp.App.Id,
				Format:       ttsApp.AudioParams.Format,
				Codec:        ttsApp.AudioParams.Codec,
				Rate:         int(ttsApp.AudioParams.Rate),
				Bits:         int(ttsApp.AudioParams.Bits),
				Channels:     int(ttsApp.AudioParams.Channels),
				ResultType:   ttsApp.AudioParams.ResultType,
				SpeechRate:   float32(ttsApp.AudioParams.SpeechRate),
				LoudnessRate: float32(ttsApp.AudioParams.LoudnessRate),
				Lang:         ttsApp.AudioParams.Lang,
			}
		case consts.AsrApp:
			asrApp := app.GetAsrApp()
			asrConf = core.ASRConfig{
				Id:         asrApp.App.Id,
				Format:     asrApp.Format,
				Codec:      asrApp.Codec,
				Rate:       int(asrApp.Rate),
				Bits:       int(asrApp.Bits),
				Channels:   int(asrApp.Channels),
				ResultType: asrApp.ResultType,
			}
		case consts.ReportApp:
			reportApp := app.GetReportApp()
			reportConf = core.ReportConfig{
				Id: reportApp.App.Id,
			}
		}
	}
	config = &core.Config{
		Id:           resp.UnitAppConfig.Id,
		ModelName:    resp.UnitAppConfig.Name,
		ModelView:    resp.UnitAppConfig.View,
		ChatConfig:   chatConf,
		ASRConfig:    asrConf,
		TTSConfig:    ttsConf,
		ReportConfig: reportConf,
	}
	return
}
