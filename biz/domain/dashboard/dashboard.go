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

// IDashboardDomain 数据看板领域接口：按角色拆分为超管/单位管理/班主任三组分支
type IDashboardDomain interface {
	// 指标总览
	GetDataOverview4Admin(ctx context.Context, start, end time.Time) (*core_api.DashboardGetDataOverviewResp, error)
	GetDataOverview4Unit(ctx context.Context, unitOID bson.ObjectID, start, end time.Time) (*core_api.DashboardGetDataOverviewResp, error)
	GetDataOverview4ClsTch(ctx context.Context, userId string, unitOID bson.ObjectID, start, end time.Time) (*core_api.DashboardGetDataOverviewResp, error)

	// 数据趋势
	GetDataTrend4Admin(ctx context.Context, start, end time.Time) (*core_api.DashboardGetDataTrendResp, error)
	GetDataTrend4Unit(ctx context.Context, unitOID bson.ObjectID, start, end time.Time) (*core_api.DashboardGetDataTrendResp, error)
	GetDataTrend4ClsTch(ctx context.Context, userId string, unitOID bson.ObjectID, start, end time.Time) (*core_api.DashboardGetDataTrendResp, error)

	// 心理趋势
	GetPsychTrend4Admin(ctx context.Context, start, end time.Time) (*core_api.DashboardGetPsychTrendResp, error)
	GetPsychTrend4Unit(ctx context.Context, unitOID bson.ObjectID, start, end time.Time) (*core_api.DashboardGetPsychTrendResp, error)
	GetPsychTrend4ClsTch(ctx context.Context, userId string, unitOID bson.ObjectID, start, end time.Time) (*core_api.DashboardGetPsychTrendResp, error)

	// 单位列表（超管）
	ListUnits(ctx context.Context) (*core_api.DashboardListUnitsResp, error)

	// 用户管理
	ListClasses4Unit(ctx context.Context, unitOID bson.ObjectID, req *core_api.DashboardListClassesReq) (*core_api.DashboardListClassesResp, error)
	ListClasses4ClsTch(ctx context.Context, userId string, unitOID bson.ObjectID, req *core_api.DashboardListClassesReq) (*core_api.DashboardListClassesResp, error)
	ListUsers4Unit(ctx context.Context, unitOID bson.ObjectID, req *core_api.DashboardListUsersReq) (*core_api.DashboardListUsersResp, error)
	ListUsers4ClsTch(ctx context.Context, userId string, unitOID bson.ObjectID, req *core_api.DashboardListUsersReq) (*core_api.DashboardListUsersResp, error)
	CreateRemark4Unit(ctx context.Context, req *core_api.DashboardCreateRemarkReq) (*core_api.DashboardCreateRemarkResp, error)
	CreateRemark4ClsTch(ctx context.Context, teacherId string, req *core_api.DashboardCreateRemarkReq) (*core_api.DashboardCreateRemarkResp, error)

	// 对话记录
	UserConvRecords4Unit(ctx context.Context, userOID bson.ObjectID, targetUser *user.User, req *core_api.DashboardUserConvRecordsReq) (*core_api.DashboardUserConvRecordsResp, error)
	UserConvRecords4ClsTch(ctx context.Context, teacherId string, userOID bson.ObjectID, targetUser *user.User, req *core_api.DashboardUserConvRecordsReq) (*core_api.DashboardUserConvRecordsResp, error)
	GetReport4Unit(ctx context.Context, convOID bson.ObjectID, req *core_api.DashboardGetReportReq) (*core_api.DashboardGetReportResp, error)
	GetReport4ClsTch(ctx context.Context, teacherId string, convOID bson.ObjectID, targetUser *user.User, req *core_api.DashboardGetReportReq) (*core_api.DashboardGetReportResp, error)
	UnitConvRecords4Admin(ctx context.Context, req *core_api.DashboardUnitConvRecordsReq) (*core_api.DashboardUnitConvRecordsResp, error)
	UnitConvRecords4Unit(ctx context.Context, req *core_api.DashboardUnitConvRecordsReq) (*core_api.DashboardUnitConvRecordsResp, error)
	UnitConvRecords4ClsTch(ctx context.Context, userId string, unitOID bson.ObjectID, req *core_api.DashboardUnitConvRecordsReq) (*core_api.DashboardUnitConvRecordsResp, error)
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
