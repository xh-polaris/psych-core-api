package service

import (
	"context"
	"regexp"
	"strings"
	"sync"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/xh-polaris/psych-core-api/biz/application/dto/core_api"
	"github.com/xh-polaris/psych-core-api/biz/infra/util"

	"github.com/xh-polaris/psych-core-api/biz/infra/mapper/conversation"
	"github.com/xh-polaris/psych-core-api/biz/infra/mapper/report"

	"github.com/google/wire"
	"github.com/xh-polaris/psych-core-api/biz/cst"
	"github.com/xh-polaris/psych-core-api/biz/infra/mapper/alarm"
	"github.com/xh-polaris/psych-core-api/biz/infra/mapper/user"
	"github.com/xh-polaris/psych-core-api/pkg/errorx"
	"github.com/xh-polaris/psych-core-api/pkg/logs"
	"github.com/xh-polaris/psych-core-api/types/errno"

	"go.mongodb.org/mongo-driver/v2/bson"
)

type IAlarmService interface {
	Overview(ctx context.Context, req *core_api.DashboardGetAlarmOverviewReq) (resp *core_api.DashboardGetAlarmOverviewResp, err error)
	ListRecords(ctx context.Context, req *core_api.DashboardListAlarmRecordsReq) (resp *core_api.DashboardListAlarmRecordsResp, err error)
	UpdateAlarm(ctx context.Context, req *core_api.DashboardUpdateAlarmReq) (resp *core_api.DashboardUpdateAlarmResp, err error)
}

type AlarmService struct {
	AlarmMapper        alarm.IMongoMapper
	UserMapper         user.IMongoMapper
	ConversationMapper conversation.IMongoMapper
	ReportMapper       report.IMongoMapper
}

var AlarmServiceSet = wire.NewSet(
	wire.Struct(new(AlarmService), "*"),
	wire.Bind(new(IAlarmService), new(*AlarmService)),
)

func (s *AlarmService) Overview(ctx context.Context, req *core_api.DashboardGetAlarmOverviewReq) (resp *core_api.DashboardGetAlarmOverviewResp, err error) {
	// 鉴权
	userMeta, err := util.ExtraUserMeta(ctx)
	if err != nil {
		return nil, err
	}

	if req.UnitId != "" {
		if !userMeta.HasUnitAdminAuth(req.UnitId) {
			return nil, errorx.New(errno.ErrInsufficientAuth)
		}
	}
	if req.UnitId == "" && !userMeta.HasSuperAdminAuth() {
		return nil, errorx.New(errno.ErrInsufficientAuth)
	}

	// 提取unitID
	unitOID, err := bson.ObjectIDFromHex(req.UnitId)
	if err != nil {
		return nil, errorx.New(errno.ErrInvalidParams, errorx.KV("field", "UnitID"), errorx.KV("value", "单位ID"))
	}

	st, err := s.AlarmMapper.AggregateStats(ctx, unitOID, time.Time{}, time.Time{})
	if err != nil {
		logs.Errorf("aggregate alarm error: %s", errorx.ErrorWithoutStack(err))
		return nil, errorx.New(errno.ErrDashboardAlarmUserStat)
	}

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
	}, nil
}

