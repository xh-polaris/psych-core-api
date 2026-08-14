package dashboard

import (
	"context"
	"time"

	"github.com/xh-polaris/psych-core-api/biz/application/dto/basic"
	"github.com/xh-polaris/psych-core-api/biz/application/dto/core_api"
	"github.com/xh-polaris/psych-core-api/biz/cst"
	"github.com/xh-polaris/psych-core-api/biz/domain/wordcld"
	"github.com/xh-polaris/psych-core-api/biz/infra/mapper/user"
	"github.com/xh-polaris/psych-core-api/biz/infra/util"
	"github.com/xh-polaris/psych-core-api/pkg/errorx"
	"github.com/xh-polaris/psych-core-api/pkg/logs"
	"github.com/xh-polaris/psych-core-api/types/enum"
	"github.com/xh-polaris/psych-core-api/types/errno"
	"go.mongodb.org/mongo-driver/v2/bson"
)

// boundClassesToGradesClasses 将班主任绑定的班级（入学年份+班级号）换算为年级+班级列表
func boundClassesToGradesClasses(boundClasses []user.ClassInfo, startGrade int) ([]int32, []int32) {
	grades := make([]int32, 0, len(boundClasses))
	classes := make([]int32, 0, len(boundClasses))
	for _, bc := range boundClasses {
		grade := util.CalculateGrade(startGrade, bc.EnrollYear)
		grades = append(grades, int32(grade))
		classes = append(classes, int32(bc.Class))
	}
	return grades, classes
}

