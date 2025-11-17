package core

import (
	"github.com/xh-polaris/psych-core-api/pkg/app"
)

type WorkFlowConfig struct {
	ChatConfig   *app.ChatSetting
	ReportConfig *app.ReportSetting
	ASRConfig    *app.ASRSetting
	TTSConfig    *app.TTSSetting
}
