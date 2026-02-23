package service

import (
	"context"
	"sync"
	"time"

	"github.com/google/wire"
	"github.com/xh-polaris/psych-core-api/biz/cst"
	"github.com/xh-polaris/psych-core-api/biz/infra/mapper/alarm"
	"github.com/xh-polaris/psych-core-api/biz/infra/mapper/message"
	"github.com/xh-polaris/psych-core-api/biz/infra/mapper/user"
	"github.com/xh-polaris/psych-core-api/pkg/errorx"
	"github.com/xh-polaris/psych-core-api/pkg/logs"
	"github.com/xh-polaris/psych-core-api/types/errno"
	"github.com/xh-polaris/psych-idl/kitex_gen/basic"
	"github.com/xh-polaris/psych-idl/kitex_gen/core_api"
	"github.com/xh-polaris/psych-idl/kitex_gen/profile"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

type IAlarmService interface {
	Overview(ctx context.Context, req *core_api.DashboardGetAlarmOverviewReq) (*core_api.DashboardGetAlarmOverviewResp, error)
	ListRecords(ctx context.Context, req *core_api.DashboardListAlarmRecordsReq) (*core_api.DashboardListAlarmRecordsResp, error)
}

type AlarmService struct {
	AlarmMapper   alarm.IMongoMapper
	UserMapper    user.IMongoMapper
	MessageMapper message.MongoMapper
}

var AlarmServiceSet = wire.NewSet(
	wire.Struct(new(AlarmService), "*"),
	wire.Bind(new(IAlarmService), new(*AlarmService)),
)

func (s *AlarmService) Overview(ctx context.Context, req *core_api.DashboardGetAlarmOverviewReq) (*core_api.DashboardGetAlarmOverviewResp, error) {
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
	}, nil
}

func (s *AlarmService) ListRecords(ctx context.Context, req *core_api.DashboardListAlarmRecordsReq) (*core_api.DashboardListAlarmRecordsResp, error) {
	unitOID, err := bson.ObjectIDFromHex(req.UnitId)
	if err != nil {
		return nil, errorx.New(errno.ErrInvalidParams, errorx.KV("field", "UnitID"), errorx.KV("value", "单位ID"))
	}

	// 先count total 若为0直接返回响应
	total, err := s.AlarmMapper.CountByTime(ctx, unitOID, time.Time{}, time.Time{})
	if total == 0 {
		return &core_api.DashboardListAlarmRecordsResp{
			Pagination: &basic.Pagination{
				Total: 0,
			},
		}, nil
	}

	// 再retrieve 此时alarm数不应为0
	opt := findPageOption(req.PaginationOptions).SetSort(bson.D{{cst.Status, -1}})
	alarms, err := s.AlarmMapper.RetrieveByTime(ctx, unitOID, time.Time{}, time.Time{}, opt)
	if err != nil || len(alarms) == 0 {
		logs.Errorf("retrieve alarms error: %s", errorx.ErrorWithoutStack(err))
		return nil, err
	}
	completeAlarm, err2 := s.completeAlarm(ctx, alarms)

	// 构建响应
	return &core_api.DashboardListAlarmRecordsResp{
		Records: completeAlarm,
		Pagination: &basic.Pagination{
			Total:   total,
			Page:    req.PaginationOptions.GetPage(),
			Limit:   req.PaginationOptions.GetLimit(),
			HasNext: req.PaginationOptions.GetPage()*req.PaginationOptions.GetLimit() < total,
		},
	}, err2
}

func findPageOption(reqOpt *basic.PaginationOptions) *options.FindOptionsBuilder {
	p := reqOpt.GetPage()
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
	var msgStats map[bson.ObjectID]*message.MsgStats
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
		msgStats, msgErr = s.MessageMapper.BatchMessageStats(ctx, userIds)
		if msgErr != nil {
			logs.Warnf("查询对话统计失败: %v", errorx.ErrorWithoutStack(msgErr))
		}
	}()
	wg.Wait()

	if userErr != nil {
		return nil, errorx.New(errno.ErrUserNotFound)
	}
	if msgErr != nil {
		return nil, errorx.New(errno.ErrGetUserConversationStatic)
	}

	// 构建响应
	records := make([]*core_api.AlarmRecord, len(dbAlarms))
	for i, al := range dbAlarms {
		dbUser, userExists := userInfo[al.UserID]
		msgStats, msgExists := msgStats[al.UserID]
		if userExists {
			records[i] = &core_api.AlarmRecord{
				Id:       al.ID.String(),
				Emotion:  alarm.EmotionItoS[al.Emotion],
				Keywords: al.Keywords,
				Status:   alarm.StatusItoS[al.Status],
				User: &profile.User{
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
	}

	return records, nil
}
