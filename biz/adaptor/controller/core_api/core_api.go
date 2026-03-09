package core_api

import (
	"context"
	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
	"github.com/xh-polaris/psych-core-api/biz/application/dto/core_api"
	"github.com/xh-polaris/psych-core-api/biz/cst"
	"github.com/xh-polaris/psych-core-api/pkg/httpx"
	"github.com/xh-polaris/psych-core-api/provider"
	//"github.com/xh-polaris/psych-idl/kitex_gen/core_api"
)

// ==========================================
// Dashboard (数据看板)
// ==========================================

// DashboardGetDataOverview
// @Summary Data Overview (数据概览)
// @Description 获取数据看板的总体概览数据
// @Tags Dashboard
// @Accept application/json
// @Produce application/json
// @Param unitId query string false "Unit ID"
// @Success 200 {object} core_api.DashboardGetDataOverviewResp
// @Failure 400 {string} string "Bad Request"
// @Router /dashboard/overview [POST]
func DashboardGetDataOverview(ctx context.Context, c *app.RequestContext) {
	var err error
	var req core_api.DashboardGetDataOverviewReq
	err = c.BindAndValidate(&req)
	if err != nil {
		c.String(consts.StatusBadRequest, err.Error())
		return
	}

	p := provider.Get()
	resp, err := p.DashboardService.DashboardGetDataOverview(ctx, &req)
	httpx.PostProcess(ctx, c, &req, resp, err)
}

// DashboardGetDataTrend
// @Summary Data Trend (数据趋势)
// @Description 获取活跃度和对话数据的趋势图表
// @Tags Dashboard
// @Accept application/json
// @Produce application/json
// @Param unitId query string false "Unit ID"
// @Success 200 {object} core_api.DashboardGetDataTrendResp
// @Failure 400 {string} string "Bad Request"
// @Router /dashboard/trend [POST]
func DashboardGetDataTrend(ctx context.Context, c *app.RequestContext) {
	var err error
	var req core_api.DashboardGetDataTrendReq
	err = c.BindAndValidate(&req)
	if err != nil {
		c.String(consts.StatusBadRequest, err.Error())
		return
	}

	p := provider.Get()
	resp, err := p.DashboardService.DashboardGetDataTrend(ctx, &req)
	httpx.PostProcess(ctx, c, &req, resp, err)
}

// DashboardListUnits
// @Summary List Units (单位列表)
// @Description 获取数据看板中的单位列表统计信息
// @Tags Dashboard
// @Accept application/json
// @Produce application/json
// @Success 200 {object} core_api.DashboardListUnitsResp
// @Failure 400 {string} string "Bad Request"
// @Router /dashboard/units [POST]
func DashboardListUnits(ctx context.Context, c *app.RequestContext) {
	var err error
	var req core_api.DashboardListUnitsReq
	err = c.BindAndValidate(&req)
	if err != nil {
		c.String(consts.StatusBadRequest, err.Error())
		return
	}

	p := provider.Get()
	resp, err := p.DashboardService.DashboardListUnits(ctx, &req)
	httpx.PostProcess(ctx, c, &req, resp, err)
}

// DashboardGetPsychTrend
// @Summary Psych Trend (心理趋势)
// @Description 获取心理风险分布和高频关键词
// @Tags Dashboard
// @Accept application/json
// @Produce application/json
// @Param unitId query string true "Unit ID"
// @Success 200 {object} core_api.DashboardGetPsychTrendResp
// @Failure 400 {string} string "Bad Request"
// @Router /dashboard/psych_trend [POST]
func DashboardGetPsychTrend(ctx context.Context, c *app.RequestContext) {
	var err error
	var req core_api.DashboardGetPsychTrendReq
	err = c.BindAndValidate(&req)
	if err != nil {
		c.String(consts.StatusBadRequest, err.Error())
		return
	}

	p := provider.Get()
	resp, err := p.DashboardService.DashboardGetPsychTrend(ctx, &req)
	httpx.PostProcess(ctx, c, &req, resp, err)
}

// DashboardGetAlarmOverview
// @Summary Alarm Overview (预警概览)
// @Description 获取预警管理的统计概览（高危、已处理、待处理等）
// @Tags Dashboard
// @Accept application/json
// @Produce application/json
// @Param unitId query string true "Unit ID"
// @Success 200 {object} core_api.DashboardGetAlarmOverviewResp
// @Failure 400 {string} string "Bad Request"
// @Router /dashboard/alarm_overview [POST]
func DashboardGetAlarmOverview(ctx context.Context, c *app.RequestContext) {
	var err error
	var req core_api.DashboardGetAlarmOverviewReq
	err = c.BindAndValidate(&req)
	if err != nil {
		c.String(consts.StatusBadRequest, err.Error())
		return
	}

	p := provider.Get()
	resp, err := p.AlarmService.Overview(ctx, &req)
	httpx.PostProcess(ctx, c, &req, resp, err)
}

