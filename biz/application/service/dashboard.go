package service

import (
	"context"
	"errors"
	"math"
	"sort"
	"sync"
	"time"

	"github.com/xh-polaris/psych-core-api/biz/cst"

	"github.com/xh-polaris/psych-core-api/biz/application/dto/basic"
	"github.com/xh-polaris/psych-core-api/biz/application/dto/core_api"
	"github.com/xh-polaris/psych-core-api/biz/domain/auth"
	"github.com/xh-polaris/psych-core-api/biz/infra/util"
	"github.com/xh-polaris/psych-core-api/types/enum"

	"go.mongodb.org/mongo-driver/v2/mongo"

	"github.com/xh-polaris/psych-core-api/biz/domain/his"

	"github.com/xh-polaris/psych-core-api/biz/domain/wordcld"
	"github.com/xh-polaris/psych-core-api/biz/infra/mapper/alarm"
	"github.com/xh-polaris/psych-core-api/biz/infra/mapper/report"

	"github.com/google/wire"
	"github.com/xh-polaris/psych-core-api/biz/infra/mapper/conversation"
	"github.com/xh-polaris/psych-core-api/biz/infra/mapper/message"
	"github.com/xh-polaris/psych-core-api/biz/infra/mapper/unit"
	"github.com/xh-polaris/psych-core-api/biz/infra/mapper/user"
	"github.com/xh-polaris/psych-core-api/pkg/errorx"
	"github.com/xh-polaris/psych-core-api/pkg/logs"
	"github.com/xh-polaris/psych-core-api/types/errno"

	"go.mongodb.org/mongo-driver/v2/bson"
)

type IDashboardService interface {
	// 超管端-单位列表
	DashboardListUnits(ctx context.Context, req *core_api.DashboardListUnitsReq) (*core_api.DashboardListUnitsResp, error)

	// 数据看板
	DashboardGetDataOverview(ctx context.Context, req *core_api.DashboardGetDataOverviewReq) (*core_api.DashboardGetDataOverviewResp, error)
	DashboardGetDataTrend(ctx context.Context, req *core_api.DashboardGetDataTrendReq) (*core_api.DashboardGetDataTrendResp, error)
	DashboardGetPsychTrend(ctx context.Context, req *core_api.DashboardGetPsychTrendReq) (*core_api.DashboardGetPsychTrendResp, error)

	// 用户管理
	DashboardListClasses(ctx context.Context, req *core_api.DashboardListClassesReq) (*core_api.DashboardListClassesResp, error)
	DashboardListUsers(ctx context.Context, req *core_api.DashboardListUsersReq) (*core_api.DashboardListUsersResp, error)
	DashboardCreateRemark(ctx context.Context, req *core_api.DashboardCreateRemarkReq) (*core_api.DashboardCreateRemarkResp, error)

	// 对话记录
	DashboardUserConvRecords(ctx context.Context, req *core_api.DashboardUserConvRecordsReq) (*core_api.DashboardUserConvRecordsResp, error)
	DashboardUnitConvRecords(ctx context.Context, req *core_api.DashboardUnitConvRecordsReq) (*core_api.DashboardUnitConvRecordsResp, error)
	DashboardGetReport(ctx context.Context, req *core_api.DashboardGetReportReq) (*core_api.DashboardGetReportResp, error)
}

type DashboardService struct {
	AuthDomain         auth.IAuthDomain
	UserMapper         user.IMongoMapper
	UnitMapper         unit.IMongoMapper
	MessageMapper      message.IMongoMapper
	ConversationMapper conversation.IMongoMapper
	ReportMapper       report.IMongoMapper
	AlarmMapper        alarm.IMongoMapper
}

var DashboardServiceSet = wire.NewSet(
	wire.Struct(new(DashboardService), "*"),
	wire.Bind(new(IDashboardService), new(*DashboardService)),
)

// DashboardGetDataOverview 【指标总览】学生总数，活跃用户数，总对话数、平均对话时长、高风险用户数
func (s *DashboardService) DashboardGetDataOverview(ctx context.Context, req *core_api.DashboardGetDataOverviewReq) (*core_api.DashboardGetDataOverviewResp, error) {
	// 鉴权
	meta, role, err := s.AuthDomain.IdentifyRole(ctx, req.GetUnitId())
	if err != nil {
		return nil, err
	}

	now := time.Now()
	weekBefore := now.AddDate(0, 0, -7)
	twoWeeksBefore := now.AddDate(0, 0, -14)

	// 根据role返回不同结果
	switch role {
	case enum.UserRoleSuperAdmin:
		return s.overview4Admin(ctx, twoWeeksBefore, weekBefore, now)

	case enum.UserRoleUnitAdmin:
		unitOID, _ := bson.ObjectIDFromHex(req.GetUnitId())
		return s.overview4Unit(ctx, unitOID, twoWeeksBefore, weekBefore, now)

	case enum.UserRoleClassTeacher:
		unitOID, err := bson.ObjectIDFromHex(req.GetUnitId())
		if err != nil {
			return nil, errorx.New(errno.ErrInvalidParams, errorx.KV("field", "UnitID"), errorx.KV("value", "单位ID"))
		}
		pUnit, err := s.UnitMapper.FindOneById(ctx, unitOID)
		if err != nil {
			return nil, errorx.New(errno.ErrInvalidParams, errorx.KV("field", "UnitID"))
		}
		return s.overview4clsTch(ctx, meta.UserId, unitOID, pUnit, twoWeeksBefore, weekBefore, now)
	}

	// 不应到达这里
	return nil, errorx.New(errno.ErrUnImplement)
}

// overview4Admin 超管版数据概览
func (s *DashboardService) overview4Admin(ctx context.Context, twoWeeksBefore, weekBefore, now time.Time) (*core_api.DashboardGetDataOverviewResp, error) {
	uid := bson.ObjectID{}

	// 单位数
	curUnits, err := s.UnitMapper.Count(ctx)
	if err != nil {
		return nil, errorx.WrapByCode(err, errno.ErrDashboardUnitStat)
	}
	prevUnits, err := s.UnitMapper.CountByPeriod(ctx, time.Time{}, weekBefore)
	if err != nil {
		return nil, errorx.WrapByCode(err, errno.ErrDashboardUnitStat)
	}
	units := util.Wow{Cur: curUnits, Prev: prevUnits}

	// 用户数
	curUsers, err := s.UserMapper.CountStudents(ctx, uid)
	if err != nil {
		return nil, errorx.New(errno.ErrDashboardTotalUserStat)
	}
	prevUsers, err := s.UserMapper.CountStudentsByPeriod(ctx, nil, time.Time{}, weekBefore)
	if err != nil {
		return nil, errorx.New(errno.ErrDashboardTotalUserStat)
	}
	users := util.Wow{Cur: curUsers, Prev: prevUsers}

	// 活跃用户数
	curActive, err := s.ConversationMapper.CountActiveUsers(ctx, nil, weekBefore, now)
	if err != nil {
		return nil, errorx.WrapByCode(err, errno.ErrDashboardActiveUserStat)
	}
	prevActive, err := s.ConversationMapper.CountActiveUsers(ctx, nil, twoWeeksBefore, weekBefore)
	if err != nil {
		return nil, errorx.WrapByCode(err, errno.ErrDashboardActiveUserStat)
	}
	active := util.Wow{Cur: curActive, Prev: prevActive}

	// 对话频率
	curConv, err := s.ConversationMapper.CountUnitConvByPeriod(ctx, nil, weekBefore, now)
	if err != nil {
		return nil, errorx.WrapByCode(err, errno.ErrDashboardConversationStat)
	}
	prevConv, err := s.ConversationMapper.CountUnitConvByPeriod(ctx, nil, twoWeeksBefore, weekBefore)
	if err != nil {
		return nil, errorx.WrapByCode(err, errno.ErrDashboardConversationStat)
	}
	conv := util.Wow{Cur: curConv, Prev: prevConv}

	// 对话时长
	curAvg, err := s.ConversationMapper.AverageDurationByPeriod(ctx, nil, weekBefore, now)
	if err != nil {
		return nil, errorx.WrapByCode(err, errno.ErrDashboardAvgDurationStat)
	}
	prevAvg, err := s.ConversationMapper.AverageDurationByPeriod(ctx, nil, twoWeeksBefore, weekBefore)
	if err != nil {
		return nil, errorx.WrapByCode(err, errno.ErrDashboardAvgDurationStat)
	}
	curAvg = util.Round2(curAvg)
	prevAvg = util.Round2(prevAvg)

	// 高风险用户数
	curAlarmUsers, err := s.UserMapper.CountHighRiskStudents(ctx, nil, weekBefore, now)
	if err != nil {
		return nil, errorx.New(errno.ErrDashboardAlarmUserStat)
	}
	prevAlarmUsers, err := s.UserMapper.CountHighRiskStudents(ctx, nil, twoWeeksBefore, weekBefore)
	if err != nil {
		return nil, errorx.New(errno.ErrDashboardAlarmUserStat)
	}
	alrm := util.Wow{Cur: curAlarmUsers, Prev: prevAlarmUsers}

	return &core_api.DashboardGetDataOverviewResp{
		TotalUnits:                                   util.Int32Ptr(units.Cur),
		WeeklyIncreaseUnits:                          util.Int32Ptr(units.Inc()),
		WeeklyIncreaseUnitsRate:                      util.Float64Ptr(units.Rate()),
		TotalUsers:                                   users.Cur,
		WeeklyIncreaseUsers:                          users.Inc(),
		WeeklyIncreaseUsersRate:                      users.Rate(),
		ActiveUsers:                                  util.Int32Ptr(active.Cur),
		WeeklyIncreaseActiveUsers:                    util.Int32Ptr(active.Inc()),
		WeeklyIncreaseActiveUsersRate:                util.Float64Ptr(active.Rate()),
		TotalConversations:                           conv.Cur,
		WeeklyIncreaseConversations:                  conv.Inc(),
		WeeklyIncreaseConversationsRate:              conv.Rate(),
		AverageTimePerConversation:                   curAvg,
		WeeklyIncreaseAverageTimePerConversation:     util.Round2(curAvg - prevAvg),
		WeeklyIncreaseAverageTimePerConversationRate: util.Rate(int32(curAvg), int32(prevAvg)),
		AlarmUsers:                                   alrm.Cur,
		WeeklyIncreaseAlarmUsers:                     alrm.Inc(),
		WeeklyIncreaseAlarmUsersRate:                 alrm.Rate(),
		Code:                                         0,
		Msg:                                          "success",
	}, nil
}

