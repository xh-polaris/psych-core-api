package dashboard

import "github.com/xh-polaris/psych-core-api/biz/infra/mapper/report"

// GetReportResponse is the JSON response of /dashboard/get_report.
// Analysis and SimpleReport deliberately use the persisted report model so
// newly added report fields can be returned without being dropped by an older
// generated protobuf DTO.
type GetReportResponse struct {
	Title          string               `json:"title"`
	Topics         []string             `json:"topics"`
	Digest         string               `json:"digest"`
	Emotion        int32                `json:"emotion"`
	Body           string               `json:"body"`
	NeedAlarm      bool                 `json:"needAlarm"`
	ReportID       string               `json:"reportId"`
	Suggestions    []string             `json:"suggestions"`
	KeywordPercent map[string]float64   `json:"keywordPercent"`
	ReportStatus   int32                `json:"reportStatus"`
	CharacterID    string               `json:"characterId"`
	CharacterName  string               `json:"characterName"`
	CharacterVoice string               `json:"characterVoice"`
	CharacterImage string               `json:"characterImage"`
	Analysis       *report.Analysis     `json:"analysis,omitempty"`
	SimpleReport   *report.SimpleReport `json:"simpleReport,omitempty"`
	Code           int32                `json:"code"`
	Msg            string               `json:"msg"`
}
