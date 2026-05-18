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
	"github.com/xh-polaris/psych-core-api/biz/domain/auth"
	"github.com/xh-polaris/psych-core-api/biz/infra/util"
	"github.com/xh-polaris/psych-core-api/types/enum"

	"github.com/xh-polaris/psych-core-api/biz/infra/mapper/conversation"
	"github.com/xh-polaris/psych-core-api/biz/infra/mapper/report"
	"github.com/xh-polaris/psych-core-api/biz/infra/mapper/unit"

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
	AuthDomain         auth.IAuthDomain
	AlarmMapper        alarm.IMongoMapper
	UserMapper         user.IMongoMapper
	UnitMapper         unit.IMongoMapper
	ConversationMapper conversation.IMongoMapper
	ReportMapper       report.IMongoMapper
}

var AlarmServiceSet = wire.NewSet(
	wire.Struct(new(AlarmService), "*"),
	wire.Bind(new(IAlarmService), new(*AlarmService)),
)

func (s *AlarmService) Overview(ctx context.Context, req *core_api.DashboardGetAlarmOverviewReq) (resp *core_api.DashboardGetAlarmOverviewResp, err error) {
	meta, err := s.AuthDomain.ExtraUserMeta(ctx)
	if err != nil {
		return nil, err
	}

	var unitOID bson.ObjectID
	if req.UnitId != "" {
		id, err := bson.ObjectIDFromHex(req.UnitId)
		if err != nil {
			return nil, errorx.New(errno.ErrInvalidParams, errorx.KV("field", "UnitID"), errorx.KV("value", "单位ID"))
		}
		unitOID = id

		// 班主任（精确匹配）
		if meta.Role == enum.UserRoleClassTeacher {
			return s.getAlarmOverviewClassTeacher(ctx, meta.UserId, unitOID)
		}
		// 单位管理员
		if err := s.AuthDomain.VerifyUnitAdmin(meta, req.UnitId); err != nil {
			return nil, err
		}
	} else {
		// 管理端 - 需要超管
		if err := s.AuthDomain.VerifySuperAdmin(ctx, meta); err != nil {
			return nil, err
		}
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
	meta, err := s.AuthDomain.ExtraUserMeta(ctx)
	if err != nil {
		return nil, err
	}

	var unitOID bson.ObjectID
	if req.UnitId != "" {
		id, err := bson.ObjectIDFromHex(req.UnitId)
		if err != nil {
			return nil, errorx.New(errno.ErrInvalidParams, errorx.KV("field", "UnitID"), errorx.KV("value", "单位ID"))
		}
		unitOID = id
	} else {
		// 管理端 - 需要超管
		if err := s.AuthDomain.VerifySuperAdmin(ctx, meta); err != nil {
			return nil, err
		}
	}

	filter := bson.M{
		cst.UnitID: unitOID,
	}

	if req.UnitId != "" {
		// 班主任（精确匹配）
		if meta.Role == enum.UserRoleClassTeacher {
			grades, classes, err := s.getClassTeacherGradesClasses(ctx, meta.UserId)
			if err != nil {
				return nil, err
			}
			if len(grades) > 0 {
				filter[cst.Grade] = bson.M{"$in": grades}
				filter[cst.Class] = bson.M{"$in": classes}
			}
		} else if err := s.AuthDomain.VerifyUnitAdmin(meta, req.UnitId); err != nil {
			return nil, err
		}
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
		userInfo, userErr = s.UserMapper.BatchFindByIDs(ctx, userIds)
		if userErr != nil {
			logs.Errorf("批量查询用户信息失败: %v", errorx.ErrorWithoutStack(userErr))
		}
	}()
	go func() {
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

	unitIds := make(map[bson.ObjectID]bool)
	for _, u := range userInfo {
		unitIds[u.UnitID] = true
	}

	unitMap := make(map[bson.ObjectID]*unit.Unit)
	for unitId := range unitIds {
		u, err := s.UnitMapper.FindOneById(ctx, unitId)
		if err != nil {
			logs.Errorf("查询单位信息失败: %v", errorx.ErrorWithoutStack(err))
			continue
		}
		unitMap[unitId] = u
	}

	records := make([]*core_api.AlarmRecord, len(dbAlarms))
	for i, al := range dbAlarms {
		dbUser, userExists := userInfo[al.UserID]
		msgStats, msgExists := msgStats[al.UserID]
		if userExists {
			var calculatedGrade int32
			if u, ok := unitMap[dbUser.UnitID]; ok {
				calculatedGrade = int32(dbUser.CalculateGrade(u.StartGrade))
			}
			records[i] = &core_api.AlarmRecord{
				Id:       al.ID.Hex(),
				Emotion:  int32(al.Emotion),
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
			records[i].TotalConversationRounds = msgStats.Rounds
			records[i].LastConversationTime = msgStats.LatestTime
		}
	}

	return records, nil
}

func (s *AlarmService) UpdateAlarm(ctx context.Context, req *core_api.DashboardUpdateAlarmReq) (resp *core_api.DashboardUpdateAlarmResp, err error) {
	meta, err := s.AuthDomain.ExtraUserMeta(ctx)
	if err != nil {
		return nil, err
	}

	if req.Alarm == nil {
		return nil, errorx.New(errno.ErrMissingParams, errorx.KV("field", "预警信息"))
	}

	alarmId, err := bson.ObjectIDFromHex(req.Alarm.Id)
	if err != nil {
		logs.Errorf("parse alarm id error: %s", errorx.ErrorWithoutStack(err))
		return nil, errorx.New(errno.ErrInvalidParams, errorx.KV("field", "预警ID"))
	}

	oldAlarm, err := s.AlarmMapper.FindOneById(ctx, alarmId)
	if err != nil {
		logs.Errorf("find alarm error: %s", errorx.ErrorWithoutStack(err))
		return nil, errorx.New(errno.ErrNotFound)
	}
	if err := s.AuthDomain.VerifyUnitAdmin(meta, oldAlarm.UnitID.Hex()); err != nil {
		return nil, err
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

// getAlarmOverviewClassTeacher 班主任版预警概览
func (s *AlarmService) getAlarmOverviewClassTeacher(ctx context.Context, userId string, unitOID bson.ObjectID) (*core_api.DashboardGetAlarmOverviewResp, error) {
	userOID, err := bson.ObjectIDFromHex(userId)
	if err != nil {
		return nil, errorx.New(errno.ErrInvalidParams, errorx.KV("field", "UserID"))
	}

	boundClasses, err := s.UserMapper.GetClassTeacherBoundClasses(ctx, userOID)
	if err != nil {
		logs.Errorf("get class teacher bound classes error: %s", errorx.ErrorWithoutStack(err))
		return nil, errorx.New(errno.ErrDashboardAlarmUserStat)
	}

	if len(boundClasses) == 0 {
		return &core_api.DashboardGetAlarmOverviewResp{
			Total:       0,
			Processed:   0,
			Pending:     0,
			Track:       0,
			TotalChange: 0,
			Code:        0,
			Msg:         "success",
		}, nil
	}

	grades := make([]int32, 0, len(boundClasses))
	classes := make([]int32, 0, len(boundClasses))
	for _, bc := range boundClasses {
		grades = append(grades, int32(7))
		classes = append(classes, int32(bc.Class))
	}

	st, err := s.AlarmMapper.AggregateStatsByClassList(ctx, unitOID, grades, classes, time.Time{}, time.Time{})
	if err != nil {
		logs.Errorf("aggregate alarm by class list error: %s", errorx.ErrorWithoutStack(err))
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

// getClassTeacherGradesClasses 获取班主任的年级班级列表
func (s *AlarmService) getClassTeacherGradesClasses(ctx context.Context, userId string) ([]int32, []int32, error) {
	userOID, err := bson.ObjectIDFromHex(userId)
	if err != nil {
		return nil, nil, errorx.New(errno.ErrInvalidParams, errorx.KV("field", "UserID"))
	}

	boundClasses, err := s.UserMapper.GetClassTeacherBoundClasses(ctx, userOID)
	if err != nil {
		logs.Errorf("get class teacher bound classes error: %s", errorx.ErrorWithoutStack(err))
		return nil, nil, errorx.New(errno.ErrDashboardAlarmUserStat)
	}

	grades := make([]int32, 0, len(boundClasses))
	classes := make([]int32, 0, len(boundClasses))
	for _, bc := range boundClasses {
		grades = append(grades, int32(7))
		classes = append(classes, int32(bc.Class))
	}

	return grades, classes, nil
}
