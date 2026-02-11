package service

import (
	"context"
	"sort"
	"sync"

	"github.com/google/wire"
	"github.com/xh-polaris/psych-core-api/biz/infra/mapper/message"
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
	DashboardGetDataOverview(ctx context.Context, req *core_api.DashboardGetDataOverviewReq) (resp *core_api.DashboardGetDataOverviewResp, err error)
	DashboardListClasses(ctx context.Context, req *core_api.DashboardListClassesReq) (resp *core_api.DashboardListClassesResp, err error)
	DashboardListUsers(ctx context.Context, req *core_api.DashboardListUsersReq) (resp *core_api.DashboardListUsersResp, err error)
}

type DashboardService struct {
	UserMapper    user.IMongoMapper
	MessageMapper message.MongoMapper
}

var DashboardServiceSet = wire.NewSet(
	wire.Struct(new(DashboardService), "*"),
	wire.Bind(new(IDashboardService), new(*DashboardService)),
)

func (s *DashboardService) DashboardGetDataOverview(ctx context.Context, req *core_api.DashboardGetDataOverviewReq) (resp *core_api.DashboardGetDataOverviewResp, err error) {
	return nil, errorx.New(errno.ErrUnImplement)
}

func (s *DashboardService) DashboardListClasses(ctx context.Context, req *core_api.DashboardListClassesReq) (resp *core_api.DashboardListClassesResp, err error) {
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

func (s *DashboardService) DashboardListUsers(ctx context.Context, req *core_api.DashboardListUsersReq) (resp *core_api.DashboardListUsersResp, err error) {
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
