package service

import (
	"context"
	"github.com/google/wire"
	"github.com/xh-polaris/psych-core-api/biz/infra/rpc"
	"github.com/xh-polaris/psych-idl/kitex_gen/core_api"
)

type IDashboardService interface {
	ListClasses(ctx context.Context, req *core_api.DashboardListClassesReq) (resp *core_api.DashboardListClassesResp, err error)
	ListUsers(ctx context.Context, req *core_api.DashboardListUsersReq) (resp *core_api.DashboardListUsersResp, err error)
}

type DashboardService struct{}

var DashboardServiceSet = wire.NewSet(
	wire.Struct(new(DashboardService), "*"),
	wire.Bind(new(IDashboardService), new(*DashboardService)),
)

func (s *DashboardService) ListClasses(ctx context.Context, req *core_api.DashboardListClassesReq) (resp *core_api.DashboardListClassesResp, err error) {
	rpc.GetPsychProfile()
	return nil, nil
}

func (s *DashboardService) ListUsers(ctx context.Context, req *core_api.DashboardListUsersReq) (resp *core_api.DashboardListUsersResp, err error) {
	return nil, nil
}
