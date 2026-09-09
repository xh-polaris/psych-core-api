package dashboard

import (
	"context"
	"regexp"
	"strings"
	"sync"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/xh-polaris/psych-core-api/biz/application/dto/core_api"
	"github.com/xh-polaris/psych-core-api/biz/cst"
	"github.com/xh-polaris/psych-core-api/biz/infra/mapper/alarm"
	"github.com/xh-polaris/psych-core-api/biz/infra/mapper/conversation"
	"github.com/xh-polaris/psych-core-api/biz/infra/mapper/unit"
	"github.com/xh-polaris/psych-core-api/biz/infra/mapper/user"
	"github.com/xh-polaris/psych-core-api/biz/infra/util"
	"github.com/xh-polaris/psych-core-api/pkg/errorx"
	"github.com/xh-polaris/psych-core-api/pkg/logs"
	"github.com/xh-polaris/psych-core-api/types/errno"
	"go.mongodb.org/mongo-driver/v2/bson"
)

// GetAlarmOverview 预警概览
func (d *DashboardDomain) GetAlarmOverview(ctx context.Context, scope *Scope, req *core_api.DashboardGetAlarmOverviewReq) (*core_api.DashboardGetAlarmOverviewResp, error) {
	endTime := util.ParseEndTime(req.GetEndTime())
	startTime := util.ParseStartTime(req.GetStartTime(), endTime)
	prevEndTime := startTime
	prevStartTime := prevEndTime.AddDate(0, 0, -7)

	var unitOID bson.ObjectID
	if scope.UnitID != nil {
		unitOID = *scope.UnitID
	}

	// 班主任：限定所带班级
	if scope.IsClassTeacher() {
		rs, err := d.resolveScope(ctx, scope)
		if err != nil {
			return nil, err
		}
		if rs.emptyClasses() {
			return &core_api.DashboardGetAlarmOverviewResp{
				Total: 0, Processed: 0, Pending: 0, Track: 0, TotalChange: 0,
				Code: 0, Msg: "success",
			}, nil
		}
		st, err := d.AlarmMapper.AggregateStatsByClassList(ctx, unitOID, rs.grades, rs.classes, startTime, endTime, prevStartTime, prevEndTime)
		if err != nil {
			logs.Errorf("aggregate alarm by class list error: %s", errorx.ErrorWithoutStack(err))
			return nil, errorx.New(errno.ErrDashboardAlarmUserStat)
		}
		return buildAlarmOverview(st), nil
	}

	st, err := d.AlarmMapper.AggregateStats(ctx, unitOID, startTime, endTime, prevStartTime, prevEndTime)
	if err != nil {
		logs.Errorf("aggregate alarm error: %s", errorx.ErrorWithoutStack(err))
		return nil, errorx.New(errno.ErrDashboardAlarmUserStat)
	}
	return buildAlarmOverview(st), nil
}

