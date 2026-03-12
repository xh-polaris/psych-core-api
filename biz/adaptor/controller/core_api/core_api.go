package core_api

import (
	"context"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
	"github.com/xh-polaris/psych-core-api/biz/adaptor/middleware"
	"github.com/xh-polaris/psych-core-api/biz/application/dto/core_api"
	"github.com/xh-polaris/psych-core-api/biz/cst"
	"github.com/xh-polaris/psych-core-api/pkg/httpx"
	"github.com/xh-polaris/psych-core-api/provider"
)

// ==========================================
// Dashboard (数据看板)
// ==========================================

// DashboardGetDataOverview
// @Summary Data Overview (数据概览)
// @Description Get overall dashboard statistics, including total units, users, active users, and conversation counts. Supports period comparison.
// @Tags Dashboard
// @Accept application/json
// @Produce application/json
// @Security ApiKeyAuth
// @Param unitId query string false "Specific Unit ID to filter. If empty, returns global admin view (if authorized)."
// @Success 200 {object} core_api.DashboardGetDataOverviewResp "Successful response with overview data"
// @Failure 400 {string} string "Invalid parameters"
// @Failure 401 {string} string "Unauthorized"
// @Failure 403 {string} string "Insufficient permissions"
// @Router /dashboard/overview [POST]
func DashboardGetDataOverview(ctx context.Context, c *app.RequestContext) {
	var err error
	var req core_api.DashboardGetDataOverviewReq
	err = c.BindAndValidate(&req)
	if err != nil {
		c.String(consts.StatusBadRequest, err.Error())
		return
	}

	middleware.StoreToken(ctx, c, &req)
	p := provider.Get()
	resp, err := p.DashboardService.DashboardGetDataOverview(ctx, &req)
	httpx.PostProcess(ctx, c, &req, resp, err)
}

// DashboardGetDataTrend
// @Summary Data Trend (数据趋势)
// @Description Get activity and conversation trends over time, including daily active users and conversation frequency.
// @Tags Dashboard
// @Accept application/json
// @Produce application/json
// @Security ApiKeyAuth
// @Param unitId query string false "Specific Unit ID to filter"
// @Success 200 {object} core_api.DashboardGetDataTrendResp "Trend data points"
// @Failure 400 {string} string "Invalid parameters"
// @Failure 401 {string} string "Unauthorized"
// @Router /dashboard/trend [POST]
func DashboardGetDataTrend(ctx context.Context, c *app.RequestContext) {
	var err error
	var req core_api.DashboardGetDataTrendReq
	err = c.BindAndValidate(&req)
	if err != nil {
		c.String(consts.StatusBadRequest, err.Error())
		return
	}

	middleware.StoreToken(ctx, c, &req)
	p := provider.Get()
	resp, err := p.DashboardService.DashboardGetDataTrend(ctx, &req)
	httpx.PostProcess(ctx, c, &req, resp, err)
}

// DashboardListUnits
// @Summary List Units (单位列表)
// @Description List all units with their respective statistics (user count, risk level, etc.). Admin only.
// @Tags Dashboard
// @Accept application/json
// @Produce application/json
// @Security ApiKeyAuth
// @Success 200 {object} core_api.DashboardListUnitsResp "List of units with stats"
// @Failure 401 {string} string "Unauthorized"
// @Failure 403 {string} string "Admin access required"
// @Router /dashboard/units [POST]
func DashboardListUnits(ctx context.Context, c *app.RequestContext) {
	var err error
	var req core_api.DashboardListUnitsReq
	err = c.BindAndValidate(&req)
	if err != nil {
		c.String(consts.StatusBadRequest, err.Error())
		return
	}

	middleware.StoreToken(ctx, c, &req)
	p := provider.Get()
	resp, err := p.DashboardService.DashboardListUnits(ctx, &req)
	httpx.PostProcess(ctx, c, &req, resp, err)
}

