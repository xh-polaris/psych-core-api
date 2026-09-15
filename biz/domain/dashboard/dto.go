package dashboard

import (
	"sort"
	"strings"

	"github.com/xh-polaris/psych-core-api/biz/application/dto/core_api"
	"github.com/xh-polaris/psych-core-api/biz/infra/mapper/report"
	"github.com/xh-polaris/psych-core-api/biz/infra/mapper/user"
	"github.com/xh-polaris/psych-core-api/biz/infra/util"
	"github.com/xh-polaris/psych-core-api/types/enum"
)

// analysisToPB converts report.Analysis to core_api.ReportAnalysis
func analysisToPB(a *report.Analysis) *core_api.ReportAnalysis {
	if a == nil {
		a = &report.Analysis{}
	}
	r := &core_api.ReportAnalysis{
		Cognition:   nonNilSlice(a.Cognition),
		Behavior:    nonNilSlice(a.Behavior),
		Duration:    a.Duration,
		Trigger:     nonNilSlice(a.Trigger),
		Coping:      nonNilSlice(a.Coping),
		HelpSeeking: a.HelpSeeking,
		MissingInfo: nonNilSlice(a.MissingInfo),
		Problem:     &core_api.AnalysisProblem{Primary: &core_api.ProblemItem{}, Secondary: make([]*core_api.ProblemItem, 0)},
		Emotion:     emotionItemsToPB(a.Emotion),
		Support: &core_api.AnalysisSupport{
			Family: a.Support.Family, Teacher: a.Support.Teacher, Friend: a.Support.Friend,
			Other: nonNilSlice(a.Support.Other), ProtectiveResources: nonNilSlice(a.Support.ProtectiveResources),
			Availability: a.Support.Availability,
		},
		Function: &core_api.AnalysisFunction{
			Learning: a.Function.Learning, Sleep: a.Function.Sleep, Diet: a.Function.Diet,
			Interpersonal: a.Function.Interpersonal, EmotionRegulation: a.Function.EmotionRegulation,
		},
		Distress:   &core_api.AnalysisDistress{Level: int32(a.Distress.Level), Reason: nonNilSlice(a.Distress.Reason)},
		Confidence: &core_api.AnalysisConfidence{Overall: a.Confidence.Overall, Risk: a.Confidence.Risk, Reason: a.Confidence.Reason},
	}
	if a.Problem.Primary.Category != "" || a.Problem.Primary.Subcategory != "" {
		r.Problem.Primary = &core_api.ProblemItem{Category: a.Problem.Primary.Category, Subcategory: a.Problem.Primary.Subcategory}
	}
	if len(a.Problem.Secondary) > 0 {
		r.Problem.Secondary = make([]*core_api.ProblemItem, len(a.Problem.Secondary))
		for i, s := range a.Problem.Secondary {
			r.Problem.Secondary[i] = &core_api.ProblemItem{Category: s.Category, Subcategory: s.Subcategory}
		}
	}

	risk := &core_api.AnalysisRisk{Level: a.Risk.Level, Evidence: nonNilSlice(a.Risk.Evidence), Action: a.Risk.Action}
	risk.Score = &core_api.RiskScore{
		CurrentIdeation: int32(a.Risk.Score.CurrentIdeation), History: int32(a.Risk.Score.History),
		CurrentStress: int32(a.Risk.Score.CurrentStress), ProtectiveResources: int32(a.Risk.Score.ProtectiveResources),
		MentalHealthHistory: int32(a.Risk.Score.MentalHealthHistory), Total: int32(a.Risk.Score.Total),
	}
	risk.Profile = &core_api.RiskProfile{
		CurrentRisk:       secToPB(a.Risk.Profile.CurrentRisk),
		Stressors:         secToPB(a.Risk.Profile.Stressors),
		RiskFactors:       secToPB(a.Risk.Profile.RiskFactors),
		ProtectiveFactors: secToPB(a.Risk.Profile.ProtectiveFactors),
		InformationGap:    secToPB(a.Risk.Profile.InformationGap),
		CurrentSafety:     &core_api.RiskProfile_CurrentSafety{Summary: a.Risk.Profile.CurrentSafety.Summary, Status: a.Risk.Profile.CurrentSafety.Status},
	}
	r.Risk = risk
	return r
}

func secToPB(s report.ProfileSection) *core_api.ProfileSection {
	return &core_api.ProfileSection{Summary: s.Summary, Items: nonNilSlice(s.Items)}
}

func nonNilSlice[T any](items []T) []T {
	if items == nil {
		return []T{}
	}
	return items
}

