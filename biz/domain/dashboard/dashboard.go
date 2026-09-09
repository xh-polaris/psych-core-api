package dashboard

import (
	"context"
	"time"

	"github.com/google/wire"
	"github.com/xh-polaris/psych-core-api/biz/application/dto/core_api"
	"github.com/xh-polaris/psych-core-api/biz/infra/mapper/alarm"
	"github.com/xh-polaris/psych-core-api/biz/infra/mapper/conversation"
	"github.com/xh-polaris/psych-core-api/biz/infra/mapper/report"
	"github.com/xh-polaris/psych-core-api/biz/infra/mapper/unit"
	"github.com/xh-polaris/psych-core-api/biz/infra/mapper/user"
	"go.mongodb.org/mongo-driver/v2/bson"
)

// IDashboardDomain 数据看板领域接口：由 service 层从鉴权结果构造 Scope，
// 领域内部按 Scope 解析数据范围（超管全局 / 单位管理员本单位 / 班主任所带班级）。
type IDashboardDomain interface {
	// 指标总览
	GetDataOverview(ctx context.Context, scope *Scope, start, end time.Time) (*core_api.DashboardGetDataOverviewResp, error)
	// 数据趋势
	GetDataTrend(ctx context.Context, scope *Scope, start, end time.Time) (*core_api.DashboardGetDataTrendResp, error)
	// 心理趋势
	GetPsychTrend(ctx context.Context, scope *Scope, start, end time.Time) (*core_api.DashboardGetPsychTrendResp, error)

	// 单位列表（超管）
	ListUnits(ctx context.Context) (*core_api.DashboardListUnitsResp, error)

	// 用户管理
	ListClasses(ctx context.Context, scope *Scope, req *core_api.DashboardListClassesReq) (*core_api.DashboardListClassesResp, error)
	ListUsers(ctx context.Context, scope *Scope, req *core_api.DashboardListUsersReq) (*core_api.DashboardListUsersResp, error)
	CreateRemark(ctx context.Context, scope *Scope, req *core_api.DashboardCreateRemarkReq) (*core_api.DashboardCreateRemarkResp, error)

	// 对话记录 / 报表
	UserConvRecords(ctx context.Context, scope *Scope, userOID bson.ObjectID, targetUser *user.User, req *core_api.DashboardUserConvRecordsReq) (*core_api.DashboardUserConvRecordsResp, error)
	UnitConvRecords(ctx context.Context, scope *Scope, req *core_api.DashboardUnitConvRecordsReq) (*core_api.DashboardUnitConvRecordsResp, error)
	GetReport(ctx context.Context, scope *Scope, convOID bson.ObjectID, targetUser *user.User, req *core_api.DashboardGetReportReq) (*core_api.DashboardGetReportResp, error)

	// 预警
	GetAlarmOverview(ctx context.Context, scope *Scope, req *core_api.DashboardGetAlarmOverviewReq) (*core_api.DashboardGetAlarmOverviewResp, error)
	ListAlarmRecords(ctx context.Context, scope *Scope, req *core_api.DashboardListAlarmRecordsReq) (*core_api.DashboardListAlarmRecordsResp, error)
	UpdateAlarm(ctx context.Context, scope *Scope, req *core_api.DashboardUpdateAlarmReq) (*core_api.DashboardUpdateAlarmResp, error)
}

// DashboardDomain 数据看板领域实现
type DashboardDomain struct {
	UserMapper         user.IMongoMapper
	UnitMapper         unit.IMongoMapper
	ConversationMapper conversation.IMongoMapper
	ReportMapper       report.IMongoMapper
	AlarmMapper        alarm.IMongoMapper
}

var DashboardDomainSet = wire.NewSet(
	wire.Struct(new(DashboardDomain), "*"),
	wire.Bind(new(IDashboardDomain), new(*DashboardDomain)),
)
