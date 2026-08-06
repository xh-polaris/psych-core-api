package service

import (
	"context"

	"github.com/google/wire"
	"github.com/xh-polaris/psych-core-api/biz/application/dto/core_api"
	"github.com/xh-polaris/psych-core-api/biz/domain/auth"
	"github.com/xh-polaris/psych-core-api/biz/domain/dashboard"
	"github.com/xh-polaris/psych-core-api/biz/infra/mapper/conversation"
	"github.com/xh-polaris/psych-core-api/biz/infra/mapper/user"
	"github.com/xh-polaris/psych-core-api/biz/infra/util"
	"github.com/xh-polaris/psych-core-api/pkg/errorx"
	"github.com/xh-polaris/psych-core-api/pkg/logs"
	"github.com/xh-polaris/psych-core-api/types/enum"
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

// DashboardService 数据看板服务：仅做权限识别与按角色路由，业务实现见 dashboard 领域层。
// UserMapper/ConversationMapper 仅用于识别目标对象所属单位（IdentifyRole 前置）。
type DashboardService struct {
	AuthDomain         auth.IAuthDomain
	UserMapper         user.IMongoMapper
	ConversationMapper conversation.IMongoMapper
	DashboardDomain    dashboard.IDashboardDomain
}

var DashboardServiceSet = wire.NewSet(
	wire.Struct(new(DashboardService), "*"),
	wire.Bind(new(IDashboardService), new(*DashboardService)),
)

// DashboardGetDataOverview 【指标总览】学生总数，活跃用户数，总对话数、平均对话时长、高风险用户数
func (s *DashboardService) DashboardGetDataOverview(ctx context.Context, req *core_api.DashboardGetDataOverviewReq) (*core_api.DashboardGetDataOverviewResp, error) {
	meta, role, err := s.AuthDomain.IdentifyRole(ctx, req.GetUnitId())
	if err != nil {
		return nil, err
	}

	endTime := util.ParseEndTime(req.GetEndTime())
	startTime := util.ParseStartTime(req.GetStartTime(), endTime)

	switch role {
	case enum.UserRoleSuperAdmin:
		return s.DashboardDomain.GetDataOverview4Admin(ctx, startTime, endTime)

	case enum.UserRoleUnitAdmin:
		unitOID, _ := bson.ObjectIDFromHex(req.GetUnitId())
		return s.DashboardDomain.GetDataOverview4Unit(ctx, unitOID, startTime, endTime)

	case enum.UserRoleClassTeacher:
		unitOID, err := bson.ObjectIDFromHex(req.GetUnitId())
		if err != nil {
			return nil, errorx.New(errno.ErrInvalidParams, errorx.KV("field", "UnitID"), errorx.KV("value", "单位ID"))
		}
		return s.DashboardDomain.GetDataOverview4ClsTch(ctx, meta.UserId, unitOID, startTime, endTime)
	}
	return nil, errorx.New(errno.ErrUnImplement)
}

// DashboardGetDataTrend 【数据趋势】活跃趋势、对话频率、对话时长分布、各年级风险分布
func (s *DashboardService) DashboardGetDataTrend(ctx context.Context, req *core_api.DashboardGetDataTrendReq) (*core_api.DashboardGetDataTrendResp, error) {
	meta, role, err := s.AuthDomain.IdentifyRole(ctx, req.GetUnitId())
	if err != nil {
		return nil, err
	}

	endTime := util.ParseEndTime(req.GetEndTime())
	startTime := util.ParseStartTime(req.GetStartTime(), endTime)

	switch role {
	case enum.UserRoleSuperAdmin:
		return s.DashboardDomain.GetDataTrend4Admin(ctx, startTime, endTime)

	case enum.UserRoleUnitAdmin:
		oid, _ := bson.ObjectIDFromHex(req.GetUnitId())
		return s.DashboardDomain.GetDataTrend4Unit(ctx, oid, startTime, endTime)

	case enum.UserRoleClassTeacher:
		oid, _ := bson.ObjectIDFromHex(req.GetUnitId())
		return s.DashboardDomain.GetDataTrend4ClsTch(ctx, meta.UserId, oid, startTime, endTime)
	}
	return nil, errorx.New(errno.ErrUnImplement)
}

// DashboardGetPsychTrend 【心理趋势】情绪分布、风险性别分布、关键词词云
func (s *DashboardService) DashboardGetPsychTrend(ctx context.Context, req *core_api.DashboardGetPsychTrendReq) (*core_api.DashboardGetPsychTrendResp, error) {
	meta, role, err := s.AuthDomain.IdentifyRole(ctx, req.GetUnitId())
	if err != nil {
		return nil, err
	}

	endTime := util.ParseEndTime(req.GetEndTime())
	startTime := util.ParseStartTime(req.GetStartTime(), endTime)

	switch role {
	case enum.UserRoleSuperAdmin:
		return s.DashboardDomain.GetPsychTrend4Admin(ctx, startTime, endTime)

	case enum.UserRoleUnitAdmin:
		oid, _ := bson.ObjectIDFromHex(req.GetUnitId())
		return s.DashboardDomain.GetPsychTrend4Unit(ctx, oid, startTime, endTime)

	case enum.UserRoleClassTeacher:
		oid, _ := bson.ObjectIDFromHex(req.GetUnitId())
		return s.DashboardDomain.GetPsychTrend4ClsTch(ctx, meta.UserId, oid, startTime, endTime)
	}
	return nil, errorx.New(errno.ErrUnImplement)
}

// DashboardListUnits 超管端-所有单位列表
func (s *DashboardService) DashboardListUnits(ctx context.Context, req *core_api.DashboardListUnitsReq) (*core_api.DashboardListUnitsResp, error) {
	if _, _, err := s.AuthDomain.IdentifyRole(ctx, ""); err != nil {
		return nil, err
	}
	return s.DashboardDomain.ListUnits(ctx)
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

	switch role {
	case enum.UserRoleUnitAdmin:
		return s.DashboardDomain.ListClasses4Unit(ctx, unitOID, req)
	case enum.UserRoleClassTeacher:
		return s.DashboardDomain.ListClasses4ClsTch(ctx, meta.UserId, unitOID, req)
	}
	return nil, errorx.New(errno.ErrInvalidRole)
}

// DashboardListUsers 【用户管理】列出某班级学生
func (s *DashboardService) DashboardListUsers(ctx context.Context, req *core_api.DashboardListUsersReq) (*core_api.DashboardListUsersResp, error) {
	meta, role, err := s.AuthDomain.IdentifyRole(ctx, req.GetUnitId())
	if err != nil {
		return nil, err
	}

	unitOID, _ := bson.ObjectIDFromHex(req.UnitId)

	switch role {
	case enum.UserRoleUnitAdmin:
		return s.DashboardDomain.ListUsers4Unit(ctx, unitOID, req)
	case enum.UserRoleClassTeacher:
		return s.DashboardDomain.ListUsers4ClsTch(ctx, meta.UserId, unitOID, req)
	}
	return nil, errorx.New(errno.ErrInvalidRole)
}

// DashboardCreateRemark 添加备注
func (s *DashboardService) DashboardCreateRemark(ctx context.Context, req *core_api.DashboardCreateRemarkReq) (*core_api.DashboardCreateRemarkResp, error) {
	meta, role, err := s.AuthDomain.IdentifyRole(ctx, req.GetUnitId())
	if err != nil {
		return nil, err
	}

	switch role {
	case enum.UserRoleUnitAdmin:
		return s.DashboardDomain.CreateRemark4Unit(ctx, req)
	case enum.UserRoleClassTeacher:
		return s.DashboardDomain.CreateRemark4ClsTch(ctx, meta.UserId, req)
	}
	return nil, errorx.New(errno.ErrInsufficientAuth)
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
		return s.DashboardDomain.UserConvRecords4Unit(ctx, userOID, targetUser, req)
	case enum.UserRoleClassTeacher:
		return s.DashboardDomain.UserConvRecords4ClsTch(ctx, meta.UserId, userOID, targetUser, req)
	}
	return nil, errorx.New(errno.ErrInsufficientAuth)
}