// overview4Unit 单位管理员版数据概览
func (s *DashboardService) overview4Unit(ctx context.Context, unitOID bson.ObjectID, twoWeeksBefore, weekBefore, now time.Time) (*core_api.DashboardGetDataOverviewResp, error) {
	u := &unitOID

	// 用户数
	curUsers, err := s.UserMapper.CountStudents(ctx, unitOID)
	if err != nil {
		return nil, errorx.New(errno.ErrDashboardTotalUserStat)
	}
	prevUsers, err := s.UserMapper.CountStudentsByPeriod(ctx, u, time.Time{}, weekBefore)
	if err != nil {
		return nil, errorx.New(errno.ErrDashboardTotalUserStat)
	}
	users := util.Wow{Cur: curUsers, Prev: prevUsers}

	// 活跃用户数
	curActive, err := s.ConversationMapper.CountActiveUsers(ctx, u, weekBefore, now)
	if err != nil {
		return nil, errorx.WrapByCode(err, errno.ErrDashboardActiveUserStat)
	}
	prevActive, err := s.ConversationMapper.CountActiveUsers(ctx, u, twoWeeksBefore, weekBefore)
	if err != nil {
		return nil, errorx.WrapByCode(err, errno.ErrDashboardActiveUserStat)
	}
	active := util.Wow{Cur: curActive, Prev: prevActive}

	// 对话数
	curConv, err := s.ConversationMapper.CountUnitConvByPeriod(ctx, u, weekBefore, now)
	if err != nil {
		return nil, errorx.WrapByCode(err, errno.ErrDashboardConversationStat)
	}
	prevConv, err := s.ConversationMapper.CountUnitConvByPeriod(ctx, u, twoWeeksBefore, weekBefore)
	if err != nil {
		return nil, errorx.WrapByCode(err, errno.ErrDashboardConversationStat)
	}
	conv := util.Wow{Cur: curConv, Prev: prevConv}

	// 对话时长
	curAvg, err := s.ConversationMapper.AverageDurationByPeriod(ctx, u, weekBefore, now)
	if err != nil {
		return nil, errorx.WrapByCode(err, errno.ErrDashboardAvgDurationStat)
	}
	prevAvg, err := s.ConversationMapper.AverageDurationByPeriod(ctx, u, twoWeeksBefore, weekBefore)
	if err != nil {
		return nil, errorx.WrapByCode(err, errno.ErrDashboardAvgDurationStat)
	}
	curAvg = util.Round2(curAvg)
	prevAvg = util.Round2(prevAvg)

	// 高风险用户数
	curAlarmUsers, err := s.UserMapper.CountHighRiskStudents(ctx, u, weekBefore, now)
	if err != nil {
		return nil, errorx.New(errno.ErrDashboardAlarmUserStat)
	}
	prevAlarmUsers, err := s.UserMapper.CountHighRiskStudents(ctx, u, twoWeeksBefore, weekBefore)
	if err != nil {
		return nil, errorx.New(errno.ErrDashboardAlarmUserStat)
	}
	alrm := util.Wow{Cur: curAlarmUsers, Prev: prevAlarmUsers}

	return &core_api.DashboardGetDataOverviewResp{
		TotalUsers:                                   users.Cur,
		WeeklyIncreaseUsers:                          users.Inc(),
		WeeklyIncreaseUsersRate:                      users.Rate(),
		ActiveUsers:                                  util.Int32Ptr(active.Cur),
		WeeklyIncreaseActiveUsers:                    util.Int32Ptr(active.Inc()),
		WeeklyIncreaseActiveUsersRate:                util.Float64Ptr(active.Rate()),
		TotalConversations:                           conv.Cur,
		WeeklyIncreaseConversations:                  conv.Inc(),
		WeeklyIncreaseConversationsRate:              conv.Rate(),
		AverageTimePerConversation:                   curAvg,
		WeeklyIncreaseAverageTimePerConversation:     util.Round2(curAvg - prevAvg),
		WeeklyIncreaseAverageTimePerConversationRate: util.Rate(int32(curAvg), int32(prevAvg)),
		AlarmUsers:                                   alrm.Cur,
		WeeklyIncreaseAlarmUsers:                     alrm.Inc(),
		WeeklyIncreaseAlarmUsersRate:                 alrm.Rate(),
		Code:                                         0,
		Msg:                                          "success",
	}, nil
}

// overview4clsTch 班主任版数据概览
func (s *DashboardService) overview4clsTch(ctx context.Context, userId string, unitOID bson.ObjectID, pUnit *unit.Unit, twoWeeksBefore, weekBefore, now time.Time) (*core_api.DashboardGetDataOverviewResp, error) {
	// 获取班主任绑定的班级
	userOID, err := bson.ObjectIDFromHex(userId)
	if err != nil {
		return nil, errorx.New(errno.ErrInvalidParams, errorx.KV("field", "UserID"))
	}

	boundClasses, err := s.UserMapper.GetClassTeacherBoundClasses(ctx, userOID)
	if err != nil {
		logs.Errorf("get class teacher bound classes error: %s", errorx.ErrorWithoutStack(err))
		return nil, errorx.New(errno.ErrDashboardTotalUserStat)
	}

	if len(boundClasses) == 0 {
		return nil, errorx.New(errno.ErrNoBoundedClass, errorx.KV("id", userId), errorx.KV("role", enum.UserRoleI2S[enum.UserRoleClassTeacher]))
	}

	// 根据 EnrollYear 计算年级
	grades := make([]int32, 0, len(boundClasses))
	classes := make([]int32, 0, len(boundClasses))
	for _, bc := range boundClasses {
		grade := util.CalculateGrade(pUnit.StartGrade, bc.EnrollYear)
		grades = append(grades, int32(grade))
		classes = append(classes, int32(bc.Class))
	}

	// 学生数统计
	curUsers, err := s.UserMapper.CountStudentsByClassList(ctx, unitOID, grades, classes)
	if err != nil {
		logs.Errorf("count students by class list error: %s", errorx.ErrorWithoutStack(err))
		return nil, errorx.New(errno.ErrDashboardTotalUserStat)
	}
	prevUsers, err := s.UserMapper.CountStudentsByPeriodAndClassList(ctx, &unitOID, grades, classes, time.Time{}, weekBefore)
	if err != nil {
		logs.Errorf("count students by period and class list error: %s", errorx.ErrorWithoutStack(err))
		return nil, errorx.New(errno.ErrDashboardTotalUserStat)
	}

	usrIncre := curUsers - prevUsers
	var usrIncreRate float64
	if prevUsers > 0 {
		usrIncreRate = float64(usrIncre) / float64(prevUsers)
	}

	// 活跃用户统计
	activeThisWeek, err := s.ConversationMapper.CountActiveUsersByClassList(ctx, grades, classes, weekBefore, now)
	if err != nil {
		logs.Errorf("count active users by class list error: %s", errorx.ErrorWithoutStack(err))
		return nil, errorx.WrapByCode(err, errno.ErrDashboardActiveUserStat)
	}

	activeLastWeek, err := s.ConversationMapper.CountActiveUsersByClassList(ctx, grades, classes, twoWeeksBefore, weekBefore)
	if err != nil {
		logs.Errorf("count active users last week by class list error: %s", errorx.ErrorWithoutStack(err))
		return nil, errorx.WrapByCode(err, errno.ErrDashboardActiveUserStat)
	}

	actIncre := activeThisWeek - activeLastWeek
	var actIncreRate float64
	if activeLastWeek > 0 {
		actIncreRate = float64(actIncre) / float64(activeLastWeek)
	}

	// 对话统计
	convThisWeek, err := s.ConversationMapper.CountConversationsByClassList(ctx, grades, classes, weekBefore, now)
	if err != nil {
		logs.Errorf("count conversations this week by class list error: %s", errorx.ErrorWithoutStack(err))
		return nil, errorx.WrapByCode(err, errno.ErrDashboardConversationStat)
	}

	convLastWeek, err := s.ConversationMapper.CountConversationsByClassList(ctx, grades, classes, twoWeeksBefore, weekBefore)
	if err != nil {
		logs.Errorf("count conversations last week by class list error: %s", errorx.ErrorWithoutStack(err))
		return nil, errorx.WrapByCode(err, errno.ErrDashboardConversationStat)
	}

	totalConv, err := s.ConversationMapper.CountConversationsByClassList(ctx, grades, classes, time.Time{}, now)
	if err != nil {
		logs.Errorf("count conversations by class list error: %s", errorx.ErrorWithoutStack(err))
		return nil, errorx.WrapByCode(err, errno.ErrDashboardConversationStat)
	}

	convIncre := convThisWeek - convLastWeek
	var convIncreRate float64
	if convLastWeek > 0 {
		convIncreRate = float64(convIncre) / float64(convLastWeek)
	}

	// 平均对话时长
	avgThisWeek, err := s.ConversationMapper.AverageDurationByClassListAndPeriod(ctx, grades, classes, weekBefore, now)
	if err != nil {
		logs.Errorf("avg duration this week by class list error: %s", errorx.ErrorWithoutStack(err))
		avgThisWeek = 0
	}

	avgLastWeek, err := s.ConversationMapper.AverageDurationByClassListAndPeriod(ctx, grades, classes, twoWeeksBefore, weekBefore)
	if err != nil {
		logs.Errorf("avg duration last week by class list error: %s", errorx.ErrorWithoutStack(err))
		avgLastWeek = 0
	}

	avgThisWeek = util.Round2(avgThisWeek)
	avgLastWeek = util.Round2(avgLastWeek)
	avgIncre := avgThisWeek - avgLastWeek
	var avgIncreRate float64
	if avgLastWeek > 0 {
		avgIncreRate = float64(avgIncre) / float64(avgLastWeek)
	}

	// 高风险学生数
	alarmUsersThisWeek, err := s.UserMapper.CountHighRiskStudentsByClassList(ctx, grades, classes, weekBefore, now)
	if err != nil {
		logs.Errorf("count alarm users this week by class list error: %s", errorx.ErrorWithoutStack(err))
		alarmUsersThisWeek = 0
	}

	alarmUsersLastWeek, err := s.UserMapper.CountHighRiskStudentsByClassList(ctx, grades, classes, twoWeeksBefore, weekBefore)
	if err != nil {
		logs.Errorf("count alarm users last week by class list error: %s", errorx.ErrorWithoutStack(err))
		alarmUsersLastWeek = 0
	}

	alarmUsersIncre := alarmUsersThisWeek - alarmUsersLastWeek
	var alarmUsersIncreRate float64
	if alarmUsersLastWeek > 0 {
		alarmUsersIncreRate = float64(alarmUsersIncre) / float64(alarmUsersLastWeek)
	}

	return &core_api.DashboardGetDataOverviewResp{
		TotalUsers:                                   curUsers,
		WeeklyIncreaseUsers:                          usrIncre,
		WeeklyIncreaseUsersRate:                      usrIncreRate,
		ActiveUsers:                                  &activeThisWeek,
		WeeklyIncreaseActiveUsers:                    &actIncre,
		WeeklyIncreaseActiveUsersRate:                &actIncreRate,
		TotalConversations:                           totalConv,
		WeeklyIncreaseConversations:                  convIncre,
		WeeklyIncreaseConversationsRate:              convIncreRate,
		AverageTimePerConversation:                   avgThisWeek,
		WeeklyIncreaseAverageTimePerConversation:     avgIncre,
		WeeklyIncreaseAverageTimePerConversationRate: avgIncreRate,
		AlarmUsers:                                   alarmUsersThisWeek,
		WeeklyIncreaseAlarmUsers:                     alarmUsersIncre,
		WeeklyIncreaseAlarmUsersRate:                 alarmUsersIncreRate,
		Code:                                         0,
		Msg:                                          "success",
	}, nil
}

