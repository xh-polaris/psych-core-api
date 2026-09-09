package dashboard

import (
	"context"
	"time"

	"github.com/xh-polaris/psych-core-api/biz/application/dto/core_api"
	"github.com/xh-polaris/psych-core-api/biz/cst"
	"github.com/xh-polaris/psych-core-api/biz/infra/mapper/user"
	"github.com/xh-polaris/psych-core-api/pkg/errorx"
	"github.com/xh-polaris/psych-core-api/pkg/logs"
	"go.mongodb.org/mongo-driver/v2/bson"
)

// ==================== 工具类辅助（纯函数/数据，不涉及存储） ====================

// buildTrendPoints 将按星期聚合的计数转为固定 7 个趋势点（1=Mon ... 7=Sun，缺失补 0）
func buildTrendPoints(cnt map[int32]int32) []*core_api.TrendPoint {
	points := make([]*core_api.TrendPoint, 0, 7)
	for w := int32(1); w <= 7; w++ {
		points = append(points, &core_api.TrendPoint{Week: w, Hour: 0, Count: cnt[w]})
	}
	return points
}

// fullDayWindow 将起讫时间规整为整日窗口（起始日 0 点 ~ 结束日 24 点）
func fullDayWindow(start, end time.Time) (time.Time, time.Time) {
	days := dayRange(start, end)
	return days[0], days[len(days)-1].AddDate(0, 0, 1)
}

func dayRange(start, end time.Time) []time.Time {
	s := time.Date(start.Year(), start.Month(), start.Day(), 0, 0, 0, 0, start.Location())
	e := time.Date(end.Year(), end.Month(), end.Day(), 0, 0, 0, 0, end.Location())
	if s.After(e) {
		e = s
	}
	days := 0
	for d := s; !d.After(e); d = d.AddDate(0, 0, 1) {
		days++
	}
	out := make([]time.Time, 0, days)
	for d := s; !d.After(e); d = d.AddDate(0, 0, 1) {
		out = append(out, d)
	}
	return out
}

// durationBucketDefs 对话时长分桶边界（max=-1 表示无上限）
var durationBucketDefs = []struct{ min, max float64 }{
	{0, 5}, {6, 10}, {11, 20}, {21, 30}, {31, 60}, {61, 120}, {121, -1},
}

// updateRemark 更新用户备注（单位管理/班主任共用）
func (d *DashboardDomain) updateRemark(ctx context.Context, userOID bson.ObjectID, remark string) error {
	update := bson.M{
		cst.Remark: &user.Remark{
			Content:    remark,
			CreateTime: time.Now(),
		},
	}
	if err := d.UserMapper.UpdateFields(ctx, userOID, update); err != nil {
		logs.Errorf("update user error: %s", errorx.ErrorWithoutStack(err))
		return err
	}
	return nil
}