// DashboardGetReport 查看报表详情
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
		return s.DashboardDomain.GetReport4Unit(ctx, convOID, req)
	case enum.UserRoleClassTeacher:
		return s.DashboardDomain.GetReport4ClsTch(ctx, meta.UserId, convOID, usr, req)
	}
	return nil, errorx.New(errno.ErrInsufficientAuth)
}

// DashboardUnitConvRecords 单位/平台对话记录列表
func (s *DashboardService) DashboardUnitConvRecords(ctx context.Context, req *core_api.DashboardUnitConvRecordsReq) (*core_api.DashboardUnitConvRecordsResp, error) {
	meta, role, err := s.AuthDomain.IdentifyRole(ctx, req.GetUnitId())
	if err != nil {
		return nil, err
	}

	switch role {
	case enum.UserRoleSuperAdmin:
		return s.DashboardDomain.UnitConvRecords4Admin(ctx, req)

	case enum.UserRoleUnitAdmin:
		return s.DashboardDomain.UnitConvRecords4Unit(ctx, req)

	case enum.UserRoleClassTeacher:
		unitOID, err := bson.ObjectIDFromHex(req.GetUnitId())
		if err != nil {
			return nil, errorx.New(errno.ErrInvalidParams, errorx.KV("field", "UnitID"), errorx.KV("value", "单位 ID"))
		}
		if meta.UnitId != "" && meta.UnitId != req.GetUnitId() {
			return nil, errorx.New(errno.ErrInsufficientAuth)
		}
		return s.DashboardDomain.UnitConvRecords4ClsTch(ctx, meta.UserId, unitOID, req)
	}
	return nil, errorx.New(errno.ErrInsufficientAuth)
}
