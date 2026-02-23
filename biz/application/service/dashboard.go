package service

import (
	"context"
	"sort"
	"sync"
	"time"

	"github.com/google/wire"
	"github.com/xh-polaris/psych-core-api/biz/infra/mapper/conversation"
	"github.com/xh-polaris/psych-core-api/biz/infra/mapper/message"
	"github.com/xh-polaris/psych-core-api/biz/infra/mapper/unit"
	"github.com/xh-polaris/psych-core-api/biz/infra/mapper/user"
	"github.com/xh-polaris/psych-core-api/pkg/errorx"
	"github.com/xh-polaris/psych-core-api/pkg/logs"
	"github.com/xh-polaris/psych-core-api/types/errno"
	"github.com/xh-polaris/psych-idl/kitex_gen/basic"
	"github.com/xh-polaris/psych-idl/kitex_gen/core_api"
	"github.com/xh-polaris/psych-idl/kitex_gen/profile"
	"go.mongodb.org/mongo-driver/v2/bson"
)

type IDashboardService interface {
	DashboardGetDataOverview(ctx context.Context, req *core_api.DashboardGetDataOverviewReq) (*core_api.DashboardGetDataOverviewResp, error)
	DashboardGetDataTrend(ctx context.Context, req *core_api.DashboardGetDataTrendReq) (*core_api.DashboardGetDataTrendResp, error)
	DashboardListUnits(ctx context.Context, req *core_api.DashboardListUnitsReq) (*core_api.DashboardListUnitsResp, error)
	DashboardGetPsychTrend(ctx context.Context, req *core_api.DashboardGetPsychTrendReq) (*core_api.DashboardGetPsychTrendResp, error)
	DashboardListClasses(ctx context.Context, req *core_api.DashboardListClassesReq) (*core_api.DashboardListClassesResp, error)
	DashboardListUsers(ctx context.Context, req *core_api.DashboardListUsersReq) (*core_api.DashboardListUsersResp, error)
}

type DashboardService struct {
	UserMapper         user.IMongoMapper
	UnitMapper         unit.IMongoMapper
	MessageMapper      message.MongoMapper
	ConversationMapper conversation.IMongoMapper
}

var DashboardServiceSet = wire.NewSet(
	wire.Struct(new(DashboardService), "*"),
	wire.Bind(new(IDashboardService), new(*DashboardService)),
)

func (s *DashboardService) DashboardGetDataOverview(ctx context.Context, req *core_api.DashboardGetDataOverviewReq) (*core_api.DashboardGetDataOverviewResp, error) {
	now := time.Now()
	weekBefore := now.AddDate(0, 0, -7)
	twoWeeksBefore := now.AddDate(0, 0, -14)

	// 区分管理端 / 单位端
	if req.UnitId == nil || req.GetUnitId() == "" {
		return s.dashboardOverviewAdmin(ctx, twoWeeksBefore, weekBefore, now)
	}

	unitOID, err := bson.ObjectIDFromHex(req.GetUnitId())
	if err != nil {
		return nil, errorx.New(errno.ErrInvalidParams, errorx.KV("field", "UnitID"), errorx.KV("value", "单位ID"))
	}
	return s.dashboardOverviewUnit(ctx, unitOID, twoWeeksBefore, weekBefore, now)
}

