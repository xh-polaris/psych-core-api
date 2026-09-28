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

// UserRiskLevel（与新报表 simple_report.riskLevel 数值对齐，数值越大风险越高）
const (
	UserRiskLevelUnknown    = -1 // 未明确提及
	UserRiskLevelLow        = 0  // 低风险
	UserRiskLevelMediumLow  = 1  // 中低风险
	UserRiskLevelMediumHigh = 2  // 中高风险
	UserRiskLevelHigh       = 3  // 高风险
)

// RiskLevelLabel 将数值风险等级映射为中文标签
func RiskLevelLabel(level int32) string {
	switch level {
	case UserRiskLevelLow:
		return "低风险"
	case UserRiskLevelMediumLow:
		return "中低风险"
	case UserRiskLevelMediumHigh:
		return "中高风险"
	case UserRiskLevelHigh:
		return "高风险"
	default:
		return "未明确提及"
	}
}

// RiskLevelToInt 将报表的字符串风险等级映射为 -1-3：Unknown | Low | MediumLow | MediumHigh | High。
func RiskLevelToInt(level string) int {
	s := strings.TrimSpace(level)
	switch {
	case s == "-1":
		return UserRiskLevelUnknown
	case s == "0":
		return UserRiskLevelLow
	case s == "1":
		return UserRiskLevelMediumLow
	case s == "2":
		return UserRiskLevelMediumHigh
	case s == "3":
		return UserRiskLevelHigh
	case strings.Contains(s, "高危") || strings.Contains(s, "严重") || strings.Contains(s, "紧急"):
		return UserRiskLevelHigh
	case strings.Contains(s, "中高") || strings.Contains(s, "较高"):
		return UserRiskLevelMediumHigh
	case strings.Contains(s, "中低") || strings.Contains(s, "较低"):
		return UserRiskLevelMediumLow
	case strings.Contains(s, "低"):
		return UserRiskLevelLow
	default:
		return UserRiskLevelUnknown
	}
}

// UserStatus
const (
	UserStatusActive  = 1
	UserStatusDeleted = 2
)