// GetDataOverview4ClsTch 班主任版数据概览
func (d *DashboardDomain) GetDataOverview4ClsTch(ctx context.Context, userId string, unitOID bson.ObjectID, start, end time.Time) (*core_api.DashboardGetDataOverviewResp, error) {
	pUnit, err := d.UnitMapper.FindOneById(ctx, unitOID)
	if err != nil {
		return nil, errorx.New(errno.ErrInvalidParams, errorx.KV("field", "UnitID"))
	}

	// 获取班主任绑定的班级
	userOID, err := bson.ObjectIDFromHex(userId)
	if err != nil {
		return nil, errorx.New(errno.ErrInvalidParams, errorx.KV("field", "UserID"))
	}

	boundClasses, err := d.UserMapper.GetClassTeacherBoundClasses(ctx, userOID)
	if err != nil {
		logs.Errorf("get class teacher bound classes error: %s", errorx.ErrorWithoutStack(err))
		return nil, errorx.New(errno.ErrDashboardTotalUserStat)
	}

	if len(boundClasses) == 0 {
		return nil, errorx.New(errno.ErrNoBoundedClass, errorx.KV("id", userId), errorx.KV("role", enum.UserRoleI2S[enum.UserRoleClassTeacher]))
	}

	// 根据 EnrollYear 计算年级
	grades, classes := boundClassesToGradesClasses(boundClasses, pUnit.StartGrade)

	// 学生数统计（增长：end 时快照对比 start 时快照）
	totalUsers, err := d.UserMapper.CountStudentsByClassList(ctx, unitOID, grades, classes)
	if err != nil {
		logs.Errorf("count students by class list error: %s", errorx.ErrorWithoutStack(err))
		return nil, errorx.New(errno.ErrDashboardTotalUserStat)
	}
	endUsers, err := d.UserMapper.CountStudentsByPeriodAndClassList(ctx, &unitOID, grades, classes, time.Time{}, end)
	if err != nil {
		logs.Errorf("count students by period and class list error: %s", errorx.ErrorWithoutStack(err))
		return nil, errorx.New(errno.ErrDashboardTotalUserStat)
	}
	startUsers, err := d.UserMapper.CountStudentsByPeriodAndClassList(ctx, &unitOID, grades, classes, time.Time{}, start)
	if err != nil {
		logs.Errorf("count students by period and class list error: %s", errorx.ErrorWithoutStack(err))
		return nil, errorx.New(errno.ErrDashboardTotalUserStat)
	}

	usersWow := util.Wow{Cur: endUsers, Prev: startUsers}

	// 活跃用户统计（增长：截止 end 的新增活跃数）
	totalActive, err := d.ConversationMapper.CountActiveUsersByClassList(ctx, grades, classes, start, end)
	if err != nil {
		logs.Errorf("count active users by class list error: %s", errorx.ErrorWithoutStack(err))
		return nil, errorx.WrapByCode(err, errno.ErrDashboardActiveUserStat)
	}

	endActive, err := d.ConversationMapper.CountActiveUsersByClassList(ctx, grades, classes, time.Time{}, end)
	if err != nil {
		logs.Errorf("count active users by class list error: %s", errorx.ErrorWithoutStack(err))
		return nil, errorx.WrapByCode(err, errno.ErrDashboardActiveUserStat)
	}

	startActive, err := d.ConversationMapper.CountActiveUsersByClassList(ctx, grades, classes, time.Time{}, start)
	if err != nil {
		logs.Errorf("count active users by class list error: %s", errorx.ErrorWithoutStack(err))
		return nil, errorx.WrapByCode(err, errno.ErrDashboardActiveUserStat)
	}

	activeWow := util.Wow{Cur: endActive, Prev: startActive}

	// 对话统计（增长：截止 end 的对话数对比截止 start，TotalConversations 即截止 end 的累计对话数）
	endConv, err := d.ConversationMapper.CountConversationsByClassList(ctx, grades, classes, time.Time{}, end)
	if err != nil {
		logs.Errorf("count conversations by class list error: %s", errorx.ErrorWithoutStack(err))
		return nil, errorx.WrapByCode(err, errno.ErrDashboardConversationStat)
	}

	startConv, err := d.ConversationMapper.CountConversationsByClassList(ctx, grades, classes, time.Time{}, start)
	if err != nil {
		logs.Errorf("count conversations by class list error: %s", errorx.ErrorWithoutStack(err))
		return nil, errorx.WrapByCode(err, errno.ErrDashboardConversationStat)
	}

	convWow := util.Wow{Cur: endConv, Prev: startConv}

	// 平均对话时长（窗口均值对比截止 start 的累计均值）
	avgThisWeek, err := d.ConversationMapper.AverageDurationByClassListAndPeriod(ctx, grades, classes, start, end)
	if err != nil {
		logs.Errorf("avg duration this week by class list error: %s", errorx.ErrorWithoutStack(err))
		avgThisWeek = 0
	}

	avgPrev, err := d.ConversationMapper.AverageDurationByClassListAndPeriod(ctx, grades, classes, time.Time{}, start)
	if err != nil {
		logs.Errorf("avg duration before start by class list error: %s", errorx.ErrorWithoutStack(err))
		avgPrev = 0
	}

	avgThisWeek = util.Round2(avgThisWeek)
	avgPrev = util.Round2(avgPrev)
	avgIncre := util.Round2(avgThisWeek - avgPrev)

	// 高风险学生数（存在 alarm 记录的当前高危学生，增长：截止 end 对比截止 start）
	alarmUsersTotal, err := d.AlarmMapper.CountAlarmUsersByClassList(ctx, unitOID, grades, classes, time.Time{}, end)
	if err != nil {
		logs.Errorf("count alarm users by class list error: %s", errorx.ErrorWithoutStack(err))
		alarmUsersTotal = 0
	}

	alarmUsersStart, err := d.AlarmMapper.CountAlarmUsersByClassList(ctx, unitOID, grades, classes, time.Time{}, start)
	if err != nil {
		logs.Errorf("count alarm users by class list error: %s", errorx.ErrorWithoutStack(err))
		alarmUsersStart = 0
	}

	alarmWow := util.Wow{Cur: alarmUsersTotal, Prev: alarmUsersStart}

	return &core_api.DashboardGetDataOverviewResp{
		TotalUsers:                                   totalUsers,
		WeeklyIncreaseUsers:                          usersWow.Inc(),
		WeeklyIncreaseUsersRate:                      usersWow.Rate(),
		ActiveUsers:                                  util.Int32Ptr(totalActive),
		WeeklyIncreaseActiveUsers:                    util.Int32Ptr(activeWow.Inc()),
		WeeklyIncreaseActiveUsersRate:                util.Float64Ptr(activeWow.Rate()),
		TotalConversations:                           endConv,
		WeeklyIncreaseConversations:                  convWow.Inc(),
		WeeklyIncreaseConversationsRate:              convWow.Rate(),
		AverageTimePerConversation:                   avgThisWeek,
		WeeklyIncreaseAverageTimePerConversation:     avgIncre,
		WeeklyIncreaseAverageTimePerConversationRate: util.RateF(avgThisWeek, avgPrev),
		AlarmUsers:                                   alarmUsersTotal,
		WeeklyIncreaseAlarmUsers:                     alarmWow.Inc(),
		WeeklyIncreaseAlarmUsersRate:                 alarmWow.Rate(),
		Code:                                         0,
		Msg:                                          "success",
	}, nil
}