// 管理端数据概览
func (s *DashboardService) dashboardOverviewAdmin(ctx context.Context, twoWeeksBefore, weekBefore, now time.Time) (*core_api.DashboardGetDataOverviewResp, error) {
	// 单位数量（累计）
	totalUnits, err := s.UnitMapper.Count(ctx)
	if err != nil {
		logs.Errorf("count unit error: %s", errorx.ErrorWithoutStack(err))
		return nil, errorx.WrapByCode(err, errno.ErrUnitCount)
	}
	beforeUnits, err := s.UnitMapper.CountByPeriod(ctx, time.Time{}, weekBefore)
	if err != nil {
		logs.Errorf("count unit by period error: %s", errorx.ErrorWithoutStack(err))
		return nil, errorx.WrapByCode(err, errno.ErrUnitCount)
	}
	weeklyIncreaseUnits := totalUnits - beforeUnits
	var weeklyIncreaseUnitsRate float64
	if beforeUnits > 0 {
		weeklyIncreaseUnitsRate = float64(weeklyIncreaseUnits) / float64(beforeUnits)
	}

	// 用户数量（累计）
	totalUsers, err := s.UserMapper.Count(ctx)
	if err != nil {
		logs.Errorf("count user error: %s", errorx.ErrorWithoutStack(err))
		return nil, errorx.WrapByCode(err, errno.ErrUserCount)
	}
	beforeUsers, err := s.UserMapper.CountByPeriod(ctx, time.Time{}, weekBefore)
	if err != nil {
		logs.Errorf("count user by period error: %s", errorx.ErrorWithoutStack(err))
		return nil, errorx.WrapByCode(err, errno.ErrUserCount)
	}
	weeklyIncreaseUsers := totalUsers - beforeUsers
	var weeklyIncreaseUsersRate float64
	if beforeUsers > 0 {
		weeklyIncreaseUsersRate = float64(weeklyIncreaseUsers) / float64(beforeUsers)
	}

	// 活跃用户（过去 7 天内有对话的用户数）
	activeThisWeek, err := s.ConversationMapper.CountActiveUsers(ctx, nil, weekBefore, now)
	if err != nil {
		logs.Errorf("count active users error: %s", errorx.ErrorWithoutStack(err))
		return nil, errorx.WrapByCode(err, errno.ErrUserCount)
	}
	activeLastWeek, err := s.ConversationMapper.CountActiveUsers(ctx, nil, twoWeeksBefore, weekBefore)
	if err != nil {
		logs.Errorf("count active users last week error: %s", errorx.ErrorWithoutStack(err))
		return nil, errorx.WrapByCode(err, errno.ErrUserCount)
	}
	weeklyIncreaseActiveUsers := activeThisWeek - activeLastWeek
	var weeklyIncreaseActiveUsersRate float64
	if activeLastWeek > 0 {
		weeklyIncreaseActiveUsersRate = float64(weeklyIncreaseActiveUsers) / float64(activeLastWeek)
	}

	// 对话数量（总对话数 + 本周/上周新增）
	totalConversations, err := s.ConversationMapper.Count(ctx, nil)
	if err != nil {
		logs.Errorf("count conversations error: %s", errorx.ErrorWithoutStack(err))
		return nil, errorx.New(errno.ErrInternalError)
	}
	conversationsThisWeek, err := s.ConversationMapper.CountByPeriod(ctx, nil, weekBefore, now)
	if err != nil {
		logs.Errorf("count conversations this week error: %s", errorx.ErrorWithoutStack(err))
		return nil, errorx.New(errno.ErrInternalError)
	}
	conversationsLastWeek, err := s.ConversationMapper.CountByPeriod(ctx, nil, twoWeeksBefore, weekBefore)
	if err != nil {
		logs.Errorf("count conversations last week error: %s", errorx.ErrorWithoutStack(err))
		return nil, errorx.New(errno.ErrInternalError)
	}
	weeklyIncreaseConversations := conversationsThisWeek - conversationsLastWeek
	var weeklyIncreaseConversationsRate float64
	if conversationsLastWeek > 0 {
		weeklyIncreaseConversationsRate = float64(weeklyIncreaseConversations) / float64(conversationsLastWeek)
	}

	// 平均单次对话时长（分钟）：本周 vs 上周
	avgThisWeek, err := s.ConversationMapper.AverageDurationByPeriod(ctx, nil, weekBefore, now)
	if err != nil {
		logs.Errorf("avg duration this week error: %s", errorx.ErrorWithoutStack(err))
		return nil, errorx.New(errno.ErrInternalError)
	}
	avgLastWeek, err := s.ConversationMapper.AverageDurationByPeriod(ctx, nil, twoWeeksBefore, weekBefore)
	if err != nil {
		logs.Errorf("avg duration last week error: %s", errorx.ErrorWithoutStack(err))
		return nil, errorx.New(errno.ErrInternalError)
	}
	weeklyIncreaseAvgDuration := avgThisWeek - avgLastWeek
	var weeklyIncreaseAvgDurationRate float64
	if avgLastWeek > 0 {
		weeklyIncreaseAvgDurationRate = weeklyIncreaseAvgDuration / avgLastWeek
	}

	// 高风险用户数（riskLevel == high），暂不做周环比（无时间维度）
	alarmUsers, err := s.UserMapper.CountAlarmUsers(ctx, nil)
	if err != nil {
		logs.Errorf("count alarm users error: %s", errorx.ErrorWithoutStack(err))
		return nil, errorx.WrapByCode(err, errno.ErrUserCount)
	}

	return &core_api.DashboardGetDataOverviewResp{
		TotalUnits:                                   &totalUnits,
		WeeklyIncreaseUnits:                          &weeklyIncreaseUnits,
		WeeklyIncreaseUnitsRate:                      &weeklyIncreaseUnitsRate,
		TotalUsers:                                   totalUsers,
		WeeklyIncreaseUsers:                          weeklyIncreaseUsers,
		WeeklyIncreaseUsersRate:                      weeklyIncreaseUsersRate,
		ActiveUsers:                                  &activeThisWeek,
		WeeklyIncreaseActiveUsers:                    &weeklyIncreaseActiveUsers,
		WeeklyIncreaseActiveUsersRate:                &weeklyIncreaseActiveUsersRate,
		TotalConversations:                           totalConversations,
		WeeklyIncreaseConversations:                  weeklyIncreaseConversations,
		WeeklyIncreaseConversationsRate:              weeklyIncreaseConversationsRate,
		AverageTimePerConversation:                   avgThisWeek,
		WeeklyIncreaseAverageTimePerConversation:     weeklyIncreaseAvgDuration,
		WeeklyIncreaseAverageTimePerConversationRate: weeklyIncreaseAvgDurationRate,
		AlarmUsers:                                   alarmUsers,
		WeeklyIncreaseAlarmUsers:                     0,
		WeeklyIncreaseAlarmUsersRate:                 0,
		Code:                                         0,
		Msg:                                          "success",
	}, nil
}