// ListAlarmRecords 预警记录列表
func (d *DashboardDomain) ListAlarmRecords(ctx context.Context, scope *Scope, req *core_api.DashboardListAlarmRecordsReq) (*core_api.DashboardListAlarmRecordsResp, error) {
	var unitOID bson.ObjectID
	if scope.UnitID != nil {
		unitOID = *scope.UnitID
	}

	filter := bson.M{
		cst.UnitID: unitOID,
	}

	// 班主任添加班级筛选
	if scope.IsClassTeacher() {
		rs, err := d.resolveScope(ctx, scope)
		if err != nil {
			return nil, err
		}
		if len(rs.grades) > 0 {
			filter[cst.Grade] = bson.M{"$in": rs.grades}
			filter[cst.Class] = bson.M{"$in": rs.classes}
		}
	}
	if req.Emotion != nil {
		filter[cst.Emotion] = req.GetEmotion()
	}
	if req.Status != nil {
		filter[cst.Status] = int(req.GetStatus())
	}
	if req.Keyword != nil {
		keyword := strings.TrimSpace(req.GetKeyword())
		if keyword != "" {
			// 基础防注入：限制长度并过滤控制字符。
			if utf8.RuneCountInString(keyword) > 64 {
				return nil, errorx.New(errno.ErrInvalidParams, errorx.KV("field", "keyword"), errorx.KV("value", "关键词长度超限"))
			}
			for _, r := range keyword {
				if unicode.IsControl(r) {
					return nil, errorx.New(errno.ErrInvalidParams, errorx.KV("field", "keyword"), errorx.KV("value", "关键词包含非法字符"))
				}
			}

			filter[cst.Keywords] = bson.M{
				cst.Regex:   regexp.QuoteMeta(keyword),
				cst.Options: "i",
			}
		}
	}

	// total 需要与筛选条件保持一致，避免分页总数与查询结果不一致。
	total, err := d.AlarmMapper.CountByFields(ctx, filter)
	if err != nil {
		logs.Errorf("[alarm mapper] CountByFields err: %s", err)
		return nil, errorx.New(errno.ErrDashboardListAlarms)
	}

	if total == 0 {
		return &core_api.DashboardListAlarmRecordsResp{
			Records:    []*core_api.AlarmRecord{},
			Pagination: util.PaginationRes(0, req.PaginationOptions),
			Code:       0,
			Msg:        "success",
		}, nil
	}

	// 构建分页和排序option
	opt := util.PagedFindOpt(req.PaginationOptions).SetSort(bson.D{{cst.Status, -1}})

	alarms, err := d.AlarmMapper.FindManyWithOption(ctx, filter, opt)
	if err != nil {
		logs.Errorf("retrieve alarms error: %s", errorx.ErrorWithoutStack(err))
		return nil, errorx.New(errno.ErrInternalError)
	}
	completeAlarm, err2 := d.completeAlarm(ctx, alarms)

	// 构建响应
	return &core_api.DashboardListAlarmRecordsResp{
		Records:    completeAlarm,
		Pagination: util.PaginationRes(total, req.PaginationOptions),
		Code:       0,
		Msg:        "success",
	}, err2
}

// UpdateAlarm 更新预警（情绪/关键词/处理状态）
func (d *DashboardDomain) UpdateAlarm(ctx context.Context, scope *Scope, req *core_api.DashboardUpdateAlarmReq) (*core_api.DashboardUpdateAlarmResp, error) {
	if req.Alarm == nil {
		return nil, errorx.New(errno.ErrMissingParams, errorx.KV("field", "预警信息"))
	}

	alarmId, err := bson.ObjectIDFromHex(req.Alarm.Id)
	if err != nil {
		logs.Errorf("parse alarm id error: %s", errorx.ErrorWithoutStack(err))
		return nil, errorx.New(errno.ErrInvalidParams, errorx.KV("field", "预警ID"))
	}

	oldAlarm, err := d.AlarmMapper.FindOneById(ctx, alarmId)
	if err != nil {
		logs.Errorf("find alarm error: %s", errorx.ErrorWithoutStack(err))
		return nil, errorx.New(errno.ErrNotFound)
	}

	// 校验目标预警归属当前范围：超管全局可改；单位管理/班主任仅限本单位（班主任班级范围由上级鉴权限制）
	if !scope.IsGlobal() && (scope.UnitID == nil || *scope.UnitID != oldAlarm.UnitID) {
		return nil, errorx.New(errno.ErrInsufficientAuth)
	}

	// 构建更新字段
	update := bson.M{}

	// 更新情绪状态
	update[cst.Emotion] = req.Alarm.Emotion

	// 更新关键词
	if len(req.Alarm.Keywords) > 0 {
		update[cst.Keywords] = req.Alarm.Keywords
	}

	// 更新处理状态
	update[cst.Status] = req.Alarm.Status

	// 更新时间
	update[cst.UpdateTime] = time.Now()

	// 执行更新
	if len(update) > 0 {
		if err = d.AlarmMapper.UpdateFields(ctx, alarmId, update); err != nil {
			logs.Errorf("update alarm error: %s", errorx.ErrorWithoutStack(err))
			return nil, errorx.New(errno.ErrInternalError)
		}
	}

	// 构造返回结果
	return &core_api.DashboardUpdateAlarmResp{
		Code: 0,
		Msg:  "success",
	}, nil
}

