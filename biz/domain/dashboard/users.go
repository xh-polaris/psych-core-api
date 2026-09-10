package dashboard

import (
	"context"
	"sync"

	"github.com/xh-polaris/psych-core-api/biz/application/dto/basic"
	"github.com/xh-polaris/psych-core-api/biz/application/dto/core_api"
	"github.com/xh-polaris/psych-core-api/biz/infra/mapper/alarm"
	"github.com/xh-polaris/psych-core-api/biz/infra/mapper/message"
	"github.com/xh-polaris/psych-core-api/biz/infra/mapper/report"
	"github.com/xh-polaris/psych-core-api/biz/infra/mapper/user"
	"github.com/xh-polaris/psych-core-api/biz/infra/util"
	"github.com/xh-polaris/psych-core-api/pkg/errorx"
	"github.com/xh-polaris/psych-core-api/pkg/logs"
	"github.com/xh-polaris/psych-core-api/types/enum"
	"github.com/xh-polaris/psych-core-api/types/errno"
	"go.mongodb.org/mongo-driver/v2/bson"
)

// ListUsers 列出学生（单位端全量 / 班主任端限定所带班级）
func (d *DashboardDomain) ListUsers(ctx context.Context, scope *Scope, req *core_api.DashboardListUsersReq) (*core_api.DashboardListUsersResp, error) {
	rs, err := d.resolveScope(ctx, scope)
	if err != nil {
		return nil, err
	}
	if scope.IsClassTeacher() {
		return d.listUsers4ClsTch(ctx, rs, req)
	}
	return d.listUsers4Unit(ctx, rs, req)
}

// listUsers4Unit 单位端-列出用户
func (d *DashboardDomain) listUsers4Unit(ctx context.Context, rs *resolvedScope, req *core_api.DashboardListUsersReq) (*core_api.DashboardListUsersResp, error) {
	// 创建搜索配置
	opts := &user.ListUserOptions{
		UnitID:     *rs.scope.UnitID,
		StartGrade: rs.startGrade,
		Grade:      req.Grade,
		Class:      req.Class,
		Level:      req.Level,
		Gender:     req.Gender,
		Keyword:    req.Keyword,
		Page:       req.PaginationOptions.GetPage(),
		Limit:      req.PaginationOptions.GetLimit(),
	}

	dbUsers, total, err := d.UserMapper.ListUsers(ctx, opts)
	if err != nil {
		return nil, errorx.New(errno.ErrUserNotFound)
	}

	pg := util.PaginationRes(int32(total), req.PaginationOptions)
	riskUsers, err := d.completeRiskUser(ctx, dbUsers, rs.startGrade)

	return &core_api.DashboardListUsersResp{
		RiskUsers:  riskUsers,
		Pagination: pg,
	}, err
}

// listUsers4ClsTch 班主任端-列出所带班级的学生
func (d *DashboardDomain) listUsers4ClsTch(ctx context.Context, rs *resolvedScope, req *core_api.DashboardListUsersReq) (*core_api.DashboardListUsersResp, error) {
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

	if rs.emptyClasses() {
		return emptyResp(), nil
	}

	grades, classes := rs.grades, rs.classes

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
		UnitID:     *rs.scope.UnitID,
		StartGrade: rs.startGrade,
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
	riskUsers, err := d.completeRiskUser(ctx, dbUsers, rs.startGrade)

	return &core_api.DashboardListUsersResp{
		RiskUsers:  riskUsers,
		Pagination: pg,
	}, err
}

// CreateRemark 添加备注（班主任/单位管理员需目标学生在范围内）
func (d *DashboardDomain) CreateRemark(ctx context.Context, scope *Scope, req *core_api.DashboardCreateRemarkReq) (*core_api.DashboardCreateRemarkResp, error) {
	userOID, err := bson.ObjectIDFromHex(req.UserId)
	if err != nil {
		return nil, errorx.New(errno.ErrInvalidParams, errorx.KV("field", "UserID"), errorx.KV("value", "用户ID"))
	}
	targetUser, err := d.UserMapper.FindOneById(ctx, userOID)
	if err != nil {
		logs.Errorf("find user by id error: %s", errorx.ErrorWithoutStack(err))
		return nil, errorx.New(errno.ErrNotFound, errorx.KV("field", "用户"))
	}

	rs, err := d.resolveScope(ctx, scope)
	if err != nil {
		return nil, err
	}

	// 校验归属
	if err := d.ensureStudentInScope(ctx, rs, targetUser); err != nil {
		return nil, err
	}

	if err := d.updateRemark(ctx, userOID, req.GetRemark()); err != nil {
		return nil, err
	}

	return &core_api.DashboardCreateRemarkResp{
		Code: 0,
		Msg:  "success",
	}, nil
}

