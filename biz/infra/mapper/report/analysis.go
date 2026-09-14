package report

import "fmt"

// ScalarString normalizes report fields that can be emitted as either a JSON
// string or number by different report schema versions.
func ScalarString(value any) string {
	if value == nil {
		return ""
	}
	return fmt.Sprint(value)
}

// ======== Analysis — 13 维度状态提取 ========

type Analysis struct {
	Problem     AnalysisProblem    `bson:"problem" json:"problem"`
	Emotion     AnalysisEmotion    `bson:"emotion" json:"emotion"`
	Cognition   []string           `bson:"cognition,omitempty" json:"cognition,omitempty"`
	Behavior    []string           `bson:"behavior,omitempty" json:"behavior,omitempty"`
	Duration    string             `bson:"duration" json:"duration"`
	Trigger     []string           `bson:"trigger,omitempty" json:"trigger,omitempty"`
	Coping      []string           `bson:"coping,omitempty" json:"coping,omitempty"`
	Support     AnalysisSupport    `bson:"support" json:"support"`
	HelpSeeking string             `bson:"helpSeeking" json:"helpSeeking"`
	Function    AnalysisFunction   `bson:"function" json:"function"`
	Distress    AnalysisDistress   `bson:"distress" json:"distress"`
	Risk        AnalysisRisk       `bson:"risk" json:"risk"`
	Confidence  AnalysisConfidence `bson:"confidence" json:"confidence"`
	MissingInfo []string           `bson:"missingInfo,omitempty" json:"missingInfo,omitempty"`
}

type ProblemItem struct {
	Category    string `bson:"category" json:"category"`
	Subcategory string `bson:"subcategory" json:"subcategory"`
}

type AnalysisProblem struct {
	Primary   ProblemItem   `bson:"primary" json:"primary"`
	Secondary []ProblemItem `bson:"secondary,omitempty" json:"secondary,omitempty"`
}

type AnalysisEmotion struct {
	Types     []string `bson:"type" json:"type"`
	Intensity string   `bson:"intensity" json:"intensity"`
}

type AnalysisSupport struct {
	Family              bool     `bson:"family" json:"family"`
	Teacher             bool     `bson:"teacher" json:"teacher"`
	Friend              bool     `bson:"friend" json:"friend"`
	Other               []string `bson:"other,omitempty" json:"other,omitempty"`
	ProtectiveResources []string `bson:"protectiveResources,omitempty" json:"protectiveResources,omitempty"`
	Availability        string   `bson:"availability,omitempty" json:"availability,omitempty"`
}

type AnalysisFunction struct {
	Learning          string `bson:"learning" json:"learning"`
	Sleep             string `bson:"sleep" json:"sleep"`
	Diet              string `bson:"diet" json:"diet"`
	Interpersonal     string `bson:"interpersonal" json:"interpersonal"`
	DailyLife         string `bson:"dailyLife" json:"dailyLife"`
	EmotionRegulation string `bson:"emotionRegulation,omitempty" json:"emotionRegulation,omitempty"`
}

type AnalysisDistress struct {
	// Level historically was a string. Report v2 sends a numeric severity level,
	// so keep it polymorphic to remain able to decode both existing Mongo data
	// and newly generated reports.
	Level  any      `bson:"level" json:"level"`
	Reason []string `bson:"reason,omitempty" json:"reason,omitempty"`
}

type AnalysisRisk struct {
	Level    string      `bson:"level" json:"level"`
	Score    RiskScore   `bson:"score" json:"score"`
	Profile  RiskProfile `bson:"profile" json:"profile"`
	Evidence []string    `bson:"evidence,omitempty" json:"evidence,omitempty"`
	Action   string      `bson:"action" json:"action"`
}

type RiskScore struct {
	CurrentIdeation     int `bson:"currentIdeation" json:"currentIdeation"`
	History             int `bson:"history" json:"history"`
	CurrentStress       int `bson:"currentStress" json:"currentStress"`
	ProtectiveResources int `bson:"protectiveResources" json:"protectiveResources"`
	MentalHealthHistory int `bson:"mentalHealthHistory" json:"mentalHealthHistory"`
	Total               int `bson:"total" json:"total"`
}

