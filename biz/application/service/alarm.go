package service

import (
	"context"
	"github.com/google/wire"
	"github.com/xh-polaris/psych-core-api/biz/infra/mapper/alarm"
	"github.com/xh-polaris/psych-core-api/biz/infra/rpc"
	"github.com/xh-polaris/psych-idl/kitex_gen/core_api"
)

type IAlarmService interface {
	Overview(ctx context.Context, req *core_api.DashboardGetAlarmOverviewReq) (resp *core_api.DashboardGetAlarmOverviewResp, err error)
	ListRecords(ctx context.Context, req *core_api.DashboardListAlarmRecordsReq) (resp *core_api.DashboardListAlarmRecordsResp, err error)
}

type AlarmService struct {
	AlarmMapper alarm.IMongoMapper
}

var AlarmServiceSet = wire.NewSet(
	wire.Struct(new(AlarmService), "*"),
	wire.Bind(new(IAlarmService), new(*AlarmService)),
)

func (s *AlarmService) Overview(ctx context.Context, req *core_api.DashboardGetAlarmOverviewReq) (resp *core_api.DashboardGetAlarmOverviewResp, err error) {
	rpc.GetPsychProfile()
	return nil, nil
}

func (s *AlarmService) ListRecords(ctx context.Context, req *core_api.DashboardListAlarmRecordsReq) (resp *core_api.DashboardListAlarmRecordsResp, err error) {
	return nil, nil
}