// DashboardGetPsychTrend
// @Summary Psych Trend (心理趋势)
// @Description Get psychological risk distribution and high-frequency keywords for a unit or globally.
// @Tags Dashboard
// @Accept application/json
// @Produce application/json
// @Security ApiKeyAuth
// @Param unitId query string false "Unit ID to filter"
// @Success 200 {object} core_api.DashboardGetPsychTrendResp "Psychological risk and keyword data"
// @Failure 400 {string} string "Invalid parameters"
// @Router /dashboard/psych_trend [POST]
func DashboardGetPsychTrend(ctx context.Context, c *app.RequestContext) {
	var err error
	var req core_api.DashboardGetPsychTrendReq
	err = c.BindAndValidate(&req)
	if err != nil {
		c.String(consts.StatusBadRequest, err.Error())
		return
	}

	middleware.StoreToken(ctx, c, &req)
	p := provider.Get()
	resp, err := p.DashboardService.DashboardGetPsychTrend(ctx, &req)
	httpx.PostProcess(ctx, c, &req, resp, err)
}

// DashboardGetAlarmOverview
// @Summary Alarm Overview (预警概览)
// @Description Get statistical overview of alarm management (high risk, processed, pending).
// @Tags Dashboard
// @Accept application/json
// @Produce application/json
// @Security ApiKeyAuth
// @Param unitId query string true "Unit ID to filter"
// @Success 200 {object} core_api.DashboardGetAlarmOverviewResp "Alarm statistics overview"
// @Failure 400 {string} string "Missing or invalid Unit ID"
// @Router /dashboard/alarm_overview [POST]
func DashboardGetAlarmOverview(ctx context.Context, c *app.RequestContext) {
	var err error
	var req core_api.DashboardGetAlarmOverviewReq
	err = c.BindAndValidate(&req)
	if err != nil {
		c.String(consts.StatusBadRequest, err.Error())
		return
	}

	middleware.StoreToken(ctx, c, &req)
	p := provider.Get()
	resp, err := p.AlarmService.Overview(ctx, &req)
	httpx.PostProcess(ctx, c, &req, resp, err)
}

// DashboardListAlarmRecords
// @Summary List Alarm Records (预警记录列表)
// @Description List psychological alarm records with filtering by emotion, status, and keywords.
// @Tags Dashboard
// @Accept application/json
// @Produce application/json
// @Security ApiKeyAuth
// @Param request body core_api.DashboardListAlarmRecordsReq true "Filtering and pagination options"
// @Success 200 {object} core_api.DashboardListAlarmRecordsResp "Paginated alarm records"
// @Failure 400 {string} string "Invalid request body"
// @Router /dashboard/alarm_records [POST]
func DashboardListAlarmRecords(ctx context.Context, c *app.RequestContext) {
	var err error
	var req core_api.DashboardListAlarmRecordsReq
	err = c.BindAndValidate(&req)
	if err != nil {
		c.String(consts.StatusBadRequest, err.Error())
		return
	}

	middleware.StoreToken(ctx, c, &req)
	p := provider.Get()
	resp, err := p.AlarmService.ListRecords(ctx, &req)
	httpx.PostProcess(ctx, c, &req, resp, err)
}

// DashboardListClasses
// @Summary List Classes (班级列表)
// @Description List classes within a unit along with user and alarm counts.
// @Tags Dashboard
// @Accept application/json
// @Produce application/json
// @Security ApiKeyAuth
// @Param unitId query string true "Unit ID"
// @Param grade query int false "Grade filter"
// @Param class query int false "Class filter"
// @Success 200 {object} core_api.DashboardListClassesResp "Hierarchical grade/class statistics"
// @Failure 400 {string} string "Invalid parameters"
// @Router /dashboard/classes [POST]
func DashboardListClasses(ctx context.Context, c *app.RequestContext) {
	var err error
	var req core_api.DashboardListClassesReq
	err = c.BindAndValidate(&req)
	if err != nil {
		c.String(consts.StatusBadRequest, err.Error())
		return
	}

	middleware.StoreToken(ctx, c, &req)
	p := provider.Get()
	resp, err := p.DashboardService.DashboardListClasses(ctx, &req)
	httpx.PostProcess(ctx, c, &req, resp, err)
}

// DashboardListUsers
// @Summary List Users (用户管理列表)
// @Description List risk users within a unit with filtering and pagination.
// @Tags Dashboard
// @Accept application/json
// @Produce application/json
// @Security ApiKeyAuth
// @Param request body core_api.DashboardListUsersReq true "Filter by level, gender, keyword and pagination"
// @Success 200 {object} core_api.DashboardListUsersResp "Paginated risk user list"
// @Failure 400 {string} string "Invalid request body"
// @Router /dashboard/users [POST]
func DashboardListUsers(ctx context.Context, c *app.RequestContext) {
	var err error
	var req core_api.DashboardListUsersReq
	err = c.BindAndValidate(&req)
	if err != nil {
		c.String(consts.StatusBadRequest, err.Error())
		return
	}

	middleware.StoreToken(ctx, c, &req)
	p := provider.Get()
	resp, err := p.DashboardService.DashboardListUsers(ctx, &req)
	httpx.PostProcess(ctx, c, &req, resp, err)
}