// simpleReportToPB converts report.SimpleReport to core_api.SimpleReportMsg（v2 瘦身结构直映射）
func simpleReportToPB(sr *report.SimpleReport) *core_api.SimpleReportMsg {
	if sr == nil {
		return nil
	}
	return &core_api.SimpleReportMsg{
		Keywords:      nonNilSlice(sr.Keywords),
		Emotion:       nonNilSlice(sr.Emotion),
		RiskLevel:     sr.RiskLevel,
		DistressLevel: sr.DistressLevel,
		Focus:         sr.Focus,
		Suggestions:   nonNilSlice(sr.Suggestions),
		Content:       sr.Content,
	}
}

// emotionItemsToPB converts report analysis emotion items to PB
func emotionItemsToPB(items report.AnalysisEmotions) []*core_api.AnalysisEmotion {
	if len(items) == 0 {
		return make([]*core_api.AnalysisEmotion, 0)
	}
	res := make([]*core_api.AnalysisEmotion, 0, len(items))
	for _, e := range items {
		res = append(res, &core_api.AnalysisEmotion{Type: e.Type, Intensity: e.Intensity})
	}
	return res
}

// buildEmotionRatio 按每用户最后一份报表的情绪类型统计分布（key 为 SimpleReport.Emotion 字符串）
func buildEmotionRatio(stats []*report.UserPsychStat) *core_api.EmotionRatio {
	cnt := make(map[string]int32)
	var total int32
	for _, st := range stats {
		e := strings.TrimSpace(st.Emotion)
		if e == "" {
			continue
		}
		cnt[e]++
		total++
	}
	return &core_api.EmotionRatio{
		Ratio: util.StrCnt2Ratio(cnt, total),
		Total: total,
	}
}

// buildRiskDistribution 按每用户最后一份报表的风险等级×性别统计分布
func buildRiskDistribution(stats []*report.UserPsychStat) []*core_api.RiskDistribution {
	res := make([]*core_api.RiskDistribution, 0, 8)
	for lvl := int32(1); lvl <= 4; lvl++ {
		for g := int32(1); g <= 2; g++ {
			res = append(res, &core_api.RiskDistribution{Level: lvl, Gender: g, Count: 0})
		}
	}
	for _, st := range stats {
		if st.Gender < 1 || st.Gender > 2 {
			continue
		}
		lvl := st.RiskLevel
		if lvl < int32(enum.UserRiskLevelHigh) || lvl > int32(enum.UserRiskLevelNormal) {
			continue // 0=未明确或旧数据，不计入分布
		}
		idx := (lvl-1)*2 + (st.Gender - 1)
		res[idx].Count++
	}
	return res
}

// aggregateGradesAndClasses 整理年级-班级统计与班主任信息为响应结构
func aggregateGradesAndClasses(mapperRes []*user.ClassStatResult, clsTeachers user.ClassTeachers) []*core_api.GradeInfo {
	if len(mapperRes) == 0 {
		return make([]*core_api.GradeInfo, 0)
	}

	gradeMap := make(map[int]*core_api.GradeInfo)
	// 将入参切片（有序）填充入有序map
	for _, item := range mapperRes {
		gradeInfo, exists := gradeMap[int(item.Info.Grade)]
		// 响应中年级尚不存在 创建该年级
		if !exists {
			gradeInfo = &core_api.GradeInfo{
				Grade:   item.Info.Grade,
				Classes: make([]*core_api.ClassInfo, 0),
			}
			gradeMap[int(item.Info.Grade)] = gradeInfo
		}
		// 年级已存在
		uNum := item.UserNum
		aNum := item.AlarmNum

		// 检查班主任是否存在，避免空指针 panic
		var teacherName, teacherPhone string
		if clsTeachers[int(item.Info.Grade)] != nil &&
			clsTeachers[int(item.Info.Grade)][int(item.Info.Class)] != nil {
			teacherName = clsTeachers[int(item.Info.Grade)][int(item.Info.Class)].Name
			teacherPhone = clsTeachers[int(item.Info.Grade)][int(item.Info.Class)].Code
		}

		gradeInfo.Classes = append(gradeInfo.Classes, &core_api.ClassInfo{
			Class:        item.Info.Class,
			UserNum:      uNum,
			AlarmNum:     aNum,
			TeacherName:  teacherName,
			TeacherPhone: teacherPhone,
		})
	}

	// 有序map转为有序切片
	grades := make([]*core_api.GradeInfo, 0, len(gradeMap))
	for _, grade := range gradeMap {
		grades = append(grades, grade)
	}
	// 确保排序
	sort.Slice(grades, func(i, j int) bool {
		return grades[i].Grade < grades[j].Grade
	})

	return grades
}
