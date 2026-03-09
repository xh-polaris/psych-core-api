package conf

import (
	"github.com/xh-polaris/psych-core-api/biz/application/dto/core_api"
	"github.com/xh-polaris/psych-core-api/pkg/app"
	"github.com/xh-polaris/psych-core-api/pkg/errorx"
	"github.com/xh-polaris/psych-core-api/types/errno"
)

type Coze struct {
	BaseURL string
	PAT     string
}

type ChatConfig struct {
	URL       string
	AccessKey string
}

type TTSConfig struct {
	URL         string
	AccessKey   string
	Namespace   string
	ResourceId  string
	AudioParams struct {
		Format       string
		Codec        string
		Rate         int32
		Bits         int32
		Channels     int
		SpeechRate   int32
		LoudnessRate int32
		Lang         string
		ResultType   string
	}
}

type ASRConfig struct {
	Provider   string
	URL        string
	AppID      string
	AccessKey  string
	ResourceId string
	Format     string // 音频容器 (volc)pcm(pcm_s16le) / wav(pcm_s16le) / ogg
	Codec      string // 编码方式 (volc)raw / opus，默认为 raw(pcm)
	Rate       int    // 采样频率 (volc)默认为 16000，目前只支持16000
	Bits       int    // 比特率  (volc)默认为 16。
	Channels   int    // 声道个数 (volc)默认为 1
	ModelName  string // 模型名称 (volc)目前只有bigmodel
	EnablePunc bool   // 启用标点
	EnableDdc  bool   // 启用语义顺滑
	ResultType string // 返回方式,full为全量, single为增量
}

type ModelConfig struct {
	Chat map[string]*ChatConfig
	TTS  map[string]*TTSConfig
	ASR  *ASRConfig
}

// ChatConf 获取对话配置
func (c *Config) ChatConf(chat *core_api.ChatApp) (*app.ChatSetting, error) {
	if chat == nil {
		return nil, errorx.New(errno.ConfigErr, errorx.KV("app", "chat"))
	}
	if cc, ok := c.ModelConfig.Chat[chat.Provider]; ok {
		return &app.ChatSetting{Provider: chat.Provider, Url: cc.URL, Model: "",
			BotId: chat.AppId, UserId: "", AccessKey: cc.AccessKey}, nil
	}
	return nil, errorx.New(errno.ConfigErr, errorx.KV("app", "chat"))
}

// TTSConf 获取TTS配置
func (c *Config) TTSConf(tts *core_api.TTSApp) (*app.TTSSetting, error) {
	if tts == nil {
		return nil, errorx.New(errno.ConfigErr, errorx.KV("app", "tts"))
	}
	if ct, ok := c.ModelConfig.TTS[tts.Provider]; ok {
		ap := &app.AudioParams{Format: ct.AudioParams.Format, Codec: ct.AudioParams.Codec, Rate: ct.AudioParams.Rate,
			Bits: ct.AudioParams.Bits, Channels: ct.AudioParams.Channels, SpeechRate: ct.AudioParams.SpeechRate,
			LoudnessRate: ct.AudioParams.LoudnessRate, Lang: ct.AudioParams.Lang, ResultType: ct.AudioParams.ResultType}
		return &app.TTSSetting{Provider: tts.Provider, Url: ct.URL, AppID: tts.AppId, AccessKey: ct.AccessKey,
			Speaker: tts.Speaker, Namespace: ct.Namespace, ResourceId: ct.ResourceId, AudioParams: ap}, nil
	}
	return nil, errorx.New(errno.ConfigErr, errorx.KV("app", "tts"))
}

// ASRConf 获取ASR配置
func (c *Config) ASRConf() (*app.ASRSetting, error) {
	asr := c.ModelConfig.ASR
	return &app.ASRSetting{Provider: asr.Provider, Url: asr.URL, AppID: asr.AppID, AccessKey: asr.AccessKey,
		ResourceId: asr.ResourceId, Format: asr.Format, Codec: asr.Codec, Rate: asr.Rate, Bits: asr.Bits,
		Channels: asr.Channels, ModelName: asr.ModelName, EnablePunc: asr.EnablePunc, EnableDdc: asr.EnableDdc,
		ResultType: asr.ResultType}, nil
}

// ReportConf 获取报表配置
func (c *Config) ReportConf(report *core_api.ReportApp) (*app.ReportSetting, error) {
	if report == nil {
		return nil, errorx.New(errno.ConfigErr, errorx.KV("app", "report"))
	}
	return &app.ReportSetting{Provider: report.Provider, Url: "", AppId: report.AppId, AccessKey: ""}, nil
}
