package service

import (
	"context"

	"github.com/google/wire"
	"github.com/xh-polaris/psych-core-api/biz/application/dto/core_api"
	"github.com/xh-polaris/psych-core-api/biz/domain/auth"
	"github.com/xh-polaris/psych-core-api/biz/domain/dashboard"
	"github.com/xh-polaris/psych-core-api/biz/infra/mapper/alarm"
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

	// 对话记录 / 报表
	DashboardUserConvRecords(ctx context.Context, req *core_api.DashboardUserConvRecordsReq) (*core_api.DashboardUserConvRecordsResp, error)
	DashboardUnitConvRecords(ctx context.Context, req *core_api.DashboardUnitConvRecordsReq) (*core_api.DashboardUnitConvRecordsResp, error)
	DashboardGetReport(ctx context.Context, req *core_api.DashboardGetReportReq) (*core_api.DashboardGetReportResp, error)

	// 预警
	DashboardGetAlarmOverview(ctx context.Context, req *core_api.DashboardGetAlarmOverviewReq) (*core_api.DashboardGetAlarmOverviewResp, error)
	DashboardListAlarmRecords(ctx context.Context, req *core_api.DashboardListAlarmRecordsReq) (*core_api.DashboardListAlarmRecordsResp, error)
	DashboardUpdateAlarm(ctx context.Context, req *core_api.DashboardUpdateAlarmReq) (*core_api.DashboardUpdateAlarmResp, error)
}

// DashboardService 数据看板服务：仅做权限识别、构造 Scope 并按角色路由，业务实现见 dashboard 领域层。
// UserMapper/ConversationMapper 用于识别目标对象所属单位（IdentifyRole 前置）；AlarmMapper 用于更新预警的归属校验。
type DashboardService struct {
	AuthDomain         auth.IAuthDomain
	UserMapper         user.IMongoMapper
	ConversationMapper conversation.IMongoMapper
	AlarmMapper        alarm.IMongoMapper
	DashboardDomain    dashboard.IDashboardDomain
}

var DashboardServiceSet = wire.NewSet(
	wire.Struct(new(DashboardService), "*"),
	wire.Bind(new(IDashboardService), new(*DashboardService)),
)

// buildScope 从请求单位号 + 鉴权元信息构造领域范围。unitID 为空（超管全局）时 UnitID=nil。
func (s *DashboardService) buildScope(unitID string, meta *auth.Meta, role int) (*dashboard.Scope, error) {
	scope := &dashboard.Scope{Role: role, UserID: meta.UserId}
	if unitID != "" {
		oid, err := bson.ObjectIDFromHex(unitID)
		if err != nil {
			return nil, errorx.New(errno.ErrInvalidParams, errorx.KV("field", "UnitID"), errorx.KV("value", "单位ID"))
		}
		scope.UnitID = &oid
	}
	return scope, nil
}

// DashboardGetDataOverview 【指标总览】学生总数，活跃用户数，总对话数、平均对话时长、高风险用户数
func (s *DashboardService) DashboardGetDataOverview(ctx context.Context, req *core_api.DashboardGetDataOverviewReq) (*core_api.DashboardGetDataOverviewResp, error) {
	meta, role, err := s.AuthDomain.IdentifyRole(ctx, req.GetUnitId())
	if err != nil {
		return nil, err
	}
	scope, err := s.buildScope(req.GetUnitId(), meta, role)
	if err != nil {
		return nil, err
	}

	endTime := util.ParseEndTime(req.GetEndTime())
	startTime := util.ParseStartTime(req.GetStartTime(), endTime)
	return s.DashboardDomain.GetDataOverview(ctx, scope, startTime, endTime)
}

// DashboardGetDataTrend 【数据趋势】活跃趋势、对话频率、对话时长分布、各年级风险分布
func (s *DashboardService) DashboardGetDataTrend(ctx context.Context, req *core_api.DashboardGetDataTrendReq) (*core_api.DashboardGetDataTrendResp, error) {
	meta, role, err := s.AuthDomain.IdentifyRole(ctx, req.GetUnitId())
	if err != nil {
		return nil, err
	}
	scope, err := s.buildScope(req.GetUnitId(), meta, role)
	if err != nil {
		return nil, err
	}

	endTime := util.ParseEndTime(req.GetEndTime())
	startTime := util.ParseStartTime(req.GetStartTime(), endTime)
	return s.DashboardDomain.GetDataTrend(ctx, scope, startTime, endTime)
}

// DashboardGetPsychTrend 【心理趋势】情绪分布、风险性别分布、关键词词云
func (s *DashboardService) DashboardGetPsychTrend(ctx context.Context, req *core_api.DashboardGetPsychTrendReq) (*core_api.DashboardGetPsychTrendResp, error) {
	meta, role, err := s.AuthDomain.IdentifyRole(ctx, req.GetUnitId())
	if err != nil {
		return nil, err
	}
	scope, err := s.buildScope(req.GetUnitId(), meta, role)
	if err != nil {
		return nil, err
	}

	endTime := util.ParseEndTime(req.GetEndTime())
	startTime := util.ParseStartTime(req.GetStartTime(), endTime)
	return s.DashboardDomain.GetPsychTrend(ctx, scope, startTime, endTime)
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
	switch role {
	case enum.UserRoleUnitAdmin, enum.UserRoleClassTeacher:
		scope, err := s.buildScope(req.GetUnitId(), meta, role)
		if err != nil {
			return nil, err
		}
		return s.DashboardDomain.ListClasses(ctx, scope, req)
	}
	return nil, errorx.New(errno.ErrInvalidRole)
}

