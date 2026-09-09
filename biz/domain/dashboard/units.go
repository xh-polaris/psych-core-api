package dashboard

import (
	"context"
	"math"
	"time"

	"github.com/xh-polaris/psych-core-api/biz/application/dto/core_api"
	"github.com/xh-polaris/psych-core-api/pkg/errorx"
	"github.com/xh-polaris/psych-core-api/pkg/logs"
	"github.com/xh-polaris/psych-core-api/types/errno"
)

// ListUnits 超管端-所有单位列表
func (d *DashboardDomain) ListUnits(ctx context.Context) (*core_api.DashboardListUnitsResp, error) {
	// 查询所有单位
	units, err := d.UnitMapper.FindAll(ctx)
	if err != nil {
		logs.Errorf("list units error: %s", errorx.ErrorWithoutStack(err))
		return nil, errorx.WrapByCode(err, errno.ErrDashboardUnitStat)
	}

	respUnits := make([]*core_api.DashboardUnit, 0, len(units))

	for _, u := range units {
		unitID := u.ID

		// 用户总数
		userCount, err := d.UserMapper.CountStudents(ctx, unitID)
		if err != nil {
			logs.Errorf("count users for unit %s error: %s", u.Name, errorx.ErrorWithoutStack(err))
			continue
		}

		// 平均对话时长（分钟）
		avgMinutes, err := d.ConversationMapper.AverageDuration(ctx, &unitID)
		if err != nil {
			logs.Errorf("avg conversation duration for unit %s error: %s", u.Name, errorx.ErrorWithoutStack(err))
			avgMinutes = 0
		}
		// 保留两位小数
		avgMinutes = math.Round(avgMinutes*100) / 100

		// 高风险用户数（存在 alarm 记录的当前高危用户）
		riskCount, err := d.AlarmMapper.CountAlarmUsers(ctx, &unitID, time.Time{}, time.Now())
		if err != nil {
			logs.Errorf("count alarm users for unit %s error: %s", u.Name, errorx.ErrorWithoutStack(err))
			riskCount = 0
		}

		// 最近更新时间（单位最后更新时间）
		updateTs := u.UpdateTime.Unix()

		respUnits = append(respUnits, &core_api.DashboardUnit{
			Id:                         u.ID.Hex(),
			Name:                       u.Name,
			UserCount:                  userCount,
			RiskUserCount:              riskCount,
			AverageConversationMinutes: avgMinutes,
			UpdateTime:                 updateTs,
			// Property / Type
		})
	}

	return &core_api.DashboardListUnitsResp{
		Units: respUnits,
		Code:  0,
		Msg:   "success",
	}, nil
}