func (s *AlarmService) ListRecords(ctx context.Context, req *core_api.DashboardListAlarmRecordsReq) (resp *core_api.DashboardListAlarmRecordsResp, err error) {
	// 鉴权
	userMeta, err := util.ExtraUserMeta(ctx)
	if err != nil {
		return nil, err
	}

	if req.UnitId != "" {
		if !userMeta.HasUnitAdminAuth(req.UnitId) {
			return nil, errorx.New(errno.ErrInsufficientAuth)
		}
	}
	if req.UnitId == "" && !userMeta.HasSuperAdminAuth() {
		return nil, errorx.New(errno.ErrInsufficientAuth)
	}

	// 提取unitID
	unitOID, err := bson.ObjectIDFromHex(req.UnitId)
	if err != nil {
		return nil, errorx.New(errno.ErrInvalidParams, errorx.KV("field", "UnitID"), errorx.KV("value", "单位ID"))
	}

	// 构建筛选条件
	filter := bson.M{
		cst.UnitID: unitOID,
	}
	if req.Emotion != nil {
		filter[cst.Emotion] = int(req.GetEmotion())
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
	total, err := s.AlarmMapper.CountByFields(ctx, filter)
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

	alarms, err := s.AlarmMapper.FindManyWithOption(ctx, filter, opt)
	if err != nil {
		logs.Errorf("retrieve alarms error: %s", errorx.ErrorWithoutStack(err))
		return nil, errorx.New(errno.ErrInternalError)
	}
	completeAlarm, err2 := s.completeAlarm(ctx, alarms)

	// 构建响应
	return &core_api.DashboardListAlarmRecordsResp{
		Records:    completeAlarm,
		Pagination: util.PaginationRes(total, req.PaginationOptions),
		Code:       0,
		Msg:        "success",
	}, err2
}

func (s *AlarmService) completeAlarm(ctx context.Context, dbAlarms []*alarm.Alarm) ([]*core_api.AlarmRecord, error) {
	// 提取需获取信息的userId列表
	userIds := make([]bson.ObjectID, len(dbAlarms))
	for i, al := range dbAlarms {
		userIds[i] = al.UserID
	}

	// 并行处理：获取user基础信息和对话情况
	var userInfo map[bson.ObjectID]*user.User
	var msgStats map[bson.ObjectID]*conversation.ConvStats
	var userErr, msgErr error

	var wg sync.WaitGroup
	wg.Add(2)
	go func() { // 获取user基础信息
		defer wg.Done()
		userInfo, userErr = s.UserMapper.BatchFindByIDs(ctx, userIds)
		if userErr != nil {
			logs.Errorf("批量查询用户信息失败: %v", errorx.ErrorWithoutStack(userErr))
		}
	}()
	go func() { // 对话情况
		defer wg.Done()
		msgStats, msgErr = s.ConversationMapper.BatchConvStats(ctx, userIds)
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

	// 构建响应
	records := make([]*core_api.AlarmRecord, len(dbAlarms))
	for i, al := range dbAlarms {
		dbUser, userExists := userInfo[al.UserID]
		msgStats, msgExists := msgStats[al.UserID]
		if userExists {
			records[i] = &core_api.AlarmRecord{
				Id:       al.ID.Hex(),
				Emotion:  int32(al.Emotion),
				Keywords: al.Keywords,
				Status:   int32(al.Status),
				User: &core_api.UserVO{
					Id:    dbUser.ID.Hex(),
					Code:  dbUser.Code,
					Name:  dbUser.Name,
					Grade: int32(dbUser.Grade),
					Class: int32(dbUser.Class),
					Remark: &core_api.Remark{
						Content: dbUser.Remark.Content,
						Time:    dbUser.Remark.CreateTime.Unix(),
					},
				},
			}
		}
		if msgExists {
			records[i].TotalConversationRounds = msgStats.Rounds
			records[i].LastConversationTime = msgStats.LatestTime
		}
	}

	return records, nil
}

func (s *AlarmService) UpdateAlarm(ctx context.Context, req *core_api.DashboardUpdateAlarmReq) (resp *core_api.DashboardUpdateAlarmResp, err error) {
	userMeta, err := util.ExtraUserMeta(ctx)
	if err != nil {
		return nil, err
	}

	// 参数校验
	if req.Alarm == nil {
		return nil, errorx.New(errno.ErrMissingParams, errorx.KV("field", "预警信息"))
	}

	// 解析预警ID
	alarmId, err := bson.ObjectIDFromHex(req.Alarm.Id)
	if err != nil {
		logs.Errorf("parse alarm id error: %s", errorx.ErrorWithoutStack(err))
		return nil, errorx.New(errno.ErrInvalidParams, errorx.KV("field", "预警ID"))
	}

	// 鉴权：需要在同一unit下
	oldAlarm, err := s.AlarmMapper.FindOneById(ctx, alarmId)
	// optimize 查不到时考虑直接创建而非报错
	if err != nil {
		logs.Errorf("find alarm error: %s", errorx.ErrorWithoutStack(err))
		return nil, errorx.New(errno.ErrNotFound)
	}
	if !userMeta.HasUnitAdminAuth(oldAlarm.UnitID.Hex()) {
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
		if err = s.AlarmMapper.UpdateFields(ctx, alarmId, update); err != nil {
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