// 单位端数据概览
func (s *DashboardService) dashboardOverviewUnit(ctx context.Context, unitOID bson.ObjectID, twoWeeksBefore, weekBefore, now time.Time) (*core_api.DashboardGetDataOverviewResp, error) {
	// 学生总数（当前单位）
	totalUsers, err := s.UserMapper.CountByUnitID(ctx, unitOID)
	if err != nil {
		logs.Errorf("count unit users error: %s", errorx.ErrorWithoutStack(err))
		return nil, errorx.WrapByCode(err, errno.ErrUserCount)
	}
	beforeUsers, err := s.UserMapper.CountByUnitIDAndPeriod(ctx, unitOID, time.Time{}, weekBefore)
	if err != nil {
		logs.Errorf("count unit users by period error: %s", errorx.ErrorWithoutStack(err))
		return nil, errorx.WrapByCode(err, errno.ErrUserCount)
	}
	weeklyIncreaseUsers := totalUsers - beforeUsers
	var weeklyIncreaseUsersRate float64
	if beforeUsers > 0 {
		weeklyIncreaseUsersRate = float64(weeklyIncreaseUsers) / float64(beforeUsers)
	}

	// 活跃用户（当前单位，过去 7 天）
	activeThisWeek, err := s.ConversationMapper.CountActiveUsers(ctx, &unitOID, weekBefore, now)
	if err != nil {
		logs.Errorf("count unit active users error: %s", errorx.ErrorWithoutStack(err))
		return nil, errorx.WrapByCode(err, errno.ErrUserCount)
	}
	activeLastWeek, err := s.ConversationMapper.CountActiveUsers(ctx, &unitOID, twoWeeksBefore, weekBefore)
	if err != nil {
		logs.Errorf("count unit active users last week error: %s", errorx.ErrorWithoutStack(err))
		return nil, errorx.WrapByCode(err, errno.ErrUserCount)
	}
	weeklyIncreaseActiveUsers := activeThisWeek - activeLastWeek
	var weeklyIncreaseActiveUsersRate float64
	if activeLastWeek > 0 {
		weeklyIncreaseActiveUsersRate = float64(weeklyIncreaseActiveUsers) / float64(activeLastWeek)
	}

	// 对话数量（当前单位）
	totalConversations, err := s.ConversationMapper.Count(ctx, &unitOID)
	if err != nil {
		logs.Errorf("count unit conversations error: %s", errorx.ErrorWithoutStack(err))
		return nil, errorx.New(errno.ErrInternalError)
	}
	conversationsThisWeek, err := s.ConversationMapper.CountByPeriod(ctx, &unitOID, weekBefore, now)
	if err != nil {
		logs.Errorf("count unit conversations this week error: %s", errorx.ErrorWithoutStack(err))
		return nil, errorx.New(errno.ErrInternalError)
	}
	conversationsLastWeek, err := s.ConversationMapper.CountByPeriod(ctx, &unitOID, twoWeeksBefore, weekBefore)
	if err != nil {
		logs.Errorf("count unit conversations last week error: %s", errorx.ErrorWithoutStack(err))
		return nil, errorx.New(errno.ErrInternalError)
	}
	weeklyIncreaseConversations := conversationsThisWeek - conversationsLastWeek
	var weeklyIncreaseConversationsRate float64
	if conversationsLastWeek > 0 {
		weeklyIncreaseConversationsRate = float64(weeklyIncreaseConversations) / float64(conversationsLastWeek)
	}

	// 平均单次对话时长（当前单位，本周 vs 上周）
	avgThisWeek, err := s.ConversationMapper.AverageDurationByPeriod(ctx, &unitOID, weekBefore, now)
	if err != nil {
		logs.Errorf("unit avg duration this week error: %s", errorx.ErrorWithoutStack(err))
		return nil, errorx.New(errno.ErrInternalError)
	}
	avgLastWeek, err := s.ConversationMapper.AverageDurationByPeriod(ctx, &unitOID, twoWeeksBefore, weekBefore)
	if err != nil {
		logs.Errorf("unit avg duration last week error: %s", errorx.ErrorWithoutStack(err))
		return nil, errorx.New(errno.ErrInternalError)
	}
	weeklyIncreaseAvgDuration := avgThisWeek - avgLastWeek
	var weeklyIncreaseAvgDurationRate float64
	if avgLastWeek > 0 {
		weeklyIncreaseAvgDurationRate = weeklyIncreaseAvgDuration / avgLastWeek
	}

	// 高风险用户数（当前单位）
	alarmUsers, err := s.UserMapper.CountAlarmUsers(ctx, &unitOID)
	if err != nil {
		logs.Errorf("count unit alarm users error: %s", errorx.ErrorWithoutStack(err))
		return nil, errorx.WrapByCode(err, errno.ErrUserCount)
	}

	return &core_api.DashboardGetDataOverviewResp{
		TotalUsers:                                   totalUsers,
		WeeklyIncreaseUsers:                          weeklyIncreaseUsers,
		WeeklyIncreaseUsersRate:                      weeklyIncreaseUsersRate,
		ActiveUsers:                                  &activeThisWeek,
		WeeklyIncreaseActiveUsers:                    &weeklyIncreaseActiveUsers,
		WeeklyIncreaseActiveUsersRate:                &weeklyIncreaseActiveUsersRate,
		TotalConversations:                           totalConversations,
		WeeklyIncreaseConversations:                  weeklyIncreaseConversations,
		WeeklyIncreaseConversationsRate:              weeklyIncreaseConversationsRate,
		AverageTimePerConversation:                   avgThisWeek,
		WeeklyIncreaseAverageTimePerConversation:     weeklyIncreaseAvgDuration,
		WeeklyIncreaseAverageTimePerConversationRate: weeklyIncreaseAvgDurationRate,
		AlarmUsers:                                   alarmUsers,
		WeeklyIncreaseAlarmUsers:                     0,
		WeeklyIncreaseAlarmUsersRate:                 0,
		Code:                                         0,
		Msg:                                          "success",
	}, nil
}