// DashboardListAlarmRecords
// @Summary List Alarm Records (预警记录)
// @Description 获取预警记录列表，支持筛选和分页
// @Tags Dashboard
// @Accept application/json
// @Produce application/json
// @Param unitId query string true "Unit ID"
// @Param emotion query string false "Emotion"
// @Param status query string false "Status"
// @Param keyword query string false "Keyword"
// @Param page query int false "Page Number"
// @Param size query int false "Page Size"
// @Success 200 {object} core_api.DashboardListAlarmRecordsResp
// @Failure 400 {string} string "Bad Request"
// @Router /dashboard/alarm_records [POST]
func DashboardListAlarmRecords(ctx context.Context, c *app.RequestContext) {
	var err error
	var req core_api.DashboardListAlarmRecordsReq
	err = c.BindAndValidate(&req)
	if err != nil {
		c.String(consts.StatusBadRequest, err.Error())
		return
	}

	p := provider.Get()
	resp, err := p.AlarmService.ListRecords(ctx, &req)
	httpx.PostProcess(ctx, c, &req, resp, err)
}

// DashboardListClasses
// @Summary List Classes (班级列表)
// @Description 获取班级列表及班级内的用户/预警统计
// @Tags Dashboard
// @Accept application/json
// @Produce application/json
// @Param unitId query string true "Unit ID"
// @Param grade query int false "Grade"
// @Param class query int false "Class"
// @Success 200 {object} core_api.DashboardListClassesResp
// @Failure 400 {string} string "Bad Request"
// @Router /dashboard/classes [POST]
func DashboardListClasses(ctx context.Context, c *app.RequestContext) {
	var err error
	var req core_api.DashboardListClassesReq
	err = c.BindAndValidate(&req)
	if err != nil {
		c.String(consts.StatusBadRequest, err.Error())
		return
	}

	p := provider.Get()
	resp, err := p.DashboardService.DashboardListClasses(ctx, &req)
	httpx.PostProcess(ctx, c, &req, resp, err)
}

// DashboardListUsers
// @Summary List Users (用户列表)
// @Description 获取风险用户列表，支持筛选和分页
// @Tags Dashboard
// @Accept application/json
// @Produce application/json
// @Param unitId query string true "Unit ID"
// @Param level query int false "Risk Level"
// @Param gender query string false "Gender"
// @Param keyword query string false "Keyword"
// @Param page query int false "Page Number"
// @Param size query int false "Page Size"
// @Success 200 {object} core_api.DashboardListUsersResp
// @Failure 400 {string} string "Bad Request"
// @Router /dashboard/users [POST]
func DashboardListUsers(ctx context.Context, c *app.RequestContext) {
	var err error
	var req core_api.DashboardListUsersReq
	err = c.BindAndValidate(&req)
	if err != nil {
		c.String(consts.StatusBadRequest, err.Error())
		return
	}

	p := provider.Get()
	resp, err := p.DashboardService.DashboardListUsers(ctx, &req)
	httpx.PostProcess(ctx, c, &req, resp, err)
}

// ==========================================
// User (C端用户)
// ==========================================

// UserSignUp
// @Summary User Sign Up (用户注册)
// @Description C端用户注册
// @Tags User
// @Accept application/json
// @Produce application/json
// @Param request body core_api.UserSignUpReq true "Register Request"
// @Success 200 {object} core_api.UserSignUpResp
// @Failure 400 {string} string "Bad Request"
// @Router /user/sign_up [POST]
func UserSignUp(ctx context.Context, c *app.RequestContext) {
	var err error
	var req core_api.UserSignUpReq
	err = c.BindAndValidate(&req)
	if err != nil {
		c.String(consts.StatusBadRequest, err.Error())
		return
	}

	p := provider.Get()
	resp, err := p.UserService.UserSignUp(ctx, &req)
	httpx.PostProcess(ctx, c, &req, resp, err)
}

