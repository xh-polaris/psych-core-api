package dashboard

import (
	"context"
	"time"

	"github.com/xh-polaris/psych-core-api/biz/application/dto/core_api"
	"github.com/xh-polaris/psych-core-api/biz/cst"
	"github.com/xh-polaris/psych-core-api/biz/infra/mapper/user"
	"github.com/xh-polaris/psych-core-api/biz/infra/util"
	"github.com/xh-polaris/psych-core-api/pkg/errorx"
	"github.com/xh-polaris/psych-core-api/pkg/logs"
	"github.com/xh-polaris/psych-core-api/types/enum"
	"github.com/xh-polaris/psych-core-api/types/errno"
	"go.mongodb.org/mongo-driver/v2/bson"
)

// GetDataOverview4Unit 单位管理员版数据概览
func (d *DashboardDomain) GetDataOverview4Unit(ctx context.Context, unitOID bson.ObjectID, start, end time.Time) (*core_api.DashboardGetDataOverviewResp, error) {
	u := &unitOID

	// 用户数（增长：end 时快照对比 start 时快照）
	totalUsers, err := d.UserMapper.CountStudents(ctx, unitOID)
	if err != nil {
		return nil, errorx.New(errno.ErrDashboardTotalUserStat)
	}
	endUsers, err := d.UserMapper.CountStudentsByPeriod(ctx, u, time.Time{}, end)
	if err != nil {
		return nil, errorx.New(errno.ErrDashboardTotalUserStat)
	}
	startUsers, err := d.UserMapper.CountStudentsByPeriod(ctx, u, time.Time{}, start)
	if err != nil {
		return nil, errorx.New(errno.ErrDashboardTotalUserStat)
	}
	users := util.Wow{Cur: endUsers, Prev: startUsers}

	// 活跃用户数（增长：截止 end 的新增活跃数）
	totalActive, err := d.ConversationMapper.CountActiveUsers(ctx, u, start, end)
	if err != nil {
		return nil, errorx.WrapByCode(err, errno.ErrDashboardActiveUserStat)
	}
	endActive, err := d.ConversationMapper.CountActiveUsers(ctx, u, time.Time{}, end)
	if err != nil {
		return nil, errorx.WrapByCode(err, errno.ErrDashboardActiveUserStat)
	}
	startActive, err := d.ConversationMapper.CountActiveUsers(ctx, u, time.Time{}, start)
	if err != nil {
		return nil, errorx.WrapByCode(err, errno.ErrDashboardActiveUserStat)
	}
	active := util.Wow{Cur: endActive, Prev: startActive}

	// 对话数（增长：截止 end 的对话数对比截止 start）
	totalConv, err := d.ConversationMapper.CountUnitConvByPeriod(ctx, u, start, end)
	if err != nil {
		return nil, errorx.WrapByCode(err, errno.ErrDashboardConversationStat)
	}
	endConv, err := d.ConversationMapper.CountUnitConvByPeriod(ctx, u, time.Time{}, end)
	if err != nil {
		return nil, errorx.WrapByCode(err, errno.ErrDashboardConversationStat)
	}
	startConv, err := d.ConversationMapper.CountUnitConvByPeriod(ctx, u, time.Time{}, start)
	if err != nil {
		return nil, errorx.WrapByCode(err, errno.ErrDashboardConversationStat)
	}
	conv := util.Wow{Cur: endConv, Prev: startConv}

	// 对话时长（窗口均值对比截止 start 的累计均值）
	curAvg, err := d.ConversationMapper.AverageDurationByPeriod(ctx, u, start, end)
	if err != nil {
		return nil, errorx.WrapByCode(err, errno.ErrDashboardAvgDurationStat)
	}
	prevAvg, err := d.ConversationMapper.AverageDurationByPeriod(ctx, u, time.Time{}, start)
	if err != nil {
		return nil, errorx.WrapByCode(err, errno.ErrDashboardAvgDurationStat)
	}
	curAvg = util.Round2(curAvg)
	prevAvg = util.Round2(prevAvg)

	// 高风险用户数（存在 alarm 记录的当前高危用户，增长：截止 end 对比截止 start）
	totalAlarmUsers, err := d.AlarmMapper.CountAlarmUsers(ctx, u, time.Time{}, end)
	if err != nil {
		return nil, errorx.New(errno.ErrDashboardAlarmUserStat)
	}
	startAlarmUsers, err := d.AlarmMapper.CountAlarmUsers(ctx, u, time.Time{}, start)
	if err != nil {
		return nil, errorx.New(errno.ErrDashboardAlarmUserStat)
	}
	alrm := util.Wow{Cur: totalAlarmUsers, Prev: startAlarmUsers}

	return &core_api.DashboardGetDataOverviewResp{
		TotalUsers:                                   totalUsers,
		WeeklyIncreaseUsers:                          users.Inc(),
		WeeklyIncreaseUsersRate:                      users.Rate(),
		ActiveUsers:                                  util.Int32Ptr(totalActive),
		WeeklyIncreaseActiveUsers:                    util.Int32Ptr(active.Inc()),
		WeeklyIncreaseActiveUsersRate:                util.Float64Ptr(active.Rate()),
		TotalConversations:                           totalConv,
		WeeklyIncreaseConversations:                  conv.Inc(),
		WeeklyIncreaseConversationsRate:              conv.Rate(),
		AverageTimePerConversation:                   curAvg,
		WeeklyIncreaseAverageTimePerConversation:     util.Round2(curAvg - prevAvg),
		WeeklyIncreaseAverageTimePerConversationRate: util.RateF(curAvg, prevAvg),
		AlarmUsers:                                   totalAlarmUsers,
		WeeklyIncreaseAlarmUsers:                     alrm.Inc(),
		WeeklyIncreaseAlarmUsersRate:                 alrm.Rate(),
		Code:                                         0,
		Msg:                                          "success",
	}, nil
}

