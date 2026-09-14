package enum

import "strings"

// UserGender
const (
	UserGenderMale   = 1
	UserGenderFemale = 2
	UserGenderOther  = 3
)

// UserCodeType
const (
	UserCodeTypePhone     = 1
	UserCodeTypeStudentID = 2
	UserCodeTypeEmail     = 3
)

// UserRole
const (
	UserRoleStudent      = 1
	UserRoleTeacher      = 2
	UserRoleClassTeacher = 3
	UserRoleUnitAdmin    = 4
	UserRoleSuperAdmin   = 5
)

var UserRoleI2S = map[int32]string{
	UserRoleStudent:      "Student",
	UserRoleTeacher:      "Teacher",
	UserRoleClassTeacher: "ClassTeacher",
	UserRoleUnitAdmin:    "UnitAdmin",
	UserRoleSuperAdmin:   "SuperAdmin",
}

// UserRiskLevel（与新报表 simple_report.riskLevel 数值对齐）
const (
	UserRiskLevelUnknown = 0 // 未明确提及
	UserRiskLevelHigh    = 1 // 高风险
	UserRiskLevelMedium  = 2 // 中高风险
	UserRiskLevelLow     = 3 // 中低风险
	UserRiskLevelNormal  = 4 // 低风险
)

// RiskLevelLabel 将数值风险等级映射为中文标签
func RiskLevelLabel(level int32) string {
	switch level {
	case UserRiskLevelHigh:
		return "高风险"
	case UserRiskLevelMedium:
		return "中高风险"
	case UserRiskLevelLow:
		return "中低风险"
	case UserRiskLevelNormal:
		return "低风险"
	default:
		return "未明确提及"
	}
}

// RiskLevelToInt 将报表的字符串风险等级映射为 1-4: High | Medium | Low | Normal
func RiskLevelToInt(level string) int {
	s := strings.TrimSpace(level)
	switch {
	case s == "1":
		return UserRiskLevelHigh
	case s == "2":
		return UserRiskLevelMedium
	case s == "3":
		return UserRiskLevelLow
	case s == "4":
		return UserRiskLevelNormal
	case strings.Contains(s, "高危") || strings.Contains(s, "严重") || strings.Contains(s, "紧急"):
		return UserRiskLevelHigh
	case strings.Contains(s, "较高") || strings.Contains(s, "中"):
		return UserRiskLevelMedium
	case strings.Contains(s, "低"):
		return UserRiskLevelLow
	default:
		return UserRiskLevelNormal
	}
}

// UserStatus
const (
	UserStatusActive  = 1
	UserStatusDeleted = 2
)
