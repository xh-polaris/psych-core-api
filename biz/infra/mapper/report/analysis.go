package report

import (
	"fmt"
	"strconv"
	"strings"

	"go.mongodb.org/mongo-driver/v2/bson"
)

// ======== Analysis — 13 维度状态提取 ========

type Analysis struct {
	Problem     AnalysisProblem    `bson:"problem" json:"problem"`
	Emotion     AnalysisEmotions   `bson:"emotion" json:"emotion"` // 情绪列表（1-3 项，按影响程度排序）
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
	Type      string  `bson:"type" json:"type"`
	Intensity float64 `bson:"intensity" json:"intensity"` // 0.5~2.0，中度基准 1.0
}

// AnalysisEmotions 兼容历史报告的两种存储格式：
// 新版为数组，旧版曾将单项情绪直接存为对象。对外统一表现为数组
type AnalysisEmotions []AnalysisEmotion

func (e *AnalysisEmotions) UnmarshalBSONValue(typ byte, data []byte) error {
	raw := bson.RawValue{Type: bson.Type(typ), Value: data}
	switch raw.Type {
	case bson.TypeNull:
		*e = nil
		return nil
	case bson.TypeArray:
		var values []AnalysisEmotion
		if err := raw.Unmarshal(&values); err != nil {
			return err
		}
		*e = values
		return nil
	case bson.TypeEmbeddedDocument:
		return e.unmarshalLegacyDocument(raw)
	default:
		return fmt.Errorf("unsupported analysis.emotion BSON type %s", raw.Type)
	}
}

// unmarshalLegacyDocument 兼容旧版对象格式。其中 type / intensity 既可能是单值，
// 也可能分别是等长数组；后者按下标还原成多条情绪记录。
func (e *AnalysisEmotions) unmarshalLegacyDocument(raw bson.RawValue) error {
	var doc bson.M
	if err := raw.Unmarshal(&doc); err != nil {
		return err
	}

	types := legacyEmotionStrings(doc["type"])
	if len(types) == 0 {
		return fmt.Errorf("legacy analysis.emotion.type is empty or invalid")
	}
	intensities := legacyEmotionIntensities(doc["intensity"])

	values := make(AnalysisEmotions, 0, len(types))
	for i, emotionType := range types {
		intensity := 1.0 // 历史数据未记录或无法解析强度时，以中等强度兜底展示
		if len(intensities) == 1 {
			intensity = intensities[0]
		} else if i < len(intensities) {
			intensity = intensities[i]
		}
		values = append(values, AnalysisEmotion{Type: emotionType, Intensity: intensity})
	}
	*e = values
	return nil
}

func legacyEmotionStrings(value any) []string {
	switch v := value.(type) {
	case string:
		if v == "" {
			return nil
		}
		return []string{v}
	case bson.A:
		values := make([]string, 0, len(v))
		for _, item := range v {
			if text, ok := item.(string); ok && text != "" {
				values = append(values, text)
			}
		}
		return values
	default:
		return nil
	}
}

func legacyEmotionIntensities(value any) []float64 {
	if values, ok := value.(bson.A); ok {
		result := make([]float64, 0, len(values))
		for _, item := range values {
			if intensity, ok := legacyEmotionIntensity(item); ok {
				result = append(result, intensity)
			}
		}
		return result
	}
	if intensity, ok := legacyEmotionIntensity(value); ok {
		return []float64{intensity}
	}
	return nil
}

func legacyEmotionIntensity(value any) (float64, bool) {
	switch v := value.(type) {
	case float64:
		return v, true
	case float32:
		return float64(v), true
	case int:
		return float64(v), true
	case int32:
		return float64(v), true
	case int64:
		return float64(v), true
	case string:
		parsed, err := strconv.ParseFloat(v, 64)
		return parsed, err == nil
	default:
		return 0, false
	}
}

type AnalysisSupport struct {
	Family              bool     `bson:"family" json:"family"`
	Teacher             bool     `bson:"teacher" json:"teacher"`
	Friend              bool     `bson:"friend" json:"friend"`
	Other               []string `bson:"other,omitempty" json:"other,omitempty"`
	ProtectiveResources []string `bson:"protectiveResources,omitempty" json:"protectiveResources,omitempty"`
	Availability        string   `bson:"availability,omitempty" json:"availability,omitempty"` // 充足 | 一般 | 不足 | 未明确提及
}

type AnalysisFunction struct {
	Learning          string `bson:"learning" json:"learning"`
	Sleep             string `bson:"sleep" json:"sleep"`
	Diet              string `bson:"diet" json:"diet"`
	Interpersonal     string `bson:"interpersonal" json:"interpersonal"`
	EmotionRegulation string `bson:"emotionRegulation" json:"emotionRegulation"`
}

// DistressLevelNum 兼容新旧两版存储的困扰程度：
// 新版为 0-4 数值；旧版为中文字符串（如"2级：中度困扰"），读取时自动归一化为数值。
type DistressLevelNum int32

func (l *DistressLevelNum) UnmarshalBSONValue(typ byte, data []byte) error {
	rv := bson.RawValue{Type: bson.Type(typ), Value: data}
	if rv.Type == bson.TypeString {
		*l = DistressLevelNum(legacyDistressLevel(rv.StringValue()))
		return nil
	}
	var n int32
	if err := rv.Unmarshal(&n); err != nil {
		return err
	}
	*l = DistressLevelNum(n)
	return nil
}

func legacyDistressLevel(s string) int32 {
	switch {
	case strings.Contains(s, "高危"):
		return 4
	case strings.Contains(s, "重度"):
		return 3
	case strings.Contains(s, "中度"):
		return 2
	case strings.Contains(s, "轻度"):
		return 1
	default:
		return 0
	}
}

type AnalysisDistress struct {
	Level  DistressLevelNum `bson:"level" json:"level"` // 0正常波动 | 1轻度 | 2中度 | 3重度 | 4高危
	Reason []string         `bson:"reason,omitempty" json:"reason,omitempty"`
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

// ======== SimpleReport — 瘦身简易报告（v2 提示词产出） ========
// 展示细节由 analysis 与 content 承担，此处仅保留统计与展示必需字段。

type SimpleReport struct {
	Keywords      []string `bson:"keywords,omitempty" json:"keywords,omitempty"` // 2-3 个，源自学生讲到的话题
	Emotion       []string `bson:"emotion" json:"emotion"`                       // 情绪类型列表（1-3 项，与 analysis.emotion 顺序一致）
	RiskLevel     int32    `bson:"riskLevel" json:"riskLevel"`                   // 0未明确 | 1高风险 | 2中高风险 | 3中低风险 | 4低风险
	DistressLevel int32    `bson:"distressLevel" json:"distressLevel"`           // 0正常波动 | 1轻度 | 2中度 | 3重度 | 4高危
	Focus         string   `bson:"focus" json:"focus"`                           // 需要重点关注的问题
	Suggestions   []string `bson:"suggestions" json:"suggestions"`               // 面向老师的 3 条建议
	Content       string   `bson:"content" json:"content"`                       // 报告正文（500-1500字，十六节固定格式）
}