// GetDataTrend4Unit 单位管理员版数据趋势
func (d *DashboardDomain) GetDataTrend4Unit(ctx context.Context, unitOID bson.ObjectID, start, end time.Time) (*core_api.DashboardGetDataTrendResp, error) {
	pUnit, _ := d.UnitMapper.FindOneById(ctx, unitOID)
	startGrade := 1
	if pUnit != nil {
		startGrade = pUnit.StartGrade
	}
	ds, de := fullDayWindow(start, end)
	return d.dataTrend(ctx, &unitOID, startGrade, ds, de)
}

// GetPsychTrend4Unit 单位管理员版心理趋势
func (d *DashboardDomain) GetPsychTrend4Unit(ctx context.Context, unitOID bson.ObjectID, start, end time.Time) (*core_api.DashboardGetPsychTrendResp, error) {
	return d.psychTrend(ctx, &unitOID, start, end)
}

// ListClasses4Unit 单位端-列出班级
func (d *DashboardDomain) ListClasses4Unit(ctx context.Context, unitOID bson.ObjectID, req *core_api.DashboardListClassesReq) (*core_api.DashboardListClassesResp, error) {
	pUnit, err := d.UnitMapper.FindOneById(ctx, unitOID)
	if err != nil {
		return nil, errorx.New(errno.ErrInvalidParams, errorx.KV("field", "UnitID"))
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

// ListUsers4Unit 单位端-列出用户
func (d *DashboardDomain) ListUsers4Unit(ctx context.Context, unitOID bson.ObjectID, req *core_api.DashboardListUsersReq) (*core_api.DashboardListUsersResp, error) {
	pUnit, err := d.UnitMapper.FindOneById(ctx, unitOID)
	if err != nil {
		logs.Errorf("get unit error: %s", errorx.ErrorWithoutStack(err))
		return nil, errorx.New(errno.ErrInvalidParams, errorx.KV("field", "UnitID"))
	}

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

	dbUsers, total, err := d.UserMapper.ListUsers(ctx, opts)
	if err != nil {
		return nil, errorx.New(errno.ErrUserNotFound)
	}

	pg := util.PaginationRes(int32(total), req.PaginationOptions)
	riskUsers, err := d.completeRiskUser(ctx, dbUsers, pUnit.StartGrade)

	return &core_api.DashboardListUsersResp{
		RiskUsers:  riskUsers,
		Pagination: pg,
	}, err
}

// CreateRemark4Unit 单位端-添加备注
func (d *DashboardDomain) CreateRemark4Unit(ctx context.Context, req *core_api.DashboardCreateRemarkReq) (*core_api.DashboardCreateRemarkResp, error) {
	userOID, err := bson.ObjectIDFromHex(req.UserId)
	if err != nil {
		return nil, errorx.New(errno.ErrInvalidParams, errorx.KV("field", "UserID"), errorx.KV("value", "用户ID"))
	}
	targetUser, err := d.UserMapper.FindOneById(ctx, userOID)
	if err != nil {
		logs.Errorf("find user by id error: %s", errorx.ErrorWithoutStack(err))
		return nil, errorx.New(errno.ErrNotFound, errorx.KV("field", "用户"))
	}
	// 校验目标用户归属本单位
	if targetUser.UnitID.Hex() != req.GetUnitId() {
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

// UserConvRecords4Unit 单位端-获取某用户对话记录
func (d *DashboardDomain) UserConvRecords4Unit(ctx context.Context, userOID bson.ObjectID, targetUser *user.User, req *core_api.DashboardUserConvRecordsReq) (*core_api.DashboardUserConvRecordsResp, error) {
	// 获取用户对话频率趋势
	userConvTrend, err := d.getUserConvTrend(ctx, userOID)
	if err != nil {
		return nil, err
	}

	// 批量处理对话详情
	convDetail, pagination, err := d.listUserConvDetails(ctx, userOID, req.PaginationOptions)
	if err != nil {
		return nil, err
	}

	unitDAO, err := d.UnitMapper.FindOneById(ctx, targetUser.UnitID)
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

// GetReport4Unit 单位端-查看报表详情
func (d *DashboardDomain) GetReport4Unit(ctx context.Context, convOID bson.ObjectID, req *core_api.DashboardGetReportReq) (*core_api.DashboardGetReportResp, error) {
	rpt, err := d.ReportMapper.FindByConversationPreferSuccess(ctx, convOID)
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
		Analysis:       analysisToPB(rpt.Analysis),
		SimpleReport:   simpleReportToPB(rpt.SimpleReport),
		Code:           0,
		Msg:            "success",
	}
	if rpt.Character != nil {
		resp.CharacterId = rpt.Character.ID.Hex()
		resp.CharacterName = rpt.Character.Name
		resp.CharacterVoice = rpt.Character.Voice
		resp.CharacterImage = rpt.Character.Image
	}

	if rpt.Status != enum.ReportStatusSuccess {
		resp.Code = errno.ErrReportNotReady
		resp.Msg = "报表处理中，请稍后"
	}

	return resp, nil
}

// UnitConvRecords4Unit 单位端-获取单位下对话记录列表
func (d *DashboardDomain) UnitConvRecords4Unit(ctx context.Context, req *core_api.DashboardUnitConvRecordsReq) (*core_api.DashboardUnitConvRecordsResp, error) {
	unitOID, err := bson.ObjectIDFromHex(req.GetUnitId())
	if err != nil {
		return nil, errorx.New(errno.ErrInvalidParams, errorx.KV("field", "UnitId"), errorx.KV("value", "单位ID"))
	}

	total, err := d.ConversationMapper.CountByUnit(ctx, &unitOID)
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
	convs, err := d.ConversationMapper.FindManyByUnitId(ctx, &unitOID, util.PagedFindOpt(req.PaginationOptions).SetSort(bson.D{{cst.EndTime, -1}}))
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
	users, err := d.UserMapper.BatchFindByIDs(ctx, usrIds)
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

	unitDAO, err := d.UnitMapper.FindOneById(ctx, unitOID)
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