// ==========================================
// User (C端用户)
// ==========================================

// UserSignUp
// @Summary User Sign Up (用户注册)
// @Description Register a new end-user (e.g., student/teacher).
// @Tags User
// @Accept application/json
// @Produce application/json
// @Param request body core_api.UserSignUpReq true "User registration details"
// @Success 200 {object} core_api.UserSignUpResp "Successful registration response"
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
// @Description Login for end-users using password or verification code.
// @Tags User
// @Accept application/json
// @Produce application/json
// @Param request body core_api.UserSignInReq true "Login credentials"
// @Success 200 {object} core_api.UserSignInResp "Login success with JWT token"
// @Failure 400 {string} string "Invalid credentials"
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
// @Description Retrieve details of the currently authenticated user.
// @Tags User
// @Accept application/json
// @Produce application/json
// @Security ApiKeyAuth
// @Param userId query string true "Target User ID"
// @Success 200 {object} core_api.UserGetInfoResp "User profile information"
// @Failure 401 {string} string "Unauthorized"
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
// @Description Update profile data for the end-user.
// @Tags User
// @Accept application/json
// @Produce application/json
// @Security ApiKeyAuth
// @Param request body core_api.UserUpdateInfoReq true "Fields to update"
// @Success 200 {object} basic.Response "Status response"
// @Failure 400 {string} string "Invalid input"
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
// @Description Change the login password for an end-user.
// @Tags User
// @Accept application/json
// @Produce application/json
// @Security ApiKeyAuth
// @Param request body core_api.UserUpdatePasswordReq true "Password update details"
// @Success 200 {object} basic.Response "Status response"
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
// @Description Register a new organization/unit account.
// @Tags Unit
// @Accept application/json
// @Produce application/json
// @Param request body core_api.UnitSignUpReq true "Unit registration details"
// @Success 200 {object} core_api.UnitSignUpResp "Successful unit registration"
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
// @Description Login for unit/organization administrator accounts.
// @Tags Unit
// @Accept application/json
// @Produce application/json
// @Param request body core_api.UnitSignInReq true "Unit admin credentials"
// @Success 200 {object} core_api.UnitSignInResp "Login success"
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
// @Description Retrieve details of the specified unit.
// @Tags Unit
// @Accept application/json
// @Produce application/json
// @Security ApiKeyAuth
// @Param unitId query string true "Unit ID"
// @Success 200 {object} core_api.UnitGetInfoResp "Unit details"
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
// @Description Update organizational details for a unit.
// @Tags Unit
// @Accept application/json
// @Produce application/json
// @Security ApiKeyAuth
// @Param request body core_api.UnitUpdateInfoReq true "Fields to update"
// @Success 200 {object} basic.Response "Status response"
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
// @Description Change the login password for a unit administrator.
// @Tags Unit
// @Accept application/json
// @Produce application/json
// @Security ApiKeyAuth
// @Param request body core_api.UnitUpdatePasswordReq true "Password update details"
// @Success 200 {object} basic.Response "Status response"
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
// @Description Associate an existing user with this unit.
// @Tags Unit
// @Accept application/json
// @Produce application/json
// @Security ApiKeyAuth
// @Param request body core_api.UnitLinkUserReq true "Linking details"
// @Success 200 {object} basic.Response "Status response"
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
// @Description Batch import users and automatically link them to the unit.
// @Tags Unit
// @Accept application/json
// @Produce application/json
// @Security ApiKeyAuth
// @Param request body core_api.UnitCreateAndLinkUserReq true "Batch creation details"
// @Success 200 {object} core_api.UnitCreateAndLinkUserResp "Batch processing results"
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
// @Description Initialize or create service configurations (Chat, TTS, Report) for a unit.
// @Tags Config
// @Accept application/json
// @Produce application/json
// @Security ApiKeyAuth
// @Param request body core_api.ConfigCreateOrUpdateReq true "Configuration details"
// @Success 200 {object} basic.Response "Status response"
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
// @Description Update existing service configurations for a unit. Admin only.
// @Tags Config
// @Accept application/json
// @Produce application/json
// @Security ApiKeyAuth
// @Param request body core_api.ConfigCreateOrUpdateReq true "Configuration fields to update"
// @Success 200 {object} basic.Response "Status response"
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
// @Description Retrieve service configurations for a specific Unit ID.
// @Tags Config
// @Accept application/json
// @Produce application/json
// @Security ApiKeyAuth
// @Param unitId query string true "Unit ID"
// @Param admin query bool false "Whether to return sensitive admin-only fields (e.g., AppIDs)"
// @Success 200 {object} core_api.ConfigGetByUnitIdResp "Configuration details"
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