// UserSignIn
// @Summary User Sign In (用户登录)
// @Description C端用户登录
// @Tags User
// @Accept application/json
// @Produce application/json
// @Param request body core_api.UserSignInReq true "Login Request"
// @Success 200 {object} core_api.UserSignInResp
// @Failure 400 {string} string "Bad Request"
// @Router /user/sign_in [POST]
func UserSignIn(ctx context.Context, c *app.RequestContext) {
	var err error
	var req core_api.UserSignInReq
	err = c.BindAndValidate(&req)
	if err != nil {
		c.String(consts.StatusBadRequest, err.Error())
		return
	}

	p := provider.Get()
	resp, err := p.UserService.UserSignIn(ctx, &req)
	httpx.PostProcess(ctx, c, &req, resp, err)
}

// UserGetInfo
// @Summary Get User Info (获取用户信息)
// @Description 获取当前登录用户的详细信息
// @Tags User
// @Accept application/json
// @Produce application/json
// @Param userId query string true "User ID"
// @Success 200 {object} core_api.UserGetInfoResp
// @Failure 400 {string} string "Bad Request"
// @Router /user/get_info [GET]
func UserGetInfo(ctx context.Context, c *app.RequestContext) {
	var err error
	var req core_api.UserGetInfoReq
	req.UserId = c.Query(cst.QueryUserID)

	p := provider.Get()
	resp, err := p.UserService.UserGetInfo(ctx, &req)
	httpx.PostProcess(ctx, c, &req, resp, err)
}

// UserUpdateInfo
// @Summary Update User Info (更新用户信息)
// @Description 更新用户个人资料
// @Tags User
// @Accept application/json
// @Produce application/json
// @Param request body core_api.UserUpdateInfoReq true "Update Info Request"
// @Success 200 {object} basic.Response
// @Failure 400 {string} string "Bad Request"
// @Router /user/update_info [POST]
func UserUpdateInfo(ctx context.Context, c *app.RequestContext) {
	var err error
	var req core_api.UserUpdateInfoReq
	err = c.BindAndValidate(&req)
	if err != nil {
		c.String(consts.StatusBadRequest, err.Error())
		return
	}

	p := provider.Get()
	resp, err := p.UserService.UserUpdateInfo(ctx, &req)
	httpx.PostProcess(ctx, c, &req, resp, err)
}

// UserUpdatePassword
// @Summary Update User Password (修改用户密码)
// @Description 修改用户登录密码
// @Tags User
// @Accept application/json
// @Produce application/json
// @Param request body core_api.UserUpdatePasswordReq true "Update Password Request"
// @Success 200 {object} basic.Response
// @Failure 400 {string} string "Bad Request"
// @Router /user/update_password [POST]
func UserUpdatePassword(ctx context.Context, c *app.RequestContext) {
	var err error
	var req core_api.UserUpdatePasswordReq
	err = c.BindAndValidate(&req)
	if err != nil {
		c.String(consts.StatusBadRequest, err.Error())
		return
	}

	p := provider.Get()
	resp, err := p.UserService.UserUpdatePassword(ctx, &req)
	httpx.PostProcess(ctx, c, &req, resp, err)
}

// ==========================================
// Unit (B端/单位)
// ==========================================

// UnitSignUp
// @Summary Unit Sign Up (单位注册)
// @Description B端单位账号注册
// @Tags Unit
// @Accept application/json
// @Produce application/json
// @Param request body core_api.UnitSignUpReq true "Register Request"
// @Success 200 {object} core_api.UnitSignUpResp
// @Failure 400 {string} string "Bad Request"
// @Router /unit/sign_up [POST]
func UnitSignUp(ctx context.Context, c *app.RequestContext) {
	var err error
	var req core_api.UnitSignUpReq
	err = c.BindAndValidate(&req)
	if err != nil {
		c.String(consts.StatusBadRequest, err.Error())
		return
	}

	p := provider.Get()
	resp, err := p.UnitService.UnitSignUp(ctx, &req)
	httpx.PostProcess(ctx, c, &req, resp, err)
}

// UnitSignIn
// @Summary Unit Sign In (单位登录)
// @Description B端单位账号登录
// @Tags Unit
// @Accept application/json
// @Produce application/json
// @Param request body core_api.UnitSignInReq true "Login Request"
// @Success 200 {object} core_api.UnitSignInResp
// @Failure 400 {string} string "Bad Request"
// @Router /unit/sign_in [POST]
func UnitSignIn(ctx context.Context, c *app.RequestContext) {
	var err error
	var req core_api.UnitSignInReq
	err = c.BindAndValidate(&req)
	if err != nil {
		c.String(consts.StatusBadRequest, err.Error())
		return
	}

	p := provider.Get()
	resp, err := p.UnitService.UnitSignIn(ctx, &req)
	httpx.PostProcess(ctx, c, &req, resp, err)
}