// DashboardGetDataTrend 获取近一周活跃用户数、每日对话频率、对话平均时长分布、各年级预警数占比
func (s *DashboardService) DashboardGetDataTrend(ctx context.Context, req *core_api.DashboardGetDataTrendReq) (*core_api.DashboardGetDataTrendResp, error) {
	meta, role, err := s.AuthDomain.IdentifyRole(ctx, req.GetUnitId())
	if err != nil {
		return nil, err
	}

	now := time.Now()
	todayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	startDay := todayStart.AddDate(0, 0, -6)
	toWeek := func(t time.Time) int32 {
		wd := int32(t.Weekday())
		if wd == 0 {
			return 7
		}
		return wd
	}

	switch role {
	case enum.UserRoleSuperAdmin:
		return s.dataTrend4Admin(ctx, startDay, toWeek)

	case enum.UserRoleUnitAdmin:
		oid, _ := bson.ObjectIDFromHex(req.GetUnitId())
		return s.dataTrend4Unit(ctx, oid, startDay, toWeek)

	case enum.UserRoleClassTeacher:
		oid, _ := bson.ObjectIDFromHex(req.GetUnitId())
		pUnit, _ := s.UnitMapper.FindOneById(ctx, oid)
		return s.dataTrend4clsTch(ctx, meta.UserId, oid, pUnit, startDay, toWeek)
	}
	return nil, errorx.New(errno.ErrUnImplement)
}

// dataTrend4Admin 超管版数据趋势（全局）
func (s *DashboardService) dataTrend4Admin(ctx context.Context, startDay time.Time, toWeek func(time.Time) int32) (*core_api.DashboardGetDataTrendResp, error) {
	return s.dataTrend(ctx, nil, 0, startDay, toWeek)
}

// dataTrend4Unit 单位管理员版数据趋势
func (s *DashboardService) dataTrend4Unit(ctx context.Context, unitOID bson.ObjectID, startDay time.Time, toWeek func(time.Time) int32) (*core_api.DashboardGetDataTrendResp, error) {
	pUnit, _ := s.UnitMapper.FindOneById(ctx, unitOID)
	startGrade := 1
	if pUnit != nil {
		startGrade = pUnit.StartGrade
	}
	return s.dataTrend(ctx, &unitOID, startGrade, startDay, toWeek)
}

// dataTrend 超管/单位管理共用逻辑
func (s *DashboardService) dataTrend(ctx context.Context, unitOID *bson.ObjectID, startGrade int, startDay time.Time, toWeek func(time.Time) int32) (*core_api.DashboardGetDataTrendResp, error) {
	// 活跃趋势（按天）
	activePoints := make([]*core_api.TrendPoint, 0, 7)
	for i := 0; i < 7; i++ {
		dayStart := startDay.AddDate(0, 0, i)
		dayEnd := dayStart.AddDate(0, 0, 1)
		cnt, err := s.ConversationMapper.CountActiveUsers(ctx, unitOID, dayStart, dayEnd)
		if err != nil {
			return nil, errorx.WrapByCode(err, errno.ErrDashboardActiveUserStat)
		}
		activePoints = append(activePoints, &core_api.TrendPoint{Count: cnt, Week: toWeek(dayStart)})
	}

	// 对话频率趋势（按天）
	convPoints := make([]*core_api.TrendPoint, 0, 7)
	for i := 0; i < 7; i++ {
		dayStart := startDay.AddDate(0, 0, i)
		dayEnd := dayStart.AddDate(0, 0, 1)
		cnt, err := s.ConversationMapper.CountUnitConvByPeriod(ctx, unitOID, dayStart, dayEnd)
		if err != nil {
			return nil, errorx.WrapByCode(err, errno.ErrDashboardConversationStat)
		}
		convPoints = append(convPoints, &core_api.TrendPoint{Count: cnt, Week: toWeek(dayStart)})
	}

	// 对话时长分布
	convDurations, err := s.durationBuckets(ctx, unitOID)
	if err != nil {
		return nil, err
	}

	// 各年级高风险用户数分布
	riskDistribution, err := s.riskDistrbByGrade(ctx, unitOID, startGrade)
	if err != nil {
		return nil, err
	}

	return &core_api.DashboardGetDataTrendResp{
		ActivePoints:          activePoints,
		ConversationPoints:    convPoints,
		ConversationDurations: convDurations,
		RiskDistribution:      riskDistribution,
		Code:                  0,
		Msg:                   "success",
	}, nil
}

// dataTrend4clsTch 班主任版数据趋势
func (s *DashboardService) dataTrend4clsTch(ctx context.Context, userId string, unitOID bson.ObjectID, pUnit *unit.Unit, startDay time.Time, toWeekFn func(time.Time) int32) (*core_api.DashboardGetDataTrendResp, error) {
	userOID, err := bson.ObjectIDFromHex(userId)
	if err != nil {
		return nil, errorx.New(errno.ErrInvalidParams, errorx.KV("field", "UserID"))
	}

	boundClasses, err := s.UserMapper.GetClassTeacherBoundClasses(ctx, userOID)
	if err != nil {
		logs.Errorf("get class teacher bound classes error: %s", errorx.ErrorWithoutStack(err))
		return nil, errorx.New(errno.ErrDashboardActiveUserStat)
	}

	if len(boundClasses) == 0 {
		return nil, errorx.New(errno.ErrNoBoundedClass, errorx.KV("id", userId), errorx.KV("role", enum.UserRoleI2S[enum.UserRoleClassTeacher]))
	}

	grades := make([]int32, 0, len(boundClasses))
	classes := make([]int32, 0, len(boundClasses))
	for _, bc := range boundClasses {
		grade := util.CalculateGrade(pUnit.StartGrade, bc.EnrollYear)
		grades = append(grades, int32(grade))
		classes = append(classes, int32(bc.Class))
	}

	// 活跃趋势（按天）
	activePoints := make([]*core_api.TrendPoint, 0, 7)
	for i := 0; i < 7; i++ {
		dayStart := startDay.AddDate(0, 0, i)
		dayEnd := dayStart.AddDate(0, 0, 1)
		cnt, err := s.ConversationMapper.CountActiveUsersByClassList(ctx, grades, classes, dayStart, dayEnd)
		if err != nil {
			logs.Errorf("count active users by class list trend error (day %d): %s", i, errorx.ErrorWithoutStack(err))
			return nil, errorx.WrapByCode(err, errno.ErrDashboardActiveUserStat)
		}
		activePoints = append(activePoints, &core_api.TrendPoint{
			Count: cnt,
			Week:  toWeekFn(dayStart),
			Hour:  0,
		})
	}

	// 对话频率趋势（按天）
	conversationPoints := make([]*core_api.TrendPoint, 0, 7)
	for i := 0; i < 7; i++ {
		dayStart := startDay.AddDate(0, 0, i)
		dayEnd := dayStart.AddDate(0, 0, 1)
		cnt, err := s.ConversationMapper.CountConversationsByClassList(ctx, grades, classes, dayStart, dayEnd)
		if err != nil {
			logs.Errorf("count conversations by class list trend error (day %d): %s", i, errorx.ErrorWithoutStack(err))
			return nil, errorx.WrapByCode(err, errno.ErrDashboardConversationStat)
		}
		conversationPoints = append(conversationPoints, &core_api.TrendPoint{
			Count: cnt,
			Week:  toWeekFn(dayStart),
			Hour:  0,
		})
	}

	// 对话时长分布
	conversationDurations := make([]*core_api.ConversationDuration, 0, 7)
	buckets := []struct {
		min float64
		max float64
	}{
		{0, 5}, {6, 10}, {11, 20}, {21, 30}, {31, 60}, {61, 120}, {121, -1},
	}

	for i, b := range buckets {
		cnt, err := s.ConversationMapper.CountByDurationBucketByClassList(ctx, grades, classes, b.min, b.max)
		if err != nil {
			logs.Errorf("count by duration bucket by class list error: %s", errorx.ErrorWithoutStack(err))
			return nil, errorx.WrapByCode(err, errno.ErrDashboardConversationStat)
		}
		conversationDurations = append(conversationDurations, &core_api.ConversationDuration{
			Key:   int32(i + 1),
			Count: cnt,
		})
	}

	// 各年级高风险用户数分布
	enrollYears := make([]int32, 0, len(boundClasses))
	classes = make([]int32, 0, len(boundClasses))
	for _, bc := range boundClasses {
		enrollYears = append(enrollYears, int32(bc.EnrollYear))
		classes = append(classes, int32(bc.Class))
	}

	riskMap, total, err := s.UserMapper.CountHighRiskByGradeAndClasses(ctx, pUnit.StartGrade, enrollYears, classes)
	if err != nil {
		logs.Errorf("count high risk by grade and classes error: %s", errorx.ErrorWithoutStack(err))
		return nil, errorx.WrapByCode(err, errno.ErrDashboardConversationStat)
	}

	riskDistribution := &core_api.RiskDistributionByGrade{
		Ratio: util.RiskDistributionCnt2Ratio(riskMap, total),
		Total: total,
	}

	return &core_api.DashboardGetDataTrendResp{
		ActivePoints:          activePoints,
		ConversationPoints:    conversationPoints,
		ConversationDurations: conversationDurations,
		RiskDistribution:      riskDistribution,
		Code:                  0,
		Msg:                   "success",
	}, nil
}

// durationBuckets 对话时长分桶统计
func (s *DashboardService) durationBuckets(ctx context.Context, unitOID *bson.ObjectID) ([]*core_api.ConversationDuration, error) {
	buckets := []struct{ min, max float64 }{
		{0, 5}, {6, 10}, {11, 20}, {21, 30}, {31, 60}, {61, 120}, {121, -1},
	}
	result := make([]*core_api.ConversationDuration, 0, len(buckets))
	for i, b := range buckets {
		cnt, err := s.ConversationMapper.CountByDurationBucket(ctx, unitOID, b.min, b.max)
		if err != nil {
			return nil, errorx.WrapByCode(err, errno.ErrDashboardConversationStat)
		}
		result = append(result, &core_api.ConversationDuration{Key: int32(i + 1), Count: cnt})
	}
	return result, nil
}

