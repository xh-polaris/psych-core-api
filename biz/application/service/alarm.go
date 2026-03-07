package service

import (
	"context"
	"sync"
	"time"

	"github.com/xh-polaris/psych-core-api/biz/infra/mapper/conversation"
	"github.com/xh-polaris/psych-core-api/biz/infra/mapper/report"

	"github.com/google/wire"
	"github.com/xh-polaris/psych-core-api/biz/cst"
	"github.com/xh-polaris/psych-core-api/biz/infra/mapper/alarm"
	"github.com/xh-polaris/psych-core-api/biz/infra/mapper/user"
	"github.com/xh-polaris/psych-core-api/pkg/errorx"
	"github.com/xh-polaris/psych-core-api/pkg/logs"
	"github.com/xh-polaris/psych-core-api/types/errno"
	"github.com/xh-polaris/psych-idl/kitex_gen/basic"
	"github.com/xh-polaris/psych-idl/kitex_gen/core_api"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
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
	unitOID, err := bson.ObjectIDFromHex(req.UnitId)
	if err != nil {
		return nil, errorx.New(errno.ErrInvalidParams, errorx.KV("field", "UnitID"), errorx.KV("value", "单位ID"))
	}

	st, err := s.AlarmMapper.AggregateStats(ctx, unitOID, time.Time{}, time.Time{})
	if err != nil {
		logs.Errorf("aggregate alarm error: %s", errorx.ErrorWithoutStack(err))
		return nil, err
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
		Code:            200,
		Msg:             "success",
	}, nil
}

func (s *AlarmService) ListRecords(ctx context.Context, req *core_api.DashboardListAlarmRecordsReq) (resp *core_api.DashboardListAlarmRecordsResp, err error) {
	unitOID, err := bson.ObjectIDFromHex(req.UnitId)
	if err != nil {
		return nil, errorx.New(errno.ErrInvalidParams, errorx.KV("field", "UnitID"), errorx.KV("value", "单位ID"))
	}

	// 先count total 若为0直接返回响应
	total, err := s.AlarmMapper.CountByTime(ctx, unitOID, time.Time{}, time.Time{})
	if total == 0 {
		return &core_api.DashboardListAlarmRecordsResp{
			Pagination: &basic.Pagination{
				Total:   0,
				HasNext: false,
			},
			Code: 200,
			Msg:  "success",
		}, nil
	}

	// 再retrieve 此时alarm数不应为0
	opt := findPageOption(req.PaginationOptions).SetSort(bson.D{{cst.Status, -1}})
	alarms, err := s.AlarmMapper.RetrieveByTime(ctx, unitOID, time.Time{}, time.Time{}, opt)
	if err != nil || len(alarms) == 0 {
		logs.Errorf("retrieve alarms error: %s", errorx.ErrorWithoutStack(err))
		return nil, errorx.New(errno.ErrInternalError)
	}
	completeAlarm, err2 := s.completeAlarm(ctx, alarms)

	// 构建响应
	hasNext := req.PaginationOptions.GetPage()*req.PaginationOptions.GetLimit() < int64(total)
	return &core_api.DashboardListAlarmRecordsResp{
		Records: completeAlarm,
		Pagination: &basic.Pagination{
			Total:   int64(total),
			Page:    req.PaginationOptions.GetPage(),
			Limit:   req.PaginationOptions.GetLimit(),
			HasNext: hasNext,
		},
		Code: 200,
		Msg:  "success",
	}, err2
}

func findPageOption(reqOpt *basic.PaginationOptions) *options.FindOptionsBuilder {
	p := reqOpt.GetPage() - 1
	l := reqOpt.GetLimit()
	return options.Find().SetSkip(p * l).SetLimit(l)
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
	var keyWords map[bson.ObjectID][]string
	var userErr, msgErr, kwErr error

	var wg sync.WaitGroup
	wg.Add(3)
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
	go func() {
		defer wg.Done()
		keyWords, kwErr = s.ReportMapper.BatchGetUserKeyWords(ctx, userIds)
		if kwErr != nil {
			logs.Errorf("查询关键词失败: %v", errorx.ErrorWithoutStack(kwErr))
		}
	}()
	wg.Wait()

	if userErr != nil {
		return nil, errorx.New(errno.ErrUserNotFound)
	}
	if msgErr != nil {
		return nil, errorx.New(errno.ErrDashboardConversationStat)
	}
	if kwErr != nil {
		return nil, errorx.New(errno.ErrGetReportKeyWord)
	}

	// 构建响应
	records := make([]*core_api.AlarmRecord, len(dbAlarms))
	for i, al := range dbAlarms {
		dbUser, userExists := userInfo[al.UserID]
		msgStats, msgExists := msgStats[al.UserID]
		kw, kwExists := keyWords[al.UserID]
		if userExists {
			records[i] = &core_api.AlarmRecord{
				Id:       al.ID.Hex(),
				Emotion:  alarm.EmotionItoS[al.Emotion],
				Keywords: al.Keywords,
				Status:   alarm.StatusItoS[al.Status],
				User: &core_api.User{
					Code:  dbUser.Code,
					Name:  dbUser.Name,
					Grade: dbUser.Grade,
					Class: dbUser.Class,
				},
			}
		}
		if msgExists {
			records[i].TotalConversationRounds = msgStats.Rounds
			records[i].LastConversationTime = msgStats.LatestTime
		}
		if kwExists {
			records[i].Keywords = kw
		}
	}

	return records, nil
}

func (s *AlarmService) UpdateAlarm(ctx context.Context, req *core_api.DashboardUpdateAlarmReq) (resp *core_api.DashboardUpdateAlarmResp, err error) {
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

	// 构建更新字段
	update := bson.M{}

	// 更新情绪状态
	if req.Alarm.Emotion != "" {
		emotionValue, ok := alarm.EmotionStoI[req.Alarm.Emotion]
		if !ok {
			return nil, errorx.New(errno.ErrInvalidParams, errorx.KV("field", "情绪状态"))
		}
		update[cst.Emotion] = emotionValue
	}

	// 更新关键词
	if len(req.Alarm.Keywords) > 0 {
		update[cst.Keywords] = req.Alarm.Keywords
	}

	// 更新处理状态
	if req.Alarm.Status != "" {
		statusValue, ok := alarm.StatusStoI[req.Alarm.Status]
		if !ok {
			return nil, errorx.New(errno.ErrInvalidParams, errorx.KV("field", "处理状态"))
		}
		update[cst.Status] = statusValue
	}

	// 更新时间
	update[cst.UpdateTime] = time.Now()

	// 执行更新
	if len(update) > 0 {
		if err = s.AlarmMapper.UpdateFields(ctx, alarmId, update); err != nil {
			logs.Errorf("update alarm error: %s", errorx.ErrorWithoutStack(err))
			return nil, err
		}
	}

	// 构造返回结果
	return &core_api.DashboardUpdateAlarmResp{
		Code: 200,
		Msg:  "success",
	}, nil
}
