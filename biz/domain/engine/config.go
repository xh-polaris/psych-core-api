package engine

import (
	"github.com/xh-polaris/psych-core-api/biz/infra/consts"
	"github.com/xh-polaris/psych-core-api/biz/infra/rpc"
	"github.com/xh-polaris/psych-core-api/biz/infra/utils"
	"github.com/xh-polaris/psych-idl/kitex_gen/model"
	"github.com/xh-polaris/psych-pkg/app"
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
	req := &model.UnitAppConfigGetByUnitIdReq{UnitId: e.info[consts.UnitId].(string), Admin: true}
	if configResp, err = pm.UnitAppConfigGetByUnitId(e.ctx, req); err != nil {
		logx.Error("[engine] [%s] UnitAppConfigGetByUnitId err: %v", core.AConfig, err)
		e.MWrite(core.MErr, consts.Err(consts.GetConfigFailed))
		return
	}
	// 构造配置
	if conf, wfConf, err = e.buildConfig(configResp); err != nil {
		logx.Error("[engine] [%s] buildConfig err: %v", core.AConfig, err)
		e.MWrite(core.MErr, consts.Err(consts.GetConfigFailed))
		return
	}
	// 配置workflow
	if err = e.workflow.Orchestrate(wfConf); err != nil {
		logx.Error("[engine] [%s] workflow orchestrate err: %v", core.AConfig, err)
		e.MWrite(core.MErr, consts.Err(consts.GetConfigFailed))
		return
	}
	// 返回前端
	e.MWrite(core.MConfig, conf)
	utils.DPrint("[engine] [config] workflow config: %+v\n conf: %+v\n", wfConf, conf)
}

// 构造配置
func (e *Engine) buildConfig(resp *model.UnitAppConfigGetByUnitIdResp) (*core.Config, *core.WorkFlowConfig, error) {
	var err error
	return buildClientConfig(resp), buildAppSetting(resp), err
}

// 构造返回给客户端的配置
func buildClientConfig(resp *model.UnitAppConfigGetByUnitIdResp) *core.Config {
	config := &core.Config{
		Id:        resp.UnitAppConfig.Id,
		ModelName: resp.UnitAppConfig.Name,
		ModelView: resp.UnitAppConfig.View,
	}
	for _, one := range resp.Apps {
		switch one.Type {
		case consts.ChatApp:
			chatApp := one.GetChatApp()
			config.ChatConfig = core.ChatConfig{Id: chatApp.App.Id}
		case consts.TtsApp:
			ttsApp := one.GetTtsApp()
			config.TTSConfig = core.TTSConfig{
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
			asrApp := one.GetAsrApp()
			config.ASRConfig = core.ASRConfig{
				Id:         asrApp.App.Id,
				Format:     asrApp.Format,
				Codec:      asrApp.Codec,
				Rate:       int(asrApp.Rate),
				Bits:       int(asrApp.Bits),
				Channels:   int(asrApp.Channels),
				ResultType: asrApp.ResultType,
			}
		case consts.ReportApp:
			reportApp := one.GetReportApp()
			config.ReportConfig = core.ReportConfig{
				Id: reportApp.App.Id,
			}
		}
	}
	return config
}

// 构造模型app的Setting
func buildAppSetting(resp *model.UnitAppConfigGetByUnitIdResp) *core.WorkFlowConfig {
	var wfConfig core.WorkFlowConfig
	for _, one := range resp.Apps {
		switch one.Type {
		case consts.ChatApp:
			chatApp := one.GetChatApp()
			wfConfig.ChatConfig = &app.ChatSetting{
				Id:        chatApp.App.Id,
				Provider:  chatApp.App.Provider,
				Url:       chatApp.App.Url,
				AppId:     chatApp.App.AppId,
				AccessKey: chatApp.App.AccessKey,
			}
		case consts.TtsApp:
			ttsApp := one.GetTtsApp()
			wfConfig.TTSConfig = &app.TTSSetting{
				Id:         ttsApp.App.Id,
				Provider:   ttsApp.App.Provider,
				Url:        ttsApp.App.Url,
				AppID:      ttsApp.App.AppId,
				AccessKey:  ttsApp.App.AccessKey,
				Namespace:  ttsApp.Namespace,
				Speaker:    ttsApp.Speaker,
				ResourceId: ttsApp.ResourceId,
				AudioParams: struct {
					Format       string `json:"format"`
					Codec        string `json:"codec"`
					Rate         int32  `json:"rate"`
					Bits         int32  `json:"bits"`
					Channels     int    `json:"channels"`
					SpeechRate   int32  `json:"speech_rate"`
					LoudnessRate int32  `json:"loudness_rate"`
					Lang         string `json:"lang"`
					ResultType   string `json:"result_type"`
				}{
					Format:       ttsApp.AudioParams.Format,
					Codec:        ttsApp.AudioParams.Codec,
					Rate:         ttsApp.AudioParams.Rate,
					Bits:         ttsApp.AudioParams.Bits,
					Channels:     int(ttsApp.AudioParams.Channels),
					SpeechRate:   ttsApp.AudioParams.SpeechRate,
					LoudnessRate: ttsApp.AudioParams.LoudnessRate,
					Lang:         ttsApp.AudioParams.Lang,
					ResultType:   ttsApp.AudioParams.ResultType,
				},
			}
		case consts.AsrApp:
			asrApp := one.GetAsrApp()
			wfConfig.ASRConfig = &app.ASRSetting{
				Id:         asrApp.App.Id,
				Provider:   asrApp.App.Provider,
				Url:        asrApp.App.Url,
				AppID:      asrApp.App.AppId,
				AccessKey:  asrApp.App.AccessKey,
				ResourceId: asrApp.ResourceId,
				Format:     asrApp.Format,
				Codec:      asrApp.Codec,
				Rate:       int(asrApp.Rate),
				Bits:       int(asrApp.Bits),
				Channels:   int(asrApp.Channels),
				ModelName:  asrApp.ModelName,
				EnablePunc: asrApp.EnablePunc,
				EnableDdc:  asrApp.EnableDdc,
				ResultType: asrApp.ResultType,
			}
		case consts.ReportApp:
			reportApp := one.GetReportApp()
			wfConfig.ReportConfig = &app.ReportSetting{
				Id:        reportApp.App.Id,
				Provider:  reportApp.App.Provider,
				Url:       reportApp.App.Url,
				AppId:     reportApp.App.AppId,
				AccessKey: reportApp.App.AccessKey,
			}
		}
	}
	return &wfConfig
}