// riskDistrbByGrade 各年级高风险用户数分布
func (s *DashboardService) riskDistrbByGrade(ctx context.Context, unitOID *bson.ObjectID, startGrade int) (*core_api.RiskDistributionByGrade, error) {
	if unitOID == nil {
		return &core_api.RiskDistributionByGrade{Ratio: make(map[int32]int32), Total: 0}, nil
	}
	riskMap, total, err := s.UserMapper.CountHighRiskByGrade(ctx, *unitOID, startGrade)
	if err != nil {
		return nil, errorx.WrapByCode(err, errno.ErrDashboardConversationStat)
	}
	return &core_api.RiskDistributionByGrade{Ratio: util.RiskDistributionCnt2Ratio(riskMap, total), Total: total}, nil
}

// DashboardListUnits 超管端-所有单位列表
func (s *DashboardService) DashboardListUnits(ctx context.Context, req *core_api.DashboardListUnitsReq) (*core_api.DashboardListUnitsResp, error) {
	if _, _, err := s.AuthDomain.IdentifyRole(ctx, ""); err != nil {
		return nil, err
	}

	// 查询所有单位
	units, err := s.UnitMapper.FindAll(ctx)
	if err != nil {
		logs.Errorf("list units error: %s", errorx.ErrorWithoutStack(err))
		return nil, errorx.WrapByCode(err, errno.ErrDashboardUnitStat)
	}

	respUnits := make([]*core_api.DashboardUnit, 0, len(units))

	for _, u := range units {
		unitID := u.ID

		// 用户总数
		userCount, err := s.UserMapper.CountStudents(ctx, unitID)
		if err != nil {
			logs.Errorf("count users for unit %s error: %s", u.Name, errorx.ErrorWithoutStack(err))
			continue
		}

		// 平均对话时长（分钟）
		avgMinutes, err := s.ConversationMapper.AverageDuration(ctx, &unitID)
		if err != nil {
			logs.Errorf("avg conversation duration for unit %s error: %s", u.Name, errorx.ErrorWithoutStack(err))
			avgMinutes = 0
		}
		// 保留两位小数
		avgMinutes = math.Round(avgMinutes*100) / 100

		// 高风险用户数（当前单位）
		riskCount, err := s.UserMapper.CountHighRiskStudents(ctx, &unitID, time.Time{}, time.Now())
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

// DashboardGetPsychTrend 情绪分布，风险性别分布，关键词词云
func (s *DashboardService) DashboardGetPsychTrend(ctx context.Context, req *core_api.DashboardGetPsychTrendReq) (*core_api.DashboardGetPsychTrendResp, error) {
	meta, role, err := s.AuthDomain.IdentifyRole(ctx, req.GetUnitId())
	if err != nil {
		return nil, err
	}

	switch role {
	case enum.UserRoleSuperAdmin:
		return s.psychTrend4Admin(ctx)

	case enum.UserRoleUnitAdmin:
		oid, _ := bson.ObjectIDFromHex(req.GetUnitId())
		return s.psychTrend4Unit(ctx, oid)

	case enum.UserRoleClassTeacher:
		oid, _ := bson.ObjectIDFromHex(req.GetUnitId())
		pUnit, _ := s.UnitMapper.FindOneById(ctx, oid)
		return s.psychTrend4clsTch(ctx, meta.UserId, oid, pUnit)
	}
	return nil, errorx.New(errno.ErrUnImplement)
}

// psychTrend4Admin 超管版心理趋势（全局）
func (s *DashboardService) psychTrend4Admin(ctx context.Context) (*core_api.DashboardGetPsychTrendResp, error) {
	return s.psychTrend(ctx, nil)
}

// psychTrend4Unit 单位管理员版心理趋势
func (s *DashboardService) psychTrend4Unit(ctx context.Context, unitOID bson.ObjectID) (*core_api.DashboardGetPsychTrendResp, error) {
	return s.psychTrend(ctx, &unitOID)
}

// psychTrend 超管/单位管理共用逻辑
func (s *DashboardService) psychTrend(ctx context.Context, unitOID *bson.ObjectID) (*core_api.DashboardGetPsychTrendResp, error) {
	rskDistrib, err := s.getRiskDistribution(ctx, unitOID)
	if err != nil {
		return nil, errorx.WrapByCode(err, errno.ErrDashboardRiskDistribution)
	}
	keywords, err := s.getKeywords(ctx, unitOID)
	if err != nil {
		return nil, errorx.WrapByCode(err, errno.ErrDashboardGetUserKeywords)
	}
	emoRatio, err := s.getEmotionRatio(ctx, unitOID)
	if err != nil {
		return nil, errorx.WrapByCode(err, errno.ErrDashboardAlarmUserStat)
	}
	return &core_api.DashboardGetPsychTrendResp{
		EmotionRatio: emoRatio,
		Risks:        rskDistrib,
		Keywords:     keywords,
		Code:         0,
		Msg:          "success",
	}, nil
}

// psychTrend4clsTch 班主任版心理趋势
func (s *DashboardService) psychTrend4clsTch(ctx context.Context, userId string, unitOID bson.ObjectID, pUnit *unit.Unit) (*core_api.DashboardGetPsychTrendResp, error) {
	// 获取班主任绑定的班级
	userOID, _ := bson.ObjectIDFromHex(userId)

	boundClasses, err := s.UserMapper.GetClassTeacherBoundClasses(ctx, userOID)
	if err != nil {
		logs.Errorf("get class teacher bound classes error: %s", errorx.ErrorWithoutStack(err))
		return nil, errorx.New(errno.ErrDashboardTotalUserStat)
	}

	if len(boundClasses) == 0 {
		return nil, errorx.New(errno.ErrNoBoundedClass, errorx.KV("id", userId), errorx.KV("role", "classTeacher"))
	}

	// 计算班级列表
	grades := make([]int32, 0, len(boundClasses))
	classes := make([]int32, 0, len(boundClasses))
	for _, bc := range boundClasses {
		grade := util.CalculateGrade(pUnit.StartGrade, bc.EnrollYear)
		grades = append(grades, int32(grade))
		classes = append(classes, int32(bc.Class))
	}

	// 风险等级分布（按班级筛选）
	rskDistrib, err := s.getRiskDistributionByClassList(ctx, unitOID, grades, classes)
	if err != nil {
		logs.Errorf("get risk distribution by class list error: %s", errorx.ErrorWithoutStack(err))
		return nil, errorx.New(errno.ErrDashboardRiskDistribution)
	}

	// 关键词词云（目前不支持按班级筛选，返回空或简化结果）
	keywords := &core_api.Keywords{
		KeywordMap: make(map[string]int32),
		KeyTotal:   0,
	}

	// 情绪分布（按班级筛选）
	emoRatio, err := s.getEmotionRatioByClassList(ctx, unitOID, grades, classes)
	if err != nil {
		logs.Errorf("get emotion ratio by class list error: %s", errorx.ErrorWithoutStack(err))
		return nil, errorx.New(errno.ErrDashboardAlarmUserStat)
	}

	return &core_api.DashboardGetPsychTrendResp{
		EmotionRatio: emoRatio,
		Risks:        rskDistrib,
		Keywords:     keywords,
		Code:         0,
		Msg:          "success",
	}, nil
}

// getRiskDistributionByClassList 按班级列表获取风险等级分布
func (s *DashboardService) getRiskDistributionByClassList(ctx context.Context, unitOID bson.ObjectID, grades, classes []int32) ([]*core_api.RiskDistribution, error) {
	// 获取按风险等级和性别分组的统计数据
	stats, err := s.UserMapper.GetRiskDistributionByClassList(ctx, unitOID, grades, classes)
	if err != nil {
		logs.Errorf("get risk distribution by class list error: %s", errorx.ErrorWithoutStack(err))
		return nil, err
	}

	// 初始化结果（4种风险等级 * 2种性别）
	res := make([]*core_api.RiskDistribution, 0, 8)
	for lvl := int32(1); lvl <= 4; lvl++ {
		for g := int32(1); g <= 2; g++ {
			res = append(res, &core_api.RiskDistribution{
				Level:  lvl,
				Gender: g,
				Count:  0,
			})
		}
	}

	// 填充统计数据
	for _, stat := range stats {
		if stat.Level >= 1 && stat.Level <= 4 && stat.Gender >= 1 && stat.Gender <= 2 {
			// 计算在结果切片中的索引：(level-1)*2 + (gender-1)
			idx := (stat.Level-1)*2 + (stat.Gender - 1)
			if idx >= 0 && idx < int32(len(res)) {
				res[idx].Count = stat.Count
			}
		}
	}

	return res, nil
}

// getEmotionRatioByClassList 按班级列表获取情绪分布
func (s *DashboardService) getEmotionRatioByClassList(ctx context.Context, unitOID bson.ObjectID, grades, classes []int32) (*core_api.EmotionRatio, error) {
	total, err := s.UserMapper.CountStudentsByClassList(ctx, unitOID, grades, classes)
	if err != nil {
		logs.Errorf("count students by class list error: %s", errorx.ErrorWithoutStack(err))
		return nil, err
	}

	if total == 0 {
		return &core_api.EmotionRatio{Total: 0, Ratio: make(map[int32]int32)}, nil
	}

	distribution, err := s.AlarmMapper.EmotionDistributionByClassList(ctx, unitOID, grades, classes)
	if err != nil {
		logs.Errorf("get emotion distribution by class list error: %s", errorx.ErrorWithoutStack(err))
		return nil, err
	}

	ratio := make(map[int32]int32)
	for emotion, count := range *distribution {
		ratio[int32(emotion)] = count
	}

	return &core_api.EmotionRatio{
		Total: total,
		Ratio: util.RiskDistributionCnt2Ratio(ratio, total),
	}, nil
}

func (s *DashboardService) getEmotionRatio(ctx context.Context, unitOID *bson.ObjectID) (*core_api.EmotionRatio, error) {
	var (
		total int32
		err   error
	)

	if unitOID == nil {
		total, err = s.UserMapper.CountStudents(ctx, bson.ObjectID{})
	} else {
		total, err = s.UserMapper.CountStudents(ctx, *unitOID)
	}

	if err != nil {
		logs.Errorf("count users error: %s", errorx.ErrorWithoutStack(err))
		return nil, err
	}

	if total == 0 {
		return &core_api.EmotionRatio{Total: 0, Ratio: make(map[int32]int32)}, nil
	}

	emotionDistribution, err := s.AlarmMapper.EmotionDistribution(ctx, unitOID)
	if err != nil {
		logs.Errorf("[AlarmMapper] get emotion distribution error: %s", errorx.ErrorWithoutStack(err))
		return nil, err
	}
	if emotionDistribution == nil {
		return &core_api.EmotionRatio{Total: total, Ratio: make(map[int32]int32)}, nil
	}

	ratio := make(map[int32]int32, len(*emotionDistribution))
	var abnormalSum int32
	for emo, cnt := range *emotionDistribution {
		ratio[int32(emo)] = cnt
		abnormalSum += cnt
	}
	ratio[enum.AlarmEmotionNormal] = total - abnormalSum

	return &core_api.EmotionRatio{
		Ratio: util.RiskDistributionCnt2Ratio(ratio, total),
		Total: total,
	}, nil
}

func (s *DashboardService) getKeywords(ctx context.Context, unitOID *bson.ObjectID) (*core_api.Keywords, error) {
	if unitOID != nil {
		return wordcld.Extractor.FromUnitKWs(ctx, *unitOID)
	}
	return wordcld.Extractor.FromAllUnitsKWs(ctx)
}

func (s *DashboardService) getRiskDistribution(ctx context.Context, unitOID *bson.ObjectID) ([]*core_api.RiskDistribution, error) {
	// 调用 mapper 获取基础统计数据
	stats, err := s.UserMapper.RiskDistributionStats(ctx, unitOID)
	if err != nil {
		logs.Errorf("[DashboardService] getRiskDistribution mapper err: %s", errorx.ErrorWithoutStack(err))
		return nil, err
	}

	// 构造嵌套映射，补全count为0的level和gender
	countMap := make(map[int32]map[int32]int32)
	for _, s := range stats {
		if countMap[s.Level] == nil {
			countMap[s.Level] = make(map[int32]int32)
		}
		countMap[s.Level][s.Gender] = s.Count
	}

	// 返回8条结果：level 1-4, gender 1-2
	res := make([]*core_api.RiskDistribution, 0, 8)
	for lvl := int32(1); lvl <= 4; lvl++ {
		for g := int32(1); g <= 2; g++ {
			var cnt int32
			if countMap[lvl] != nil {
				cnt = countMap[lvl][g]
			}
			res = append(res, &core_api.RiskDistribution{Level: lvl, Gender: g, Count: cnt})
		}
	}

	return res, nil
}

// DashboardListClasses 【用户管理】班级列表
func (s *DashboardService) DashboardListClasses(ctx context.Context, req *core_api.DashboardListClassesReq) (*core_api.DashboardListClassesResp, error) {
	meta, role, err := s.AuthDomain.IdentifyRole(ctx, req.GetUnitId())
	if err != nil {
		return nil, err
	}

	unitOID, err := bson.ObjectIDFromHex(req.UnitId)
	if err != nil {
		return nil, errorx.New(errno.ErrInvalidParams, errorx.KV("field", "UnitID"), errorx.KV("value", "单位ID"))
	}

	pUnit, _ := s.UnitMapper.FindOneById(ctx, unitOID)

	switch role {
	case enum.UserRoleUnitAdmin:
		return s.listCls4Unit(ctx, unitOID, pUnit, req)
	case enum.UserRoleClassTeacher:
		return s.listCls4ClsTch(ctx, meta.UserId, unitOID, pUnit, req)
	}
	return nil, errorx.New(errno.ErrInvalidRole)
}

// 单位端 - 列出班级
func (s *DashboardService) listCls4Unit(ctx context.Context, unitOID bson.ObjectID, pUnit *unit.Unit, req *core_api.DashboardListClassesReq) (*core_api.DashboardListClassesResp, error) {
	// 筛选参数
	var grades, classes []int32
	if req.Grade != nil {
		grades = append(grades, *req.Grade)
	}
	if req.Class != nil {
		classes = append(classes, *req.Class)
	}

	// 查询结果
	clsStats, err := s.UserMapper.CountByClasses(ctx, unitOID, pUnit.StartGrade, grades, classes)
	clsTeachers, err := s.UserMapper.FindUnitClassTeachers(ctx, unitOID, pUnit.StartGrade)
	if err != nil {
		return nil, errorx.New(errno.ErrCountUserByClasses)
	}

	// 整理结果，构建响应
	return &core_api.DashboardListClassesResp{
		Grades: aggregateGradesAndClasses(clsStats, clsTeachers),
	}, nil
}

// 班主任端 - 列出所带班级
func (s *DashboardService) listCls4ClsTch(ctx context.Context, userId string, unitOID bson.ObjectID, pUnit *unit.Unit, req *core_api.DashboardListClassesReq) (*core_api.DashboardListClassesResp, error) {
	// 获取班主任绑定的班级
	userOID, _ := bson.ObjectIDFromHex(userId)

	boundClasses, err := s.UserMapper.GetClassTeacherBoundClasses(ctx, userOID)
	if err != nil {
		logs.Errorf("get class teacher bound classes error: %s", errorx.ErrorWithoutStack(err))
		return nil, errorx.New(errno.ErrCountUserByClasses)
	}

	if len(boundClasses) == 0 {
		return &core_api.DashboardListClassesResp{
			Grades: make([]*core_api.GradeInfo, 0),
		}, nil
	}

	// 筛选参数：只允许查看绑定的班级
	grades := make([]int32, 0, len(boundClasses))
	classes := make([]int32, 0, len(boundClasses))
	for _, bc := range boundClasses {
		grade := util.CalculateGrade(pUnit.StartGrade, bc.EnrollYear)
		grades = append(grades, int32(grade))
		classes = append(classes, int32(bc.Class))
	}

	// 如果请求中有筛选条件，进一步过滤
	if req.Grade != nil || req.Class != nil {
		filteredGrades := make([]int32, 0)
		filteredClasses := make([]int32, 0)
		for i, g := range grades {
			if req.Grade != nil && g != *req.Grade {
				continue
			}
			if req.Class != nil && classes[i] != *req.Class {
				continue
			}
			filteredGrades = append(filteredGrades, g)
			filteredClasses = append(filteredClasses, classes[i])
		}
		grades = filteredGrades
		classes = filteredClasses
	}

	// 查询结果
	clsStats, err := s.UserMapper.CountByClasses(ctx, unitOID, pUnit.StartGrade, grades, classes)
	clsTeachers, err := s.UserMapper.FindUnitClassTeachers(ctx, unitOID, pUnit.StartGrade)
	if err != nil {
		return nil, errorx.New(errno.ErrCountUserByClasses)
	}

	// 整理结果，构建响应
	return &core_api.DashboardListClassesResp{
		Grades: aggregateGradesAndClasses(clsStats, clsTeachers),
	}, nil
}

func aggregateGradesAndClasses(mapperRes []*user.ClassStatResult, clsTeachers user.ClassTeachers) []*core_api.GradeInfo {
	if len(mapperRes) == 0 {
		return make([]*core_api.GradeInfo, 0)
	}

	gradeMap := make(map[int]*core_api.GradeInfo)
	// 将入参切片（有序）填充入有序map
	for _, item := range mapperRes {
		gradeInfo, exists := gradeMap[int(item.Info.Grade)]
		// 响应中年级尚不存在 创建该年级
		if !exists {
			gradeInfo = &core_api.GradeInfo{
				Grade:   item.Info.Grade,
				Classes: make([]*core_api.ClassInfo, 0),
			}
			gradeMap[int(item.Info.Grade)] = gradeInfo
		}
		// 年级已存在
		uNum := item.UserNum
		aNum := item.AlarmNum

		// 检查班主任是否存在，避免空指针 panic
		var teacherName, teacherPhone string
		if clsTeachers[int(item.Info.Grade)] != nil &&
			clsTeachers[int(item.Info.Grade)][int(item.Info.Class)] != nil {
			teacherName = clsTeachers[int(item.Info.Grade)][int(item.Info.Class)].Name
			teacherPhone = clsTeachers[int(item.Info.Grade)][int(item.Info.Class)].Code
		}

		gradeInfo.Classes = append(gradeInfo.Classes, &core_api.ClassInfo{
			Class:        item.Info.Class,
			UserNum:      uNum,
			AlarmNum:     aNum,
			TeacherName:  teacherName,
			TeacherPhone: teacherPhone,
		})
	}

	// 有序map转为有序切片
	grades := make([]*core_api.GradeInfo, 0, len(gradeMap))
	for _, grade := range gradeMap {
		grades = append(grades, grade)
	}
	// 确保排序
	sort.Slice(grades, func(i, j int) bool {
		return grades[i].Grade < grades[j].Grade
	})

	return grades
}

// DashboardListUsers 【用户管理】列出某班级学生
func (s *DashboardService) DashboardListUsers(ctx context.Context, req *core_api.DashboardListUsersReq) (*core_api.DashboardListUsersResp, error) {
	meta, role, err := s.AuthDomain.IdentifyRole(ctx, req.GetUnitId())
	if err != nil {
		return nil, err
	}

	unitOID, _ := bson.ObjectIDFromHex(req.UnitId)
	pUnit, err := s.UnitMapper.FindOneById(ctx, unitOID)
	if err != nil {
		logs.Errorf("get unit error: %s", errorx.ErrorWithoutStack(err))
		return nil, errorx.New(errno.ErrInvalidParams, errorx.KV("field", "UnitID"))
	}

	switch role {
	case enum.UserRoleUnitAdmin:
		return s.listUsers4Unit(ctx, unitOID, pUnit, req)
	case enum.UserRoleClassTeacher:
		return s.listUsers4ClsTch(ctx, meta.UserId, unitOID, pUnit, req)
	}
	return nil, errorx.New(errno.ErrInvalidRole)
}

// 单位端 - 列出用户
func (s *DashboardService) listUsers4Unit(ctx context.Context, unitOID bson.ObjectID, pUnit *unit.Unit, req *core_api.DashboardListUsersReq) (*core_api.DashboardListUsersResp, error) {
	// 创建搜索配置
	opts := &user.ListUserOptions{
		UnitID:     unitOID,
		StartGrade: pUnit.StartGrade,
		Grade:      req.Grade,
		Class:      req.Class,
		Level:      req.Level,
		Gender:     req.Gender,
		Keyword:    req.Keyword,
		Page:       req.PaginationOptions.GetPage(),
		Limit:      req.PaginationOptions.GetLimit(),
	}

	dbUsers, total, err := s.UserMapper.ListUsers(ctx, opts)
	if err != nil {
		return nil, errorx.New(errno.ErrUserNotFound)
	}

	pg := util.PaginationRes(int32(total), req.PaginationOptions)
	riskUsers, err := s.completeRiskUser(ctx, dbUsers, unitOID)

	return &core_api.DashboardListUsersResp{
		RiskUsers:  riskUsers,
		Pagination: pg,
	}, err
}

// 班主任端 - 列出所带班级的学生
func (s *DashboardService) listUsers4ClsTch(ctx context.Context, userId string, unitOID bson.ObjectID, pUnit *unit.Unit, req *core_api.DashboardListUsersReq) (*core_api.DashboardListUsersResp, error) {
	userOID, err := bson.ObjectIDFromHex(userId)
	if err != nil {
		return nil, errorx.New(errno.ErrInvalidParams, errorx.KV("field", "UserID"))
	}

	boundClasses, err := s.UserMapper.GetClassTeacherBoundClasses(ctx, userOID)
	if err != nil {
		logs.Errorf("get class teacher bound classes error: %s", errorx.ErrorWithoutStack(err))
		return nil, errorx.New(errno.ErrCountUserByClasses)
	}

	if len(boundClasses) == 0 {
		page := int64(1)
		if req.PaginationOptions.Page != nil {
			page = *req.PaginationOptions.Page
		}
		limit := int64(20)
		if req.PaginationOptions.Limit != nil {
			limit = *req.PaginationOptions.Limit
		}
		return &core_api.DashboardListUsersResp{
			RiskUsers: make([]*core_api.RiskUser, 0),
			Pagination: &basic.Pagination{
				Total:   0,
				Page:    page,
				Limit:   limit,
				HasNext: false,
			},
		}, nil
	}

	// 计算年级和班级列表
	grades := make([]int32, 0, len(boundClasses))
	classes := make([]int32, 0, len(boundClasses))
	for _, bc := range boundClasses {
		grade := util.CalculateGrade(pUnit.StartGrade, bc.EnrollYear)
		grades = append(grades, int32(grade))
		classes = append(classes, int32(bc.Class))
	}

	// 如果请求中有筛选条件，进一步过滤
	if req.Grade != nil || req.Class != nil {
		filteredGrades := make([]int32, 0)
		filteredClasses := make([]int32, 0)
		for i, g := range grades {
			if req.Grade != nil && g != *req.Grade {
				continue
			}
			if req.Class != nil && classes[i] != *req.Class {
				continue
			}
			filteredGrades = append(filteredGrades, g)
			filteredClasses = append(filteredClasses, classes[i])
		}
		grades = filteredGrades
		classes = filteredClasses
	}

	if len(grades) == 0 {
		page := int64(1)
		if req.PaginationOptions.Page != nil {
			page = *req.PaginationOptions.Page
		}
		limit := int64(20)
		if req.PaginationOptions.Limit != nil {
			limit = *req.PaginationOptions.Limit
		}
		return &core_api.DashboardListUsersResp{
			RiskUsers: make([]*core_api.RiskUser, 0),
			Pagination: &basic.Pagination{
				Total:   0,
				Page:    page,
				Limit:   limit,
				HasNext: false,
			},
		}, nil
	}

	opts := &user.ListUserOptions{
		UnitID:     unitOID,
		StartGrade: pUnit.StartGrade,
		Grades:     grades,
		Classes:    classes,
		Level:      req.Level,
		Gender:     req.Gender,
		Keyword:    req.Keyword,
		Page:       req.PaginationOptions.GetPage(),
		Limit:      req.PaginationOptions.GetLimit(),
	}

	dbUsers, total, err := s.UserMapper.ListUsers(ctx, opts)
	if err != nil {
		return nil, errorx.New(errno.ErrNotFound)
	}

	pg := util.PaginationRes(int32(total), req.PaginationOptions)
	riskUsers, err := s.completeRiskUser(ctx, dbUsers, unitOID)

	return &core_api.DashboardListUsersResp{
		RiskUsers:  riskUsers,
		Pagination: pg,
	}, err
}

func (s *DashboardService) completeRiskUser(ctx context.Context, dbUsers []*user.User, unitID bson.ObjectID) ([]*core_api.RiskUser, error) {
	if len(dbUsers) == 0 {
		return make([]*core_api.RiskUser, 0), nil
	}

	unitDAO, err := s.UnitMapper.FindOneById(ctx, unitID)
	if err != nil {
		return nil, errorx.WrapByCode(err, errno.ErrInternalError)
	}

	uids := make([]bson.ObjectID, len(dbUsers))
	for i, dbUser := range dbUsers {
		uids[i] = dbUser.ID
	}

	var msgStats map[bson.ObjectID]*conversation.ConvStats
	var keyWords map[bson.ObjectID][]string
	var msgErr, kwErr error

	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		msgStats, msgErr = s.ConversationMapper.BatchConvStats(ctx, uids)
		if msgErr != nil {
			logs.Warnf("查询对话统计失败: %v", errorx.ErrorWithoutStack(msgErr))
		}
	}()

	go func() {
		defer wg.Done()
		keyWords, kwErr = s.ReportMapper.BatchGetUserKeyWords(ctx, uids)
		if kwErr != nil {
			logs.Errorf("查询关键词失败: %v", errorx.ErrorWithoutStack(kwErr))
		}
	}()

	wg.Wait()

	if kwErr != nil {
		return nil, errorx.New(errno.ErrDashboardGetUserKeywords)
	}
	if msgErr != nil || msgStats == nil {
		return nil, errorx.New(errno.ErrDashboardGetUserConversationStatic)
	}

	riskUsers := make([]*core_api.RiskUser, len(dbUsers))
	for i, dbUser := range dbUsers {
		remark := &core_api.Remark{
			Time:    dbUser.Remark.CreateTime.Unix(),
			Content: dbUser.Remark.Content,
		}
		calculatedGrade := dbUser.CalculateGrade(unitDAO.StartGrade)
		riskUsers[i] = &core_api.RiskUser{
			User: &core_api.UserVO{
				Id:     dbUser.ID.Hex(),
				Code:   dbUser.Code,
				Name:   dbUser.Name,
				Gender: int32(dbUser.Gender),
				Grade:  int32(calculatedGrade),
				Class:  int32(dbUser.Class),
				Remark: remark,
			},
			Level:    int32(dbUser.RiskLevel),
			Keywords: make([]string, 0),
		}
		if msgStats[dbUser.ID] != nil {
			riskUsers[i].TotalConversationRounds = msgStats[dbUser.ID].Rounds
			riskUsers[i].LastConversationTime = msgStats[dbUser.ID].LatestTime
		}
		if keyWords[dbUser.ID] != nil {
			riskUsers[i].Keywords = keyWords[dbUser.ID]
		}
	}

	return riskUsers, nil
}

// DashboardCreateRemark 添加备注
func (s *DashboardService) DashboardCreateRemark(ctx context.Context, req *core_api.DashboardCreateRemarkReq) (*core_api.DashboardCreateRemarkResp, error) {
	meta, role, err := s.AuthDomain.IdentifyRole(ctx, req.GetUnitId())
	if err != nil {
		return nil, err
	}

	userOID, err := bson.ObjectIDFromHex(req.UserId)
	if err != nil {
		return nil, errorx.New(errno.ErrInvalidParams, errorx.KV("field", "UserID"), errorx.KV("value", "用户ID"))
	}
	u, err := s.UserMapper.FindOneById(ctx, userOID)
	if err != nil {
		logs.Errorf("find user by id error: %s", errorx.ErrorWithoutStack(err))
		return nil, err
	}

	switch role {
	case enum.UserRoleUnitAdmin:
		return s.remarkFromUnit(ctx, userOID, req)
	case enum.UserRoleClassTeacher:
		return s.remarkFromClsTch(ctx, meta.UserId, userOID, u, req)
	}
	return nil, errorx.New(errno.ErrInsufficientAuth)
}

func (s *DashboardService) remarkFromUnit(ctx context.Context, userOID bson.ObjectID, req *core_api.DashboardCreateRemarkReq) (*core_api.DashboardCreateRemarkResp, error) {
	update := bson.M{
		cst.Remark: &user.Remark{
			Content:    req.GetRemark(),
			CreateTime: time.Now(),
		},
	}
	if err := s.UserMapper.UpdateFields(ctx, userOID, update); err != nil {
		logs.Errorf("update user error: %s", errorx.ErrorWithoutStack(err))
		return nil, err
	}

	return &core_api.DashboardCreateRemarkResp{
		Code: 0,
		Msg:  "success",
	}, nil
}

func (s *DashboardService) remarkFromClsTch(ctx context.Context, teacherId string, userOID bson.ObjectID, targetUser *user.User, req *core_api.DashboardCreateRemarkReq) (*core_api.DashboardCreateRemarkResp, error) {
	// 校验归属
	teacherOID, _ := bson.ObjectIDFromHex(teacherId)
	authorized, err := s.isStudentInTeacherClasses(ctx, teacherOID, targetUser)
	if err != nil || !authorized {
		return nil, errorx.New(errno.ErrInsufficientAuth)
	}

	return s.remarkFromUnit(ctx, userOID, req)
}

// DashboardUserConvRecords 获取某用户对话记录
func (s *DashboardService) DashboardUserConvRecords(ctx context.Context, req *core_api.DashboardUserConvRecordsReq) (*core_api.DashboardUserConvRecordsResp, error) {
	userOID, err := bson.ObjectIDFromHex(req.UserId)
	if err != nil {
		return nil, errorx.New(errno.ErrInvalidParams, errorx.KV("field", "UserID"), errorx.KV("value", "用户ID"))
	}

	targetUser, err := s.UserMapper.FindOneById(ctx, userOID)
	if err != nil {
		logs.Errorf("get user info error: %s", errorx.ErrorWithoutStack(err))
		return nil, errorx.New(errno.ErrDashboardGetUserInfo)
	}

	meta, role, err := s.AuthDomain.IdentifyRole(ctx, targetUser.UnitID.Hex())
	if err != nil {
		return nil, err
	}

	switch role {
	case enum.UserRoleUnitAdmin:
		return s.dashboardUserConvRecordsUnit(ctx, userOID, targetUser, req)
	case enum.UserRoleClassTeacher:
		return s.dashboardUserConvRecordsClassTeacher(ctx, meta.UserId, userOID, targetUser, req)
	}
	return nil, errorx.New(errno.ErrInsufficientAuth)
}

func (s *DashboardService) dashboardUserConvRecordsUnit(ctx context.Context, userOID bson.ObjectID, targetUser *user.User, req *core_api.DashboardUserConvRecordsReq) (*core_api.DashboardUserConvRecordsResp, error) {
	// 获取用户对话频率趋势
	userConvTrend, err := s.getUserConvTrend(ctx, userOID)
	if err != nil {
		return nil, err
	}

	// 批量处理对话详情
	convDetail, pagination, err := s.listUserConvDetails(ctx, userOID, req.PaginationOptions)
	if err != nil {
		return nil, err
	}

	unitDAO, err := s.UnitMapper.FindOneById(ctx, targetUser.UnitID)
	if err != nil {
		return nil, errorx.WrapByCode(err, errno.ErrInternalError)
	}
	calculatedGrade := targetUser.CalculateGrade(unitDAO.StartGrade)

	resp := &core_api.DashboardUserConvRecordsResp{
		User: &core_api.UserVO{
			Id:     targetUser.ID.Hex(),
			Name:   targetUser.Name,
			Gender: int32(targetUser.Gender),
			Grade:  int32(calculatedGrade),
			Class:  int32(targetUser.Class),
		},
		UserConvTrend: userConvTrend,
		ConvDetail:    convDetail,
		Pagination:    pagination,
		Code:          0,
		Msg:           "success",
	}

	return resp, nil
}

func (s *DashboardService) dashboardUserConvRecordsClassTeacher(ctx context.Context, teacherId string, userOID bson.ObjectID, targetUser *user.User, req *core_api.DashboardUserConvRecordsReq) (*core_api.DashboardUserConvRecordsResp, error) {
	teacherOID, _ := bson.ObjectIDFromHex(teacherId)
	authorized, err := s.isStudentInTeacherClasses(ctx, teacherOID, targetUser)
	if err != nil || !authorized {
		return nil, errorx.New(errno.ErrInsufficientAuth)
	}

	return s.dashboardUserConvRecordsUnit(ctx, userOID, targetUser, req)
}

// getUserConvTrend 获取用户对话趋势数据
func (s *DashboardService) getUserConvTrend(ctx context.Context, userOID bson.ObjectID) (*core_api.UserConvTrend, error) {
	dailyStats, err := s.ConversationMapper.CountUserDailyConv(ctx, userOID)
	if err != nil {
		logs.Errorf("get user weekly conversation stats error: %s", errorx.ErrorWithoutStack(err))
		return nil, errorx.New(errno.ErrDashboardGetUserConversationStatic)
	}

	trendPoints := make([]*core_api.TrendPoint, 0, 7)
	for day := int32(1); day <= 7; day++ {
		count := dailyStats[day] // 如果没有数据，默认为0
		trendPoints = append(trendPoints, &core_api.TrendPoint{
			Week:  day,
			Count: count,
		})
	}

	return &core_api.UserConvTrend{
		TrendPoints: trendPoints,
	}, nil
}

// listUserConvDetails 用户对话详情列表（摘要&词云）
func (s *DashboardService) listUserConvDetails(ctx context.Context, userOID bson.ObjectID, paginationOpts *basic.PaginationOptions) ([]*core_api.ConvDetail, *basic.Pagination, error) {
	// 获取对话记录
	convs, pagination, err := s.getPagedUserConvs(ctx, userOID, paginationOpts)
	if err != nil {
		return nil, nil, err
	}
	if len(convs) == 0 {
		return make([]*core_api.ConvDetail, 0), pagination, nil
	}

	tm := make(map[bson.ObjectID]int64)                // 对话时间戳
	digests := make(map[bson.ObjectID]string)          // 摘要-取自ReportMapper
	kwds := make(map[bson.ObjectID]*core_api.Keywords) // 关键词-取自词云域WordCloudExtractor

	// To Optimize：初期用户对话数，即len(convs)较小，遍历时逐个查Report即可 后续可优化为批量查询Report
	// 对每条对话记录：1.调用ReportMapper获得摘要 2.调用HisDomain获取历史消息 3.调用词云域生成词云
	var wg sync.WaitGroup
	var tmMu, dgstMu, kwdsMu sync.Mutex
	wg.Add(len(convs))

	for _, conv := range convs {
		go func(c *conversation.Conversation) {
			defer wg.Done()
			// 每个routine处理一条对话记录
			// 填充时间
			tmMu.Lock()
			tm[c.ID] = c.StartTime.Unix()
			tmMu.Unlock()

			// 获取摘要
			rpt, err := s.ReportMapper.FindByConversationPreferSuccess(ctx, c.ID)
			if err != nil {
				// 报表不存在，可能还未完成创建
				if errors.Is(err, mongo.ErrNoDocuments) {
					dgstMu.Lock()
					digests[c.ID] = "暂无摘要"
					dgstMu.Unlock()
				} else {
					// 意外错误
					// 这里不直接返回 继续尝试生成词云
					logs.Errorf("get report error: %s", errorx.ErrorWithoutStack(err))
				}
			} else {
				// 报表存在，正常填入摘要
				dgstMu.Lock()
				digests[c.ID] = rpt.Digest
				dgstMu.Unlock()
			}
			// 获取所有对话历史消息
			msgHis, err := his.Mgr.RetrieveMessage(ctx, conv.ID.Hex(), -1)
			if err != nil || len(msgHis) == 0 {
				logs.Errorf("retrieve history messages error: %s", errorx.ErrorWithoutStack(err))
				kwdsMu.Lock()
				kwds[c.ID] = &core_api.Keywords{
					KeywordMap: make(map[string]int32),
					KeyTotal:   0,
				}
				kwdsMu.Unlock()
				return
			}
			// 生成词云
			wc, err := wordcld.Extractor.FromHisMsg(msgHis)
			if err != nil {
				logs.Errorf("word cloud extractor error: %s", errorx.ErrorWithoutStack(err))
				kwdsMu.Lock()
				kwds[c.ID] = &core_api.Keywords{
					KeywordMap: make(map[string]int32),
					KeyTotal:   0,
				}
				kwdsMu.Unlock()
				return
			}

			kwdsMu.Lock()
			kwds[c.ID] = wc
			kwdsMu.Unlock()
		}(conv)
	}

	wg.Wait()

	// 构造响应中的convDetails列表
	convDetails := make([]*core_api.ConvDetail, 0, len(convs))

	for convId, convTime := range tm {
		convDetail := &core_api.ConvDetail{
			ConversationId: convId.Hex(),
			Time:           convTime,
			Digest:         "",
			Keywords:       &core_api.Keywords{},
		}
		if dgst, ok := digests[convId]; ok {
			convDetail.Digest = dgst
		}
		if kwd, ok := kwds[convId]; ok {
			convDetail.Keywords = kwd
		}
		convDetails = append(convDetails, convDetail)
	}

	// 页内按照时间新-旧排序
	sort.Slice(convDetails, func(i, j int) bool {
		return convDetails[i].Time > convDetails[j].Time // 时间戳降序排序，即最新的在前
	})

	return convDetails, pagination, nil
}

// getPagedUserConvs 获取用户对话记录（分页）
// 返回分页范围内的Conversation和分页参数
func (s *DashboardService) getPagedUserConvs(ctx context.Context, userOID bson.ObjectID, paginationOpts *basic.PaginationOptions) ([]*conversation.Conversation, *basic.Pagination, error) {
	convs, err := s.ConversationMapper.FindAllByUserId(ctx, userOID) // 已按对话时间排序
	if err != nil {
		logs.Errorf("get user convs error: %s", errorx.ErrorWithoutStack(err))
		return nil, nil, errorx.New(errno.ErrDashboardGetConversations)
	}
	total := int32(len(convs))

	startIdx, endIdx := util.PagedIndex(total, paginationOpts)

	// 返回分页范围内的Conversation
	pagedConvs := convs[startIdx:endIdx]
	pagination := util.PaginationRes(total, paginationOpts)

	return pagedConvs, pagination, nil
}

func (s *DashboardService) DashboardGetReport(ctx context.Context, req *core_api.DashboardGetReportReq) (*core_api.DashboardGetReportResp, error) {
	convOID, err := bson.ObjectIDFromHex(req.ConversationId)
	if err != nil {
		return nil, errorx.New(errno.ErrInvalidParams, errorx.KV("field", "ConversationId"), errorx.KV("value", "对话ID"))
	}

	conv, err := s.ConversationMapper.FindOneById(ctx, convOID)
	if err != nil {
		logs.Errorf("get conversation error: %s", errorx.ErrorWithoutStack(err))
		return nil, errorx.New(errno.ErrNotFound, errorx.KV("field", "对话"))
	}
	usr, err := s.UserMapper.FindOneById(ctx, conv.UserID)
	if err != nil {
		logs.Errorf("get user error: %s", errorx.ErrorWithoutStack(err))
		return nil, errorx.New(errno.ErrNotFound, errorx.KV("field", "用户"))
	}

	meta, role, err := s.AuthDomain.IdentifyRole(ctx, usr.UnitID.Hex())
	if err != nil {
		return nil, err
	}

	switch role {
	case enum.UserRoleUnitAdmin:
		return s.dashboardGetReportUnit(ctx, convOID, req)
	case enum.UserRoleClassTeacher:
		return s.dashboardGetReportClassTeacher(ctx, meta.UserId, convOID, usr, req)
	}
	return nil, errorx.New(errno.ErrInsufficientAuth)
}

func (s *DashboardService) dashboardGetReportUnit(ctx context.Context, convOID bson.ObjectID, req *core_api.DashboardGetReportReq) (*core_api.DashboardGetReportResp, error) {
	rpt, err := s.ReportMapper.FindByConversationPreferSuccess(ctx, convOID)
	if err != nil {
		logs.Errorf("get report error: %s", errorx.ErrorWithoutStack(err))
		return nil, errorx.New(errno.ErrDashboardGetReport)
	}

	resp := &core_api.DashboardGetReportResp{
		ReportId:       rpt.ID.Hex(),
		Title:          rpt.Title,
		Topics:         rpt.Topics,
		Digest:         rpt.Digest,
		Emotion:        int32(rpt.Emotion),
		Body:           rpt.Body,
		Suggestions:    rpt.Suggestions,
		NeedAlarm:      rpt.NeedAlarm,
		KeywordPercent: rpt.Keywords,
		ReportStatus:   int32(rpt.Status),
		Code:           0,
		Msg:            "success",
	}

	if rpt.Status != enum.ReportStatusSuccess {
		resp.Code = errno.ErrReportNotReady
		resp.Msg = "报表处理中，请稍后"
	}

	return resp, nil
}

func (s *DashboardService) dashboardGetReportClassTeacher(ctx context.Context, teacherId string, convOID bson.ObjectID, targetUser *user.User, req *core_api.DashboardGetReportReq) (*core_api.DashboardGetReportResp, error) {
	teacherOID, _ := bson.ObjectIDFromHex(teacherId)
	authorized, err := s.isStudentInTeacherClasses(ctx, teacherOID, targetUser)
	if err != nil || !authorized {
		return nil, errorx.New(errno.ErrInsufficientAuth)
	}

	return s.dashboardGetReportUnit(ctx, convOID, req)
}

func (s *DashboardService) DashboardUnitConvRecords(ctx context.Context, req *core_api.DashboardUnitConvRecordsReq) (*core_api.DashboardUnitConvRecordsResp, error) {
	meta, role, err := s.AuthDomain.IdentifyRole(ctx, req.GetUnitId())
	if err != nil {
		return nil, err
	}

	switch role {
	case enum.UserRoleSuperAdmin:
		return s.getAllUnitsConvs(ctx, req)

	case enum.UserRoleUnitAdmin:
		return s.getOneUnitConvs(ctx, req)

	case enum.UserRoleClassTeacher:
		unitIdStr := req.GetUnitId()
		unitOID, err := bson.ObjectIDFromHex(unitIdStr)
		if err != nil {
			return nil, errorx.New(errno.ErrInvalidParams, errorx.KV("field", "UnitID"), errorx.KV("value", "单位 ID"))
		}
		pUnit, err := s.UnitMapper.FindOneById(ctx, unitOID)
		if err != nil {
			logs.Errorf("get unit error: %s", errorx.ErrorWithoutStack(err))
			return nil, errorx.New(errno.ErrInvalidParams, errorx.KV("field", "UnitID"))
		}
		if meta.UnitId != "" && meta.UnitId != unitIdStr {
			return nil, errorx.New(errno.ErrInsufficientAuth)
		}
		return s.getClassTeacherConvs(ctx, meta.UserId, unitOID, pUnit, req)
	}
	return nil, errorx.New(errno.ErrInsufficientAuth)
}

func (s *DashboardService) isStudentInTeacherClasses(ctx context.Context, teacherOID bson.ObjectID, targetUser *user.User) (bool, error) {
	// 获取该学生所属单位的配置，以计算年级
	pUnit, err := s.UnitMapper.FindOneById(ctx, targetUser.UnitID)
	if err != nil {
		logs.Errorf("get unit error: %s", errorx.ErrorWithoutStack(err))
		return false, err
	}

	boundClasses, err := s.UserMapper.GetClassTeacherBoundClasses(ctx, teacherOID)
	if err != nil {
		logs.Errorf("get class teacher bound classes error: %s", errorx.ErrorWithoutStack(err))
		return false, err
	}

	targetUserGrade := targetUser.CalculateGrade(pUnit.StartGrade)
	for _, bc := range boundClasses {
		grade := util.CalculateGrade(pUnit.StartGrade, bc.EnrollYear)
		if int32(grade) == int32(targetUserGrade) && bc.Class == targetUser.Class {
			return true, nil
		}
	}
	return false, nil
}

// req包含unitId
func (s *DashboardService) getOneUnitConvs(ctx context.Context, req *core_api.DashboardUnitConvRecordsReq) (*core_api.DashboardUnitConvRecordsResp, error) {
	unitOID, err := bson.ObjectIDFromHex(req.GetUnitId())
	if err != nil {
		return nil, errorx.New(errno.ErrInvalidParams, errorx.KV("field", "UnitId"), errorx.KV("value", "用户ID"))
	}

	total, err := s.ConversationMapper.CountByUnit(ctx, &unitOID)
	if err != nil {
		return nil, errorx.New(errno.ErrDashboardGetConversations)
	}

	pg := util.PaginationRes(total, req.PaginationOptions)

	// 若对话数为0
	if total == 0 {
		return &core_api.DashboardUnitConvRecordsResp{
			ConversationList: make([]*core_api.ConvOverview, 0),
			Pagination:       pg,
			Code:             0,
			Msg:              "success",
		}, nil
	}

	// 至少有1条对话
	convs, err := s.ConversationMapper.FindManyByUnitId(ctx, &unitOID, util.PagedFindOpt(req.PaginationOptions).SetSort(bson.D{{cst.EndTime, -1}}))
	if err != nil || len(convs) == 0 {
		logs.Errorf("get conversation error: %s", errorx.ErrorWithoutStack(err))
		return nil, errorx.New(errno.ErrNotFound, errorx.KV("field", "对话"))
	}

	// 提取 userId / convId 列表
	usrIds := make([]bson.ObjectID, 0, len(convs))
	convIds := make([]bson.ObjectID, 0, len(convs))
	for _, conv := range convs {
		usrIds = append(usrIds, conv.UserID)
		convIds = append(convIds, conv.ID)
	}

	// 批量查询用户信息
	users, err := s.UserMapper.BatchFindByIDs(ctx, usrIds)
	if err != nil {
		logs.Errorf("get user error: %s", errorx.ErrorWithoutStack(err))
		return nil, errorx.New(errno.ErrNotFound, errorx.KV("field", "用户"))
	}

	// 批量判断会话是否存在待处理预警
	needsAlarm, err := s.AlarmMapper.BatchExistsByConvId(ctx, convIds)
	if err != nil {
		logs.Errorf("batch check need alarm error: %s", errorx.ErrorWithoutStack(err))
		return nil, errorx.New(errno.ErrDashboardGetConversations)
	}

	unitDAO, err := s.UnitMapper.FindOneById(ctx, unitOID)
	if err != nil {
		return nil, errorx.WrapByCode(err, errno.ErrInternalError)
	}

	// 构建响应
	convOverviews := make([]*core_api.ConvOverview, 0, len(convs))
	for _, conv := range convs {
		usr := users[conv.UserID]
		if usr == nil {
			continue
		}
		calculatedGrade := usr.CalculateGrade(unitDAO.StartGrade)
		convOverviews = append(convOverviews, &core_api.ConvOverview{
			User: &core_api.UserVO{
				Id:     usr.ID.Hex(),
				Name:   usr.Name,
				Grade:  int32(calculatedGrade),
				Class:  int32(usr.Class),
				Code:   usr.Code,
				Gender: int32(usr.Gender),
			},
			ConvId:    conv.ID.Hex(),
			Title:     conv.Title,
			Time:      conv.EndTime.Unix(),
			NeedAlarm: needsAlarm[conv.ID],
		})
	}

	return &core_api.DashboardUnitConvRecordsResp{
		ConversationList: convOverviews,
		Pagination:       pg,
		Code:             0,
		Msg:              "success",
	}, nil

}

func (s *DashboardService) getAllUnitsConvs(ctx context.Context, req *core_api.DashboardUnitConvRecordsReq) (*core_api.DashboardUnitConvRecordsResp, error) {
	return nil, errorx.New(errno.UnImplementErr)
}

// 班主任端 - 获取所带班级学生的对话记录
func (s *DashboardService) getClassTeacherConvs(ctx context.Context, userId string, unitOID bson.ObjectID, pUnit *unit.Unit, req *core_api.DashboardUnitConvRecordsReq) (*core_api.DashboardUnitConvRecordsResp, error) {
	// 获取班主任绑定的班级
	userOID, err := bson.ObjectIDFromHex(userId)
	if err != nil {
		return nil, errorx.New(errno.ErrInvalidParams, errorx.KV("field", "UserID"))
	}

	boundClasses, err := s.UserMapper.GetClassTeacherBoundClasses(ctx, userOID)
	if err != nil {
		logs.Errorf("get class teacher bound classes error: %s", errorx.ErrorWithoutStack(err))
		return nil, errorx.New(errno.ErrCountUserByClasses)
	}

	if len(boundClasses) == 0 {
		return &core_api.DashboardUnitConvRecordsResp{
			ConversationList: make([]*core_api.ConvOverview, 0),
			Pagination:       &basic.Pagination{Total: 0},
			Code:             0,
			Msg:              "success",
		}, nil
	}

	// 计算年级和班级列表
	grades := make([]int32, 0, len(boundClasses))
	classes := make([]int32, 0, len(boundClasses))
	for _, bc := range boundClasses {
		grade := util.CalculateGrade(pUnit.StartGrade, bc.EnrollYear)
		grades = append(grades, int32(grade))
		classes = append(classes, int32(bc.Class))
	}

	users, err := s.UserMapper.FindManyByClassList(ctx, unitOID, grades, classes)
	if err != nil {
		logs.Errorf("get users by class list error: %s", errorx.ErrorWithoutStack(err))
		return nil, errorx.New(errno.ErrNotFound, errorx.KV("field", "用户"))
	}

	if len(users) == 0 {
		return &core_api.DashboardUnitConvRecordsResp{
			ConversationList: make([]*core_api.ConvOverview, 0),
			Pagination:       &basic.Pagination{Total: 0},
			Code:             0,
			Msg:              "success",
		}, nil
	}

	userIds := make([]bson.ObjectID, len(users))
	for i, u := range users {
		userIds[i] = u.ID
	}

	total, err := s.ConversationMapper.CountByUserIds(ctx, userIds)
	if err != nil {
		return nil, errorx.New(errno.ErrDashboardGetConversations)
	}

	pg := util.PaginationRes(total, req.PaginationOptions)

	// 若对话数为 0
	if total == 0 {
		return &core_api.DashboardUnitConvRecordsResp{
			ConversationList: make([]*core_api.ConvOverview, 0),
			Pagination:       pg,
			Code:             0,
			Msg:              "success",
		}, nil
	}

	// 查询对话列表
	convs, err := s.ConversationMapper.FindManyByUserIds(ctx, userIds, util.PagedFindOpt(req.PaginationOptions).SetSort(bson.D{{cst.EndTime, -1}}))
	if err != nil || len(convs) == 0 {
		logs.Errorf("get conversation error: %s", errorx.ErrorWithoutStack(err))
		return nil, errorx.New(errno.ErrNotFound, errorx.KV("field", "对话"))
	}

	// 提取 userId / convId 列表
	usrIds := make([]bson.ObjectID, 0, len(convs))
	convIds := make([]bson.ObjectID, 0, len(convs))
	for _, conv := range convs {
		usrIds = append(usrIds, conv.UserID)
		convIds = append(convIds, conv.ID)
	}

	// 批量查询用户信息
	userMap, err := s.UserMapper.BatchFindByIDs(ctx, usrIds)
	if err != nil {
		logs.Errorf("get user error: %s", errorx.ErrorWithoutStack(err))
		return nil, errorx.New(errno.ErrNotFound, errorx.KV("field", "用户"))
	}

	// 批量判断会话是否存在待处理预警
	needsAlarm, err := s.AlarmMapper.BatchExistsByConvId(ctx, convIds)
	if err != nil {
		logs.Errorf("batch check need alarm error: %s", errorx.ErrorWithoutStack(err))
		return nil, errorx.New(errno.ErrDashboardGetConversations)
	}

	// 构建响应
	convOverviews := make([]*core_api.ConvOverview, 0, len(convs))
	for _, conv := range convs {
		usr := userMap[conv.UserID]
		if usr == nil {
			continue
		}
		calculatedGrade := usr.CalculateGrade(pUnit.StartGrade)
		convOverviews = append(convOverviews, &core_api.ConvOverview{
			User: &core_api.UserVO{
				Id:     usr.ID.Hex(),
				Name:   usr.Name,
				Grade:  int32(calculatedGrade),
				Class:  int32(usr.Class),
				Code:   usr.Code,
				Gender: int32(usr.Gender),
			},
			ConvId:    conv.ID.Hex(),
			Title:     conv.Title,
			Time:      conv.EndTime.Unix(),
			NeedAlarm: needsAlarm[conv.ID],
		})
	}

	return &core_api.DashboardUnitConvRecordsResp{
		ConversationList: convOverviews,
		Pagination:       pg,
		Code:             0,
		Msg:              "success",
	}, nil
}