func (s *DashboardService) DashboardGetDataTrend(ctx context.Context, req *core_api.DashboardGetDataTrendReq) (*core_api.DashboardGetDataTrendResp, error) {
	return nil, errorx.New(errno.ErrUnImplement)
}

func (s *DashboardService) DashboardListUnits(ctx context.Context, req *core_api.DashboardListUnitsReq) (*core_api.DashboardListUnitsResp, error) {
	return nil, errorx.New(errno.ErrUnImplement)
}

func (s *DashboardService) DashboardGetPsychTrend(ctx context.Context, req *core_api.DashboardGetPsychTrendReq) (*core_api.DashboardGetPsychTrendResp, error) {
	return nil, errorx.New(errno.ErrUnImplement)
}

func (s *DashboardService) DashboardListClasses(ctx context.Context, req *core_api.DashboardListClassesReq) (*core_api.DashboardListClassesResp, error) {
	unitOID, err := bson.ObjectIDFromHex(req.UnitId)
	if err != nil {
		return nil, errorx.New(errno.ErrInvalidParams, errorx.KV("field", "UnitID"), errorx.KV("value", "单位ID"))
	}
	// 筛选参数
	var grades, classes []int32
	if req.Grade != nil {
		grades = append(grades, *req.Grade)
	}
	if req.Class != nil {
		classes = append(classes, *req.Class)
	}

	// 查询结果
	res, err := s.UserMapper.CountByClasses(ctx, unitOID, grades, classes)
	if err != nil {
		return nil, errorx.New(errno.ErrCountUserByClasses)
	}

	// 整理结果，构建响应
	return &core_api.DashboardListClassesResp{
		Grades: aggregateAndSort(res),
	}, nil
}