// DashboardListUsers 【用户管理】列出某班级学生
func (s *DashboardService) DashboardListUsers(ctx context.Context, req *core_api.DashboardListUsersReq) (*core_api.DashboardListUsersResp, error) {
	meta, role, err := s.AuthDomain.IdentifyRole(ctx, req.GetUnitId())
	if err != nil {
		return nil, err
	}
	switch role {
	case enum.UserRoleUnitAdmin, enum.UserRoleClassTeacher:
		scope, err := s.buildScope(req.GetUnitId(), meta, role)
		if err != nil {
			return nil, err
		}
		return s.DashboardDomain.ListUsers(ctx, scope, req)
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
	case enum.UserRoleUnitAdmin, enum.UserRoleClassTeacher:
		scope, err := s.buildScope(req.GetUnitId(), meta, role)
		if err != nil {
			return nil, err
		}
		return s.DashboardDomain.CreateRemark(ctx, scope, req)
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
	scope, err := s.buildScope(targetUser.UnitID.Hex(), meta, role)
	if err != nil {
		return nil, err
	}
	return s.DashboardDomain.UserConvRecords(ctx, scope, userOID, targetUser, req)
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
	scope, err := s.buildScope(usr.UnitID.Hex(), meta, role)
	if err != nil {
		return nil, err
	}
	return s.DashboardDomain.GetReport(ctx, scope, convOID, usr, req)
}

// DashboardUnitConvRecords 单位/平台对话记录列表
func (s *DashboardService) DashboardUnitConvRecords(ctx context.Context, req *core_api.DashboardUnitConvRecordsReq) (*core_api.DashboardUnitConvRecordsResp, error) {
	meta, role, err := s.AuthDomain.IdentifyRole(ctx, req.GetUnitId())
	if err != nil {
		return nil, err
	}

	// 班主任需校验请求单位与本班主任所属单位一致
	if role == enum.UserRoleClassTeacher && meta.UnitId != "" && meta.UnitId != req.GetUnitId() {
		return nil, errorx.New(errno.ErrInsufficientAuth)
	}

	scope, err := s.buildScope(req.GetUnitId(), meta, role)
	if err != nil {
		return nil, err
	}
	return s.DashboardDomain.UnitConvRecords(ctx, scope, req)
}

// DashboardGetAlarmOverview 【预警】预警概览
func (s *DashboardService) DashboardGetAlarmOverview(ctx context.Context, req *core_api.DashboardGetAlarmOverviewReq) (*core_api.DashboardGetAlarmOverviewResp, error) {
	meta, role, err := s.AuthDomain.IdentifyRole(ctx, req.UnitId)
	if err != nil {
		return nil, err
	}
	scope, err := s.buildScope(req.UnitId, meta, role)
	if err != nil {
		return nil, err
	}
	return s.DashboardDomain.GetAlarmOverview(ctx, scope, req)
}

// DashboardListAlarmRecords 【预警】预警记录列表
func (s *DashboardService) DashboardListAlarmRecords(ctx context.Context, req *core_api.DashboardListAlarmRecordsReq) (*core_api.DashboardListAlarmRecordsResp, error) {
	meta, role, err := s.AuthDomain.IdentifyRole(ctx, req.UnitId)
	if err != nil {
		return nil, err
	}
	scope, err := s.buildScope(req.UnitId, meta, role)
	if err != nil {
		return nil, err
	}
	return s.DashboardDomain.ListAlarmRecords(ctx, scope, req)
}

// DashboardUpdateAlarm 【预警】更新预警（情绪/关键词/处理状态）
func (s *DashboardService) DashboardUpdateAlarm(ctx context.Context, req *core_api.DashboardUpdateAlarmReq) (*core_api.DashboardUpdateAlarmResp, error) {
	if req.Alarm == nil {
		return nil, errorx.New(errno.ErrMissingParams, errorx.KV("field", "预警信息"))
	}
	alarmId, err := bson.ObjectIDFromHex(req.Alarm.Id)
	if err != nil {
		logs.Errorf("parse alarm id error: %s", errorx.ErrorWithoutStack(err))
		return nil, errorx.New(errno.ErrInvalidParams, errorx.KV("field", "预警ID"))
	}

	// 以预警所属单位作为鉴权范围：确认调用者对目标预警单位有合法角色
	target, err := s.AlarmMapper.FindOneById(ctx, alarmId)
	if err != nil {
		logs.Errorf("find alarm error: %s", errorx.ErrorWithoutStack(err))
		return nil, errorx.New(errno.ErrNotFound)
	}
	meta, role, err := s.AuthDomain.IdentifyRole(ctx, target.UnitID.Hex())
	if err != nil {
		return nil, err
	}

	scope := &dashboard.Scope{Role: role, UserID: meta.UserId, UnitID: &target.UnitID}
	return s.DashboardDomain.UpdateAlarm(ctx, scope, req)
}
