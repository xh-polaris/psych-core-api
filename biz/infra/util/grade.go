package util

import "time"

// CalculateGrade 根据入学年份和单位起始年级计算当前年级
// startGrade: 单位起始年级 (如小学=1, 初中=6/7, 高中=10)
// enrollYear: 入学年份
func CalculateGrade(startGrade int, enrollYear int) int {
	now := time.Now()
	// 9 月之后是新学年
	if now.Month() >= time.September {
		return startGrade - enrollYear + now.Year()
	}
	return startGrade - enrollYear + now.Year() - 1
}