func aggregateAndSort(mapperRes []*user.ClassStatResult) []*core_api.GradeInfo {
	if len(mapperRes) == 0 {
		return make([]*core_api.GradeInfo, 0)
	}

	gradeMap := make(map[int32]*core_api.GradeInfo)
	// 将入参切片（有序）填充入有序map
	for _, item := range mapperRes {
		gradeInfo, exists := gradeMap[item.Grade]
		if !exists {
			gradeInfo = &core_api.GradeInfo{
				Grade:   item.Grade,
				Classes: make([]*core_api.ClassInfo, 0),
			}
			gradeMap[item.Grade] = gradeInfo
		}

		gradeInfo.Classes = append(gradeInfo.Classes, &core_api.ClassInfo{
			Class:        item.Class,
			UserNum:      item.UserNum,
			AlarmNum:     item.AlarmNum,
			TeacherName:  "",
			TeacherPhone: "",
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

func (s *DashboardService) DashboardListUsers(ctx context.Context, req *core_api.DashboardListUsersReq) (*core_api.DashboardListUsersResp, error) {
	unitOID, err := bson.ObjectIDFromHex(req.UnitId)
	if err != nil {
		return nil, errorx.New(errno.ErrInvalidParams, errorx.KV("field", "UnitID"), errorx.KV("value", "单位ID"))
	}
	// 查找所有用户并按风险高→低排序
	dbUsers, err := s.UserMapper.FindAllByUnitID(ctx, unitOID)
	if err != nil {
		return nil, errorx.New(errno.ErrNotFound)
	}
	sort.Slice(dbUsers, func(i, j int) bool {
		return dbUsers[i].RiskLevel < dbUsers[j].RiskLevel
	})

	// 分页结果
	total := int64(len(dbUsers))
	pg := &basic.Pagination{
		Total:   total,
		Page:    req.PaginationOptions.GetPage(),
		Limit:   req.PaginationOptions.GetLimit(),
		HasNext: req.PaginationOptions.GetPage()*req.PaginationOptions.GetLimit() < total,
	}

	// 补全响应中的riskUser
	riskUsers, err2 := s.completeRiskUser(ctx, pg, dbUsers)

	// 返回响应
	return &core_api.DashboardListUsersResp{
		RiskUsers:  riskUsers,
		Pagination: pg,
	}, err2

}

func (s *DashboardService) completeRiskUser(ctx context.Context, pg *basic.Pagination, dbUsers []*user.User) ([]*core_api.RiskUser, error) {
	if pg.Total == 0 {
		return make([]*core_api.RiskUser, 0), nil
	}

	start := (pg.GetPage() - 1) * pg.GetLimit()
	end := min(start+pg.GetLimit()-1, pg.Total-1)
	if start > end || end > pg.Total-1 {
		return make([]*core_api.RiskUser, 0), errorx.New(errno.ErrInternalError)
	}

	// 提取分页所需的dbUser切片和uid切片
	targetUsers := dbUsers[start : end+1]
	uids := make([]bson.ObjectID, end-start+1)
	for i, dbUser := range targetUsers {
		uids[i] = dbUser.ID
	}

	// 补全dbUser相关信息
	var msgStats map[bson.ObjectID]*message.MsgStats
	var msgErr, kwErr error

	var wg sync.WaitGroup
	wg.Add(2)

	// 获取对话统计信息
	go func() {
		defer wg.Done()
		msgStats, msgErr = s.MessageMapper.BatchMessageStats(ctx, uids)
		if msgErr != nil {
			logs.Warnf("查询对话统计失败: %v", errorx.ErrorWithoutStack(msgErr))
		}
	}()

	// 获取keywords
	go func() {
		defer wg.Done()
		// TODO 调用Post服务，获得所有用户的关键词
		if kwErr != nil {
			logs.Warnf("查询Post信息失败: %v", errorx.ErrorWithoutStack(kwErr))
		}
	}()

	wg.Wait()

	if kwErr != nil {
		return nil, errorx.New(errno.ErrGetUserKeywords)
	}
	if msgErr != nil || msgStats == nil {
		return nil, errorx.New(errno.ErrGetUserConversationStatic)
	}

	// 构建响应列表
	riskUsers := make([]*core_api.RiskUser, end-start+1)
	for i, dbUser := range targetUsers {
		riskUsers[i] = &core_api.RiskUser{
			User: &profile.User{
				Code:  dbUser.Code,
				Name:  dbUser.Name,
				Grade: dbUser.Grade,
				Class: dbUser.Class,
			},
			Level:    int32(dbUser.RiskLevel),
			Keywords: make([]string, 0),
		}
		if msgStats[dbUser.ID] != nil {
			riskUsers[i].TotalConversationRounds = msgStats[dbUser.ID].Rounds
			riskUsers[i].LastConversationTime = msgStats[dbUser.ID].LatestTime
		}
	}

	return riskUsers, nil
}