// GetDataTrend4ClsTch 班主任版数据趋势
func (d *DashboardDomain) GetDataTrend4ClsTch(ctx context.Context, userId string, unitOID bson.ObjectID, start, end time.Time) (*core_api.DashboardGetDataTrendResp, error) {
	pUnit, err := d.UnitMapper.FindOneById(ctx, unitOID)
	if err != nil {
		return nil, errorx.New(errno.ErrInvalidParams, errorx.KV("field", "UnitID"))
	}

	userOID, err := bson.ObjectIDFromHex(userId)
	if err != nil {
		return nil, errorx.New(errno.ErrInvalidParams, errorx.KV("field", "UserID"))
	}

	boundClasses, err := d.UserMapper.GetClassTeacherBoundClasses(ctx, userOID)
	if err != nil {
		logs.Errorf("get class teacher bound classes error: %s", errorx.ErrorWithoutStack(err))
		return nil, errorx.New(errno.ErrDashboardActiveUserStat)
	}

	if len(boundClasses) == 0 {
		return nil, errorx.New(errno.ErrNoBoundedClass, errorx.KV("id", userId), errorx.KV("role", enum.UserRoleI2S[enum.UserRoleClassTeacher]))
	}

	grades, classes := boundClassesToGradesClasses(boundClasses, pUnit.StartGrade)
	ds, de := fullDayWindow(start, end)

	// 活跃趋势（按星期聚合，起讫时间内所有周一/二/... 之和）
	activeCnt, err := d.ConversationMapper.CountActiveUsersByWeekdayByClassList(ctx, grades, classes, ds, de)
	if err != nil {
		logs.Errorf("count active users by class list trend error: %s", errorx.ErrorWithoutStack(err))
		return nil, errorx.WrapByCode(err, errno.ErrDashboardActiveUserStat)
	}
	activePoints := buildTrendPoints(activeCnt)

	// 对话频率趋势（按星期聚合）
	convCnt, err := d.ConversationMapper.CountConversationsByWeekdayByClassList(ctx, grades, classes, ds, de)
	if err != nil {
		logs.Errorf("count conversations by class list trend error: %s", errorx.ErrorWithoutStack(err))
		return nil, errorx.WrapByCode(err, errno.ErrDashboardConversationStat)
	}
	conversationPoints := buildTrendPoints(convCnt)

	// 对话时长分布
	conversationDurations := make([]*core_api.ConversationDuration, 0, len(durationBucketDefs))
	for i, b := range durationBucketDefs {
		cnt, err := d.ConversationMapper.CountByDurationBucketByClassList(ctx, grades, classes, b.min, b.max, ds, de)
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
	for _, bc := range boundClasses {
		enrollYears = append(enrollYears, int32(bc.EnrollYear))
	}

	riskMap, total, err := d.AlarmMapper.CountAlarmUsersByGradeAndClasses(ctx, unitOID, pUnit.StartGrade, enrollYears, classes, ds, de)
	if err != nil {
		logs.Errorf("count alarm users by grade and classes error: %s", errorx.ErrorWithoutStack(err))
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

// GetPsychTrend4ClsTch 班主任版心理趋势
func (d *DashboardDomain) GetPsychTrend4ClsTch(ctx context.Context, userId string, unitOID bson.ObjectID, start, end time.Time) (*core_api.DashboardGetPsychTrendResp, error) {
	pUnit, err := d.UnitMapper.FindOneById(ctx, unitOID)
	if err != nil {
		return nil, errorx.New(errno.ErrInvalidParams, errorx.KV("field", "UnitID"))
	}

	// 获取班主任绑定的班级
	userOID, _ := bson.ObjectIDFromHex(userId)

	boundClasses, err := d.UserMapper.GetClassTeacherBoundClasses(ctx, userOID)
	if err != nil {
		logs.Errorf("get class teacher bound classes error: %s", errorx.ErrorWithoutStack(err))
		return nil, errorx.New(errno.ErrDashboardTotalUserStat)
	}

	if len(boundClasses) == 0 {
		return nil, errorx.New(errno.ErrNoBoundedClass, errorx.KV("id", userId), errorx.KV("role", "classTeacher"))
	}

	// 计算班级列表
	grades, classes := boundClassesToGradesClasses(boundClasses, pUnit.StartGrade)

	// 情绪/风险等级来自时间段内每个用户最后一份报表（按班级筛选）
	stats, err := d.ReportMapper.GetUserPsychStats(ctx, &unitOID, start, end, grades, classes)
	if err != nil {
		logs.Errorf("get user psych stats by class list error: %s", errorx.ErrorWithoutStack(err))
		return nil, errorx.New(errno.ErrDashboardRiskDistribution)
	}

	// 关键词词云（按班级筛选，来自报表）
	keywords, err := wordcld.Extractor.FromUnitKWsByClassList(ctx, unitOID, grades, classes, start, end)
	if err != nil {
		logs.Errorf("get keywords by class list error: %s", errorx.ErrorWithoutStack(err))
		return nil, errorx.New(errno.ErrDashboardGetUserKeywords)
	}

	return &core_api.DashboardGetPsychTrendResp{
		EmotionRatio: buildEmotionRatio(stats),
		Risks:        buildRiskDistribution(stats),
		Keywords:     keywords,
		Code:         0,
		Msg:          "success",
	}, nil
}

// ListClasses4ClsTch 班主任端-列出所带班级
func (d *DashboardDomain) ListClasses4ClsTch(ctx context.Context, userId string, unitOID bson.ObjectID, req *core_api.DashboardListClassesReq) (*core_api.DashboardListClassesResp, error) {
	pUnit, err := d.UnitMapper.FindOneById(ctx, unitOID)
	if err != nil {
		return nil, errorx.New(errno.ErrInvalidParams, errorx.KV("field", "UnitID"))
	}

	// 获取班主任绑定的班级
	userOID, _ := bson.ObjectIDFromHex(userId)

	boundClasses, err := d.UserMapper.GetClassTeacherBoundClasses(ctx, userOID)
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
	grades, classes := boundClassesToGradesClasses(boundClasses, pUnit.StartGrade)

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
	clsStats, err := d.UserMapper.CountByClasses(ctx, unitOID, pUnit.StartGrade, grades, classes)
	if err != nil {
		return nil, errorx.New(errno.ErrCountUserByClasses)
	}
	clsTeachers, err := d.UserMapper.FindUnitClassTeachers(ctx, unitOID, pUnit.StartGrade)
	if err != nil {
		return nil, errorx.New(errno.ErrCountUserByClasses)
	}

	// 整理结果，构建响应
	return &core_api.DashboardListClassesResp{
		Grades: aggregateGradesAndClasses(clsStats, clsTeachers),
	}, nil
}

// ListUsers4ClsTch 班主任端-列出所带班级的学生
func (d *DashboardDomain) ListUsers4ClsTch(ctx context.Context, userId string, unitOID bson.ObjectID, req *core_api.DashboardListUsersReq) (*core_api.DashboardListUsersResp, error) {
	pUnit, err := d.UnitMapper.FindOneById(ctx, unitOID)
	if err != nil {
		logs.Errorf("get unit error: %s", errorx.ErrorWithoutStack(err))
		return nil, errorx.New(errno.ErrInvalidParams, errorx.KV("field", "UnitID"))
	}

	userOID, err := bson.ObjectIDFromHex(userId)
	if err != nil {
		return nil, errorx.New(errno.ErrInvalidParams, errorx.KV("field", "UserID"))
	}

	boundClasses, err := d.UserMapper.GetClassTeacherBoundClasses(ctx, userOID)
	if err != nil {
		logs.Errorf("get class teacher bound classes error: %s", errorx.ErrorWithoutStack(err))
		return nil, errorx.New(errno.ErrCountUserByClasses)
	}

	emptyResp := func() *core_api.DashboardListUsersResp {
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
		}
	}

	if len(boundClasses) == 0 {
		return emptyResp(), nil
	}

	// 计算年级和班级列表
	grades, classes := boundClassesToGradesClasses(boundClasses, pUnit.StartGrade)

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
		return emptyResp(), nil
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

	dbUsers, total, err := d.UserMapper.ListUsers(ctx, opts)
	if err != nil {
		return nil, errorx.New(errno.ErrNotFound)
	}

	pg := util.PaginationRes(int32(total), req.PaginationOptions)
	riskUsers, err := d.completeRiskUser(ctx, dbUsers, pUnit.StartGrade)

	return &core_api.DashboardListUsersResp{
		RiskUsers:  riskUsers,
		Pagination: pg,
	}, err
}

// CreateRemark4ClsTch 班主任端-添加备注
func (d *DashboardDomain) CreateRemark4ClsTch(ctx context.Context, teacherId string, req *core_api.DashboardCreateRemarkReq) (*core_api.DashboardCreateRemarkResp, error) {
	userOID, err := bson.ObjectIDFromHex(req.UserId)
	if err != nil {
		return nil, errorx.New(errno.ErrInvalidParams, errorx.KV("field", "UserID"), errorx.KV("value", "用户ID"))
	}
	targetUser, err := d.UserMapper.FindOneById(ctx, userOID)
	if err != nil {
		logs.Errorf("find user by id error: %s", errorx.ErrorWithoutStack(err))
		return nil, errorx.New(errno.ErrNotFound, errorx.KV("field", "用户"))
	}

	// 校验归属
	teacherOID, _ := bson.ObjectIDFromHex(teacherId)
	authorized, err := d.isStudentInTeacherClasses(ctx, teacherOID, targetUser)
	if err != nil || !authorized {
		return nil, errorx.New(errno.ErrInsufficientAuth)
	}

	if err := d.updateRemark(ctx, userOID, req.GetRemark()); err != nil {
		return nil, err
	}

	return &core_api.DashboardCreateRemarkResp{
		Code: 0,
		Msg:  "success",
	}, nil
}

// UserConvRecords4ClsTch 班主任端-获取某用户对话记录
func (d *DashboardDomain) UserConvRecords4ClsTch(ctx context.Context, teacherId string, userOID bson.ObjectID, targetUser *user.User, req *core_api.DashboardUserConvRecordsReq) (*core_api.DashboardUserConvRecordsResp, error) {
	teacherOID, _ := bson.ObjectIDFromHex(teacherId)
	authorized, err := d.isStudentInTeacherClasses(ctx, teacherOID, targetUser)
	if err != nil || !authorized {
		return nil, errorx.New(errno.ErrInsufficientAuth)
	}

	return d.UserConvRecords4Unit(ctx, userOID, targetUser, req)
}

// GetReport4ClsTch 班主任端-查看报表详情
func (d *DashboardDomain) GetReport4ClsTch(ctx context.Context, teacherId string, convOID bson.ObjectID, targetUser *user.User, req *core_api.DashboardGetReportReq) (*core_api.DashboardGetReportResp, error) {
	teacherOID, _ := bson.ObjectIDFromHex(teacherId)
	authorized, err := d.isStudentInTeacherClasses(ctx, teacherOID, targetUser)
	if err != nil || !authorized {
		return nil, errorx.New(errno.ErrInsufficientAuth)
	}

	return d.GetReport4Unit(ctx, convOID, req)
}

// UnitConvRecords4ClsTch 班主任端-获取所带班级学生的对话记录
func (d *DashboardDomain) UnitConvRecords4ClsTch(ctx context.Context, userId string, unitOID bson.ObjectID, req *core_api.DashboardUnitConvRecordsReq) (*core_api.DashboardUnitConvRecordsResp, error) {
	pUnit, err := d.UnitMapper.FindOneById(ctx, unitOID)
	if err != nil {
		return nil, errorx.New(errno.ErrInvalidParams, errorx.KV("field", "UnitID"))
	}

	// 获取班主任绑定的班级
	userOID, err := bson.ObjectIDFromHex(userId)
	if err != nil {
		return nil, errorx.New(errno.ErrInvalidParams, errorx.KV("field", "UserID"))
	}

	boundClasses, err := d.UserMapper.GetClassTeacherBoundClasses(ctx, userOID)
	if err != nil {
		logs.Errorf("get class teacher bound classes error: %s", errorx.ErrorWithoutStack(err))
		return nil, errorx.New(errno.ErrCountUserByClasses)
	}

	if len(boundClasses) == 0 {
		return &core_api.DashboardUnitConvRecordsResp{
			ConversationList: make([]*core_api.ConvOverview, 0),
			Pagination:       util.PaginationRes(0, req.PaginationOptions),
			Code:             0,
			Msg:              "success",
		}, nil
	}

	// 计算年级和班级列表
	grades, classes := boundClassesToGradesClasses(boundClasses, pUnit.StartGrade)

	users, err := d.UserMapper.FindManyByClassList(ctx, unitOID, grades, classes)
	if err != nil {
		logs.Errorf("get users by class list error: %s", errorx.ErrorWithoutStack(err))
		return nil, errorx.New(errno.ErrNotFound, errorx.KV("field", "用户"))
	}

	if len(users) == 0 {
		return &core_api.DashboardUnitConvRecordsResp{
			ConversationList: make([]*core_api.ConvOverview, 0),
			Pagination:       util.PaginationRes(0, req.PaginationOptions),
			Code:             0,
			Msg:              "success",
		}, nil
	}

	userIds := make([]bson.ObjectID, len(users))
	for i, u := range users {
		userIds[i] = u.ID
	}

	total, err := d.ConversationMapper.CountByUserIds(ctx, userIds)
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
	convs, err := d.ConversationMapper.FindManyByUserIds(ctx, userIds, util.PagedFindOpt(req.PaginationOptions).SetSort(bson.D{{cst.EndTime, -1}}))
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
	userMap, err := d.UserMapper.BatchFindByIDs(ctx, usrIds)
	if err != nil {
		logs.Errorf("get user error: %s", errorx.ErrorWithoutStack(err))
		return nil, errorx.New(errno.ErrNotFound, errorx.KV("field", "用户"))
	}

	// 批量判断会话是否存在待处理预警
	needsAlarm, err := d.AlarmMapper.BatchExistsByConvId(ctx, convIds)
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

// isStudentInTeacherClasses 判断学生是否属于班主任所带班级
func (d *DashboardDomain) isStudentInTeacherClasses(ctx context.Context, teacherOID bson.ObjectID, targetUser *user.User) (bool, error) {
	// 获取该学生所属单位的配置，以计算年级
	pUnit, err := d.UnitMapper.FindOneById(ctx, targetUser.UnitID)
	if err != nil {
		logs.Errorf("get unit error: %s", errorx.ErrorWithoutStack(err))
		return false, err
	}

	boundClasses, err := d.UserMapper.GetClassTeacherBoundClasses(ctx, teacherOID)
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