// completeAlarm 补全预警记录的用户信息（用户信息、对话统计、年级换算）
func (d *DashboardDomain) completeAlarm(ctx context.Context, dbAlarms []*alarm.Alarm) ([]*core_api.AlarmRecord, error) {
	userIds := make([]bson.ObjectID, len(dbAlarms))
	for i, al := range dbAlarms {
		userIds[i] = al.UserID
	}

	var userInfo map[bson.ObjectID]*user.User
	var msgStats map[bson.ObjectID]*conversation.ConvStats
	var userErr, msgErr error

	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		userInfo, userErr = d.UserMapper.BatchFindByIDs(ctx, userIds)
		if userErr != nil {
			logs.Errorf("批量查询用户信息失败: %v", errorx.ErrorWithoutStack(userErr))
		}
	}()
	go func() {
		defer wg.Done()
		msgStats, msgErr = d.ConversationMapper.BatchConvStats(ctx, userIds)
		if msgErr != nil {
			logs.Errorf("查询对话统计失败: %v", errorx.ErrorWithoutStack(msgErr))
		}
	}()
	wg.Wait()

	if userErr != nil {
		return nil, errorx.New(errno.ErrUserNotFound)
	}
	if msgErr != nil {
		return nil, errorx.New(errno.ErrDashboardConversationStat)
	}

	unitIds := make(map[bson.ObjectID]bool)
	for _, u := range userInfo {
		unitIds[u.UnitID] = true
	}

	unitMap := make(map[bson.ObjectID]*unit.Unit)
	for unitId := range unitIds {
		u, err := d.UnitMapper.FindOneById(ctx, unitId)
		if err != nil {
			logs.Errorf("查询单位信息失败: %v", errorx.ErrorWithoutStack(err))
			continue
		}
		unitMap[unitId] = u
	}

	records := make([]*core_api.AlarmRecord, len(dbAlarms))
	for i, al := range dbAlarms {
		dbUser, userExists := userInfo[al.UserID]
		stats, msgExists := msgStats[al.UserID]
		if userExists {
			var calculatedGrade int32
			if u, ok := unitMap[dbUser.UnitID]; ok {
				calculatedGrade = int32(dbUser.CalculateGrade(u.StartGrade))
			}
			records[i] = &core_api.AlarmRecord{
				Id:       al.ID.Hex(),
				Emotion:  al.Emotion,
				Keywords: al.Keywords,
				Status:   int32(al.Status),
				User: &core_api.UserVO{
					Id:    dbUser.ID.Hex(),
					Code:  dbUser.Code,
					Name:  dbUser.Name,
					Grade: calculatedGrade,
					Class: int32(dbUser.Class),
					Remark: &core_api.Remark{
						Content: dbUser.Remark.Content,
						Time:    dbUser.Remark.CreateTime.Unix(),
					},
				},
			}
		}
		if msgExists {
			records[i].TotalConversationRounds = stats.Rounds
			records[i].LastConversationTime = stats.LatestTime
		}
	}

	return records, nil
}

// buildAlarmOverview 组装预警概览响应
func buildAlarmOverview(st *alarm.OverviewStats) *core_api.DashboardGetAlarmOverviewResp {
	return &core_api.DashboardGetAlarmOverviewResp{
		Total:           st.Total,
		Processed:       st.Processed,
		Pending:         st.Pending,
		Track:           st.Track,
		TotalChange:     st.TotalChange,
		ProcessedChange: st.ProcessedChange,
		PendingChange:   st.PendingChange,
		TrackChange:     st.TrackChange,
		Code:            0,
		Msg:             "success",
	}
}