type RiskProfile struct {
	CurrentRisk       ProfileSection `bson:"currentRisk" json:"currentRisk"`
	Stressors         ProfileSection `bson:"stressors" json:"stressors"`
	RiskFactors       ProfileSection `bson:"riskFactors" json:"riskFactors"`
	ProtectiveFactors ProfileSection `bson:"protectiveFactors" json:"protectiveFactors"`
	InformationGap    ProfileSection `bson:"informationGap" json:"informationGap"`
	CurrentSafety     struct {
		Summary string `bson:"summary" json:"summary"`
		Status  string `bson:"status" json:"status"`
	} `bson:"currentSafety" json:"currentSafety"`
}

type ProfileSection struct {
	Summary string   `bson:"summary" json:"summary"`
	Items   []string `bson:"items,omitempty" json:"items,omitempty"`
}

type AnalysisConfidence struct {
	Overall string `bson:"overall" json:"overall"`
	Risk    string `bson:"risk" json:"risk"`
	Reason  string `bson:"reason" json:"reason"`
}

// ======== SimpleReport — 16 板块简易报告 ========

type SimpleReport struct {
	// Report v2 compact fields. The legacy fields below remain for reports that
	// were generated before the compact report format was introduced.
	Keywords           []string       `bson:"keywords,omitempty" json:"keywords,omitempty"`
	RiskLevel          int            `bson:"riskLevel,omitempty" json:"riskLevel,omitempty"`
	SeverityLevel      int            `bson:"severityLevel,omitempty" json:"severityLevel,omitempty"`
	Focus              string         `bson:"focus,omitempty" json:"focus,omitempty"`
	Content            string         `bson:"content,omitempty" json:"content,omitempty"`
	MainProblem        string         `bson:"mainProblem" json:"mainProblem"`
	Emotion            ReportEmotion  `bson:"emotion" json:"emotion"`
	Thoughts           string         `bson:"thoughts" json:"thoughts"`
	Behaviors          []string       `bson:"behaviors,omitempty" json:"behaviors,omitempty"`
	Needs              []string       `bson:"needs,omitempty" json:"needs,omitempty"`
	Duration           string         `bson:"duration" json:"duration"`
	FunctionImpact     string         `bson:"functionImpact" json:"functionImpact"`
	Triggers           string         `bson:"triggers" json:"triggers"`
	Coping             string         `bson:"coping" json:"coping"`
	Support            string         `bson:"support" json:"support"`
	HelpSeeking        string         `bson:"helpSeeking" json:"helpSeeking"`
	RiskObservation    ReportRiskObs  `bson:"riskObservation" json:"riskObservation"`
	SeverityAssessment ReportSeverity `bson:"severityAssessment" json:"severityAssessment"`
	Summary            ReportSummary  `bson:"summary" json:"summary"`
	ProvidedSupport    string         `bson:"providedSupport" json:"providedSupport"`
	Suggestions        []string       `bson:"suggestions" json:"suggestions"`
}

type ReportEmotion struct {
	Type      string `bson:"type" json:"type"`
	Intensity string `bson:"intensity" json:"intensity"`
}

type ReportRiskObs struct {
	Level    string `bson:"level" json:"level"`
	Evidence string `bson:"evidence,omitempty" json:"evidence,omitempty"`
}

type ReportSeverity struct {
	Level string `bson:"level" json:"level"`
	Basis string `bson:"basis" json:"basis"`
}

type ReportSummary struct {
	MainProblem  string `bson:"mainProblem" json:"mainProblem"`
	EmotionState string `bson:"emotionState" json:"emotionState"`
	Severity     string `bson:"severity" json:"severity"`
	RiskLevel    string `bson:"riskLevel" json:"riskLevel"`
	Focus        string `bson:"focus" json:"focus"`
}
