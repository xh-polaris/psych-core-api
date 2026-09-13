package enum

// report status
const (
	ReportStatusDeleted    = -1
	ReportStatusProcessing = 1
	ReportStatusSuccess    = 2
)

// DistressLevel 心理困扰程度（新报表 distressLevel / analysis.distress.level，0-4）
const (
	DistressNormal   = 0 // 正常波动
	DistressMild     = 1 // 轻度困扰
	DistressModerate = 2 // 中度困扰
	DistressSevere   = 3 // 重度困扰
	DistressHighRisk = 4 // 高危状态
)

// DistressLevelLabel 将数值困扰程度映射为中文标签
func DistressLevelLabel(level int32) string {
	switch level {
	case DistressNormal:
		return "0级：正常波动"
	case DistressMild:
		return "1级：轻度困扰"
	case DistressModerate:
		return "2级：中度困扰"
	case DistressSevere:
		return "3级：重度困扰"
	case DistressHighRisk:
		return "4级：高危状态"
	default:
		return "未明确提及"
	}
}
