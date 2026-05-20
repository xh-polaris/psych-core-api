package util

import (
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
)

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

// GradeExpr 返回 MongoDB 聚合表达式，等价于 CalculateGrade(startGrade, enrollYear)
// 表达式引用文档中的 "$enroll_year" 字段
func GradeExpr(startGrade int) bson.M {
	now := time.Now()
	base := startGrade + now.Year() - 1
	return bson.M{
		"$add": bson.A{
			base,
			bson.M{"$subtract": bson.A{0, "$" + "enroll_year"}},
			bson.M{"$cond": bson.A{
				bson.M{"$gte": bson.A{bson.M{"$month": now}, 9}},
				1, 0,
			}},
		},
	}
}