// UnitGetInfo
// @Summary Get Unit Info (获取单位信息)
// @Description 获取当前单位的详细信息
// @Tags Unit
// @Accept application/json
// @Produce application/json
// @Param unitId query string true "Unit ID"
// @Success 200 {object} core_api.UnitGetInfoResp
// @Failure 400 {string} string "Bad Request"
// @Router /unit/get_info [GET]
func UnitGetInfo(ctx context.Context, c *app.RequestContext) {
	var err error
	var req core_api.UnitGetInfoReq
	req.UnitId = c.Query(cst.QueryUnitID)

	p := provider.Get()
	resp, err := p.UnitService.UnitGetInfo(ctx, &req)
	httpx.PostProcess(ctx, c, &req, resp, err)
}

// UnitUpdateInfo
// @Summary Update Unit Info (更新单位信息)
// @Description 更新单位资料
// @Tags Unit
// @Accept application/json
// @Produce application/json
// @Param request body core_api.UnitUpdateInfoReq true "Update Info Request"
// @Success 200 {object} basic.Response
// @Failure 400 {string} string "Bad Request"
// @Router /unit/update_info [POST]
func UnitUpdateInfo(ctx context.Context, c *app.RequestContext) {
	var err error
	var req core_api.UnitUpdateInfoReq
	err = c.BindAndValidate(&req)
	if err != nil {
		c.String(consts.StatusBadRequest, err.Error())
		return
	}

	p := provider.Get()
	resp, err := p.UnitService.UnitUpdateInfo(ctx, &req)
	httpx.PostProcess(ctx, c, &req, resp, err)
}

// UnitUpdatePassword
// @Summary Update Unit Password (修改单位密码)
// @Description 修改单位登录密码
// @Tags Unit
// @Accept application/json
// @Produce application/json
// @Param request body core_api.UnitUpdatePasswordReq true "Update Password Request"
// @Success 200 {object} basic.Response
// @Failure 400 {string} string "Bad Request"
// @Router /unit/update_password [POST]
func UnitUpdatePassword(ctx context.Context, c *app.RequestContext) {
	var err error
	var req core_api.UnitUpdatePasswordReq
	err = c.BindAndValidate(&req)
	if err != nil {
		c.String(consts.StatusBadRequest, err.Error())
		return
	}

	p := provider.Get()
	resp, err := p.UnitService.UnitUpdatePassword(ctx, &req)
	httpx.PostProcess(ctx, c, &req, resp, err)
}

// UnitLinkUser
// @Summary Link User (关联用户)
// @Description 将现有用户关联到本单位
// @Tags Unit
// @Accept application/json
// @Produce application/json
// @Param request body core_api.UnitLinkUserReq true "Link Request"
// @Success 200 {object} basic.Response
// @Failure 400 {string} string "Bad Request"
// @Router /unit/link_user [POST]
func UnitLinkUser(ctx context.Context, c *app.RequestContext) {
	var err error
	var req core_api.UnitLinkUserReq
	err = c.BindAndValidate(&req)
	if err != nil {
		c.String(consts.StatusBadRequest, err.Error())
		return
	}

	p := provider.Get()
	resp, err := p.UnitService.UnitLinkUser(ctx, &req)
	httpx.PostProcess(ctx, c, &req, resp, err)
}

// UnitCreateAndLinkUser
// @Summary Create And Link Users (批量创建并关联用户)
// @Description 批量导入用户并自动关联到本单位
// @Tags Unit
// @Accept application/json
// @Produce application/json
// @Param request body core_api.UnitCreateAndLinkUserReq true "Create And Link Request"
// @Success 200 {object} core_api.UnitCreateAndLinkUserResp
// @Failure 400 {string} string "Bad Request"
// @Router /unit/create_and_link_user [POST]
func UnitCreateAndLinkUser(ctx context.Context, c *app.RequestContext) {
	var err error
	var req core_api.UnitCreateAndLinkUserReq
	err = c.BindAndValidate(&req)
	if err != nil {
		c.String(consts.StatusBadRequest, err.Error())
		return
	}

	p := provider.Get()
	resp, err := p.UnitService.UnitCreateAndLinkUser(ctx, &req)
	httpx.PostProcess(ctx, c, &req, resp, err)
}

// ==========================================
// Config (配置管理)
// ==========================================