// completeRiskUser 补全风险用户信息（对话统计、关键词、风险等级）
// 高危学生（User.RiskLevel==High，少数）以最新 AlarmRecord 为准；剩余学生以最新 SimpleReport 为准
func (d *DashboardDomain) completeRiskUser(ctx context.Context, dbUsers []*user.User, startGrade int) ([]*core_api.RiskUser, error) {
	if len(dbUsers) == 0 {
		return make([]*core_api.RiskUser, 0), nil
	}

	uids := make([]bson.ObjectID, len(dbUsers))
	highRiskUids := make([]bson.ObjectID, 0, len(dbUsers))
	for i, dbUser := range dbUsers {
		uids[i] = dbUser.ID
		if dbUser.RiskLevel == enum.UserRiskLevelHigh {
			highRiskUids = append(highRiskUids, dbUser.ID)
		}
	}

	// 三路并行：学生消息轮数、AlarmRecord（高危学生）、最新 SimpleReport（全部，作剩余学生数据源及高危回退）
	var msgStats map[bson.ObjectID]*message.MsgStats
	var alarmMap map[bson.ObjectID]*alarm.Alarm
	var latestReports map[bson.ObjectID]*report.Report
	var msgErr, alarmErr, rptErr error

	var wg sync.WaitGroup
	wg.Add(3)
	go func() {
		defer wg.Done()
		msgStats, msgErr = d.MessageMapper.BatchMessageStats(ctx, uids)
		if msgErr != nil {
			logs.Warnf("查询对话统计失败: %v", errorx.ErrorWithoutStack(msgErr))
		}
	}()
	go func() {
		defer wg.Done()
		alarmMap, alarmErr = d.AlarmMapper.BatchFindLatestByUserIds(ctx, highRiskUids)
		if alarmErr != nil {
			logs.Errorf("查询高危学生预警失败: %v", errorx.ErrorWithoutStack(alarmErr))
		}
	}()
	go func() {
		defer wg.Done()
		latestReports, rptErr = d.ReportMapper.BatchFindUserLatest(ctx, uids)
		if rptErr != nil {
			logs.Errorf("查询用户最新报表失败: %v", errorx.ErrorWithoutStack(rptErr))
		}
	}()
	wg.Wait()

	if msgErr != nil || msgStats == nil {
		return nil, errorx.New(errno.ErrDashboardGetUserConversationStatic)
	}
	if rptErr != nil {
		return nil, errorx.New(errno.ErrDashboardRiskDistribution)
	}
	if alarmErr != nil {
		return nil, errorx.New(errno.ErrDashboardAlarmUserStat)
	}

	riskUsers := make([]*core_api.RiskUser, len(dbUsers))
	for i, dbUser := range dbUsers {
		remark := &core_api.Remark{
			Time:    dbUser.Remark.CreateTime.Unix(),
			Content: dbUser.Remark.Content,
		}
		calculatedGrade := dbUser.CalculateGrade(startGrade)
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

		// 高危学生：以最新 AlarmRecord 为准（情绪/关键词），无预警则回退最新报表
		if al := alarmMap[dbUser.ID]; al != nil {
			riskUsers[i].Level = int32(enum.UserRiskLevelHigh)
			if len(al.Keywords) > 0 {
				riskUsers[i].Keywords = al.Keywords
			}
			continue
		}

		// 剩余学生：最新 SimpleReport 提供 风险等级/关键词
		if rpt := latestReports[dbUser.ID]; rpt != nil {
			if rpt.SimpleReport != nil {
				riskUsers[i].Level = int32(enum.RiskLevelToInt(rpt.SimpleReport.Summary.RiskLevel))
			}
			if kw := util.KeywordsMap2Slice(rpt.Keywords); len(kw) > 0 {
				riskUsers[i].Keywords = kw
			}
		}
		if riskUsers[i].Level <= 0 {
			riskUsers[i].Level = int32(enum.UserRiskLevelNormal)
		}
	}

	return riskUsers, nil
}