// DashboardUserConvRecords
// @Summary User Conversation Records (用户对话记录)
// @Description Get detailed conversation records for a specific user, including trends and details.
// @Tags Dashboard
// @Accept application/json
// @Produce application/json
// @Security ApiKeyAuth
// @Param request body core_api.DashboardUserConvRecordsReq true "User ID and pagination"
// @Success 200 {object} core_api.DashboardUserConvRecordsResp "Conversation history and analysis"
// @Router /dashboard/conversation_records [POST]
func DashboardUserConvRecords(ctx context.Context, c *app.RequestContext) {
	var err error
	var req core_api.DashboardUserConvRecordsReq
	err = c.BindAndValidate(&req)
	if err != nil {
		c.String(consts.StatusBadRequest, err.Error())
		return
	}

	middleware.StoreToken(ctx, c, &req)
	p := provider.Get()
	resp, err := p.DashboardService.DashboardUserConvRecords(ctx, &req)
	httpx.PostProcess(ctx, c, &req, resp, err)
}

// DashboardUpdateAlarm
// @Summary Update Alarm (更新预警状态)
// @Description Update the processing status or feedback for a psychological alarm record.
// @Tags Dashboard
// @Accept application/json
// @Produce application/json
// @Security ApiKeyAuth
// @Param request body core_api.DashboardUpdateAlarmReq true "Alarm ID and update details"
// @Success 200 {object} core_api.DashboardUpdateAlarmResp "Update status response"
// @Router /dashboard/update_alarm [POST]
func DashboardUpdateAlarm(ctx context.Context, c *app.RequestContext) {
	var err error
	var req core_api.DashboardUpdateAlarmReq
	err = c.BindAndValidate(&req)
	if err != nil {
		c.String(consts.StatusBadRequest, err.Error())
		return
	}

	middleware.StoreToken(ctx, c, &req)
	p := provider.Get()
	resp, err := p.AlarmService.UpdateAlarm(ctx, &req)
	httpx.PostProcess(ctx, c, &req, resp, err)
}

// DashboardGetReport
// @Summary Get Report (获取心理报告)
// @Description Retrieve a detailed psychological analysis report for a specific conversation.
// @Tags Dashboard
// @Accept application/json
// @Produce application/json
// @Security ApiKeyAuth
// @Param request body core_api.DashboardGetReportReq true "Conversation ID"
// @Success 200 {object} core_api.DashboardGetReportResp "Detailed report content"
// @Router /dashboard/get_report [POST]
func DashboardGetReport(ctx context.Context, c *app.RequestContext) {
	var err error
	var req core_api.DashboardGetReportReq
	err = c.BindAndValidate(&req)
	if err != nil {
		c.String(consts.StatusBadRequest, err.Error())
		return
	}

	middleware.StoreToken(ctx, c, &req)
	p := provider.Get()
	resp, err := p.DashboardService.DashboardGetReport(ctx, &req)
	httpx.PostProcess(ctx, c, &req, resp, err)
}

// DashboardUnitConvRecords
// @Summary Unit Conversation Records (单位对话记录总览)
// @Description Get overall conversation statistics and records for a unit. (Work in Progress)
// @Tags Dashboard
// @Accept application/json
// @Produce application/json
// @Security ApiKeyAuth
// @Param request body core_api.DashboardUnitConvRecordsReq true "Unit ID and pagination"
// @Success 200 {object} core_api.DashboardUnitConvRecordsResp "Unit-wide conversation overview"
// @Router /dashboard/unit_conversation_records [POST]
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