// ConfigCreate
// @Summary Create Config (创建配置)
// @Description 创建或初始化单位配置
// @Tags Config
// @Accept application/json
// @Produce application/json
// @Param request body core_api.ConfigCreateOrUpdateReq true "Config Request"
// @Success 200 {object} basic.Response
// @Failure 400 {string} string "Bad Request"
// @Router /config/create [POST]
func ConfigCreate(ctx context.Context, c *app.RequestContext) {
	var err error
	var req core_api.ConfigCreateOrUpdateReq
	err = c.BindAndValidate(&req)
	if err != nil {
		c.String(consts.StatusBadRequest, err.Error())
		return
	}

	p := provider.Get()
	resp, err := p.ConfigService.ConfigCreate(ctx, &req)
	httpx.PostProcess(ctx, c, &req, resp, err)
}

// ConfigUpdateInfo
// @Summary Update Config (更新配置)
// @Description 更新单位的配置信息（如Chat/TTS/Report配置）
// @Tags Config
// @Accept application/json
// @Produce application/json
// @Param request body core_api.ConfigCreateOrUpdateReq true "Config Request"
// @Success 200 {object} basic.Response
// @Failure 400 {string} string "Bad Request"
// @Router /config/update_info [POST]
func ConfigUpdateInfo(ctx context.Context, c *app.RequestContext) {
	var err error
	var req core_api.ConfigCreateOrUpdateReq
	err = c.BindAndValidate(&req)
	if err != nil {
		c.String(consts.StatusBadRequest, err.Error())
		return
	}

	p := provider.Get()
	resp, err := p.ConfigService.ConfigUpdateInfo(ctx, &req)
	httpx.PostProcess(ctx, c, &req, resp, err)
}

// ConfigGetByUnitID
// @Summary Get Config (获取配置)
// @Description 根据 Unit ID 获取配置详情
// @Tags Config
// @Accept application/json
// @Produce application/json
// @Param unitId query string true "Unit ID"
// @Param admin query bool false "Is Admin"
// @Success 200 {object} core_api.ConfigGetByUnitIdResp
// @Failure 400 {string} string "Bad Request"
// @Router /config/get_by_unit_id [GET]
func ConfigGetByUnitID(ctx context.Context, c *app.RequestContext) {
	var err error
	var req core_api.ConfigGetByUnitIdReq
	err = c.BindAndValidate(&req)
	if err != nil {
		c.String(consts.StatusBadRequest, err.Error())
		return
	}

	p := provider.Get()
	resp, err := p.ConfigService.ConfigGetByUnitID(ctx, &req)
	httpx.PostProcess(ctx, c, &req, resp, err)
}

// DashboardUserConvRecords .
// @router /dashboard/conversation_records [POST]
func DashboardUserConvRecords(ctx context.Context, c *app.RequestContext) {
	var err error
	var req core_api.DashboardUserConvRecordsReq
	err = c.BindAndValidate(&req)
	if err != nil {
		c.String(consts.StatusBadRequest, err.Error())
		return
	}

	p := provider.Get()
	resp, err := p.DashboardService.DashboardUserConvRecords(ctx, &req)
	httpx.PostProcess(ctx, c, &req, resp, err)
}

// DashboardUpdateAlarm .
// @router /dashboard/update_alarm [POST]
func DashboardUpdateAlarm(ctx context.Context, c *app.RequestContext) {
	var err error
	var req core_api.DashboardUpdateAlarmReq
	err = c.BindAndValidate(&req)
	if err != nil {
		c.String(consts.StatusBadRequest, err.Error())
		return
	}

	p := provider.Get()
	resp, err := p.AlarmService.UpdateAlarm(ctx, &req)
	httpx.PostProcess(ctx, c, &req, resp, err)
}

// DashboardGetReport .
// @router /dashboard/get_report [POST]
func DashboardGetReport(ctx context.Context, c *app.RequestContext) {
	var err error
	var req core_api.DashboardGetReportReq
	err = c.BindAndValidate(&req)
	if err != nil {
		c.String(consts.StatusBadRequest, err.Error())
		return
	}

	p := provider.Get()
	resp, err := p.DashboardService.DashboardGetReport(ctx, &req)
	httpx.PostProcess(ctx, c, &req, resp, err)
}

// DashboardUnitConvRecords .
// @router /dashboard/unit_conversation_records [POST]
func DashboardUnitConvRecords(ctx context.Context, c *app.RequestContext) {
	var err error
	var req core_api.DashboardUnitConvRecordsReq
	err = c.BindAndValidate(&req)
	if err != nil {
		c.String(consts.StatusBadRequest, err.Error())
		return
	}

	resp := new(core_api.DashboardUnitConvRecordsResp)

	c.JSON(consts.StatusOK, resp)
}
