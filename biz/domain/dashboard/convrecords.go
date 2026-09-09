package dashboard

import (
	"context"
	"errors"
	"sort"
	"sync"

	"github.com/xh-polaris/psych-core-api/biz/application/dto/basic"
	"github.com/xh-polaris/psych-core-api/biz/application/dto/core_api"
	"github.com/xh-polaris/psych-core-api/biz/cst"
	"github.com/xh-polaris/psych-core-api/biz/domain/his"
	"github.com/xh-polaris/psych-core-api/biz/domain/wordcld"
	"github.com/xh-polaris/psych-core-api/biz/infra/mapper/conversation"
	"github.com/xh-polaris/psych-core-api/biz/infra/mapper/user"
	"github.com/xh-polaris/psych-core-api/biz/infra/util"
	"github.com/xh-polaris/psych-core-api/pkg/errorx"
	"github.com/xh-polaris/psych-core-api/pkg/logs"
	"github.com/xh-polaris/psych-core-api/types/errno"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
)

// UserConvRecords 获取某用户对话记录（班主任需目标学生在所带班级）
func (d *DashboardDomain) UserConvRecords(ctx context.Context, scope *Scope, userOID bson.ObjectID, targetUser *user.User, req *core_api.DashboardUserConvRecordsReq) (*core_api.DashboardUserConvRecordsResp, error) {
	if scope.IsClassTeacher() {
		rs, err := d.resolveScope(ctx, scope)
		if err != nil {
			return nil, err
		}
		if err := d.ensureStudentInScope(ctx, rs, targetUser); err != nil {
			return nil, err
		}
	}
	return d.userConvRecords(ctx, userOID, targetUser, req)
}

// userConvRecords 用户对话记录（对话频率趋势 + 分页详情）
func (d *DashboardDomain) userConvRecords(ctx context.Context, userOID bson.ObjectID, targetUser *user.User, req *core_api.DashboardUserConvRecordsReq) (*core_api.DashboardUserConvRecordsResp, error) {
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

// UnitConvRecords 单位/平台对话记录列表（超管暂未实现）
func (d *DashboardDomain) UnitConvRecords(ctx context.Context, scope *Scope, req *core_api.DashboardUnitConvRecordsReq) (*core_api.DashboardUnitConvRecordsResp, error) {
	if scope.IsGlobal() {
		return nil, errorx.New(errno.UnImplementErr)
	}
	rs, err := d.resolveScope(ctx, scope)
	if err != nil {
		return nil, err
	}
	if scope.IsClassTeacher() {
		return d.unitConvRecords4ClsTch(ctx, rs, req)
	}
	return d.unitConvRecords4Unit(ctx, rs, req)
}

// unitConvRecords4Unit 单位端-获取单位下对话记录列表
func (d *DashboardDomain) unitConvRecords4Unit(ctx context.Context, rs *resolvedScope, req *core_api.DashboardUnitConvRecordsReq) (*core_api.DashboardUnitConvRecordsResp, error) {
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

// unitConvRecords4ClsTch 班主任端-获取所带班级学生的对话记录
func (d *DashboardDomain) unitConvRecords4ClsTch(ctx context.Context, rs *resolvedScope, req *core_api.DashboardUnitConvRecordsReq) (*core_api.DashboardUnitConvRecordsResp, error) {
	emptyResp := func(pg *basic.Pagination) *core_api.DashboardUnitConvRecordsResp {
		return &core_api.DashboardUnitConvRecordsResp{
			ConversationList: make([]*core_api.ConvOverview, 0),
			Pagination:       pg,
			Code:             0,
			Msg:              "success",
		}
	}

	if rs.emptyClasses() {
		return emptyResp(util.PaginationRes(0, req.PaginationOptions)), nil
	}

	unitOID := *rs.scope.UnitID
	grades, classes := rs.grades, rs.classes

	users, err := d.UserMapper.FindManyByClassList(ctx, unitOID, grades, classes)
	if err != nil {
		logs.Errorf("get users by class list error: %s", errorx.ErrorWithoutStack(err))
		return nil, errorx.New(errno.ErrNotFound, errorx.KV("field", "用户"))
	}

	if len(users) == 0 {
		return emptyResp(util.PaginationRes(0, req.PaginationOptions)), nil
	}

	userIds := make([]bson.ObjectID, len(users))
	for i, u := range users {
		userIds[i] = u.ID
	}

	total, err := d.ConversationMapper.CountByUserIds(ctx, userIds)
	if err != nil {
		return nil, errorx.New(errno.ErrDashboardGetConversations)
	}

	pg := util.PaginationRes(total, req.PaginationOptions)

	// 若对话数为 0
	if total == 0 {
		return emptyResp(pg), nil
	}

	// 查询对话列表
	convs, err := d.ConversationMapper.FindManyByUserIds(ctx, userIds, util.PagedFindOpt(req.PaginationOptions).SetSort(bson.D{{cst.EndTime, -1}}))
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
	userMap, err := d.UserMapper.BatchFindByIDs(ctx, usrIds)
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

	// 构建响应
	convOverviews := make([]*core_api.ConvOverview, 0, len(convs))
	for _, conv := range convs {
		usr := userMap[conv.UserID]
		if usr == nil {
			continue
		}
		calculatedGrade := usr.CalculateGrade(rs.startGrade)
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

// getUserConvTrend 获取用户对话趋势数据
func (d *DashboardDomain) getUserConvTrend(ctx context.Context, userOID bson.ObjectID) (*core_api.UserConvTrend, error) {
	dailyStats, err := d.ConversationMapper.CountUserDailyConv(ctx, userOID)
	if err != nil {
		logs.Errorf("get user weekly conversation stats error: %s", errorx.ErrorWithoutStack(err))
		return nil, errorx.New(errno.ErrDashboardGetUserConversationStatic)
	}

	return &core_api.UserConvTrend{
		TrendPoints: buildTrendPoints(dailyStats),
	}, nil
}

// listUserConvDetails 用户对话详情列表（摘要&词云）
func (d *DashboardDomain) listUserConvDetails(ctx context.Context, userOID bson.ObjectID, paginationOpts *basic.PaginationOptions) ([]*core_api.ConvDetail, *basic.Pagination, error) {
	// 获取对话记录
	convs, pagination, err := d.getPagedUserConvs(ctx, userOID, paginationOpts)
	if err != nil {
		return nil, nil, err
	}
	if len(convs) == 0 {
		return make([]*core_api.ConvDetail, 0), pagination, nil
	}

	tm := make(map[bson.ObjectID]int64)                // 对话时间戳
	digests := make(map[bson.ObjectID]string)          // 摘要-取自ReportMapper
	kwds := make(map[bson.ObjectID]*core_api.Keywords) // 关键词-取自词云域WordCloudExtractor

	// To Optimize：初期用户对话数，即len(convs)较小，遍历时逐个查Report即可 后续可优化为批量查询Report
	// 对每条对话记录：1.调用ReportMapper获得摘要 2.调用HisDomain获取历史消息 3.调用词云域生成词云
	var wg sync.WaitGroup
	var tmMu, dgstMu, kwdsMu sync.Mutex
	wg.Add(len(convs))

	for _, conv := range convs {
		go func(c *conversation.Conversation) {
			defer wg.Done()
			// 每个routine处理一条对话记录
			// 填充时间
			tmMu.Lock()
			tm[c.ID] = c.StartTime.Unix()
			tmMu.Unlock()

			// 获取摘要
			rpt, err := d.ReportMapper.FindByConversationPreferSuccess(ctx, c.ID)
			if err != nil {
				// 报表不存在，可能还未完成创建
				if errors.Is(err, mongo.ErrNoDocuments) {
					dgstMu.Lock()
					digests[c.ID] = "暂无摘要"
					dgstMu.Unlock()
				} else {
					// 意外错误
					// 这里不直接返回 继续尝试生成词云
					logs.Errorf("get report error: %s", errorx.ErrorWithoutStack(err))
				}
			} else {
				// 报表存在，正常填入摘要
				dgstMu.Lock()
				digests[c.ID] = rpt.Digest
				dgstMu.Unlock()
			}
			// 获取所有对话历史消息
			msgHis, err := his.Mgr.RetrieveMessage(ctx, c.ID.Hex(), -1)
			if err != nil || len(msgHis) == 0 {
				logs.Errorf("retrieve history messages error: %s", errorx.ErrorWithoutStack(err))
				kwdsMu.Lock()
				kwds[c.ID] = &core_api.Keywords{
					KeywordMap: make(map[string]int32),
					KeyTotal:   0,
				}
				kwdsMu.Unlock()
				return
			}
			// 生成词云
			wc, err := wordcld.Extractor.FromHisMsg(msgHis)
			if err != nil {
				logs.Errorf("word cloud extractor error: %s", errorx.ErrorWithoutStack(err))
				kwdsMu.Lock()
				kwds[c.ID] = &core_api.Keywords{
					KeywordMap: make(map[string]int32),
					KeyTotal:   0,
				}
				kwdsMu.Unlock()
				return
			}

			kwdsMu.Lock()
			kwds[c.ID] = wc
			kwdsMu.Unlock()
		}(conv)
	}

	wg.Wait()

	// 构造响应中的convDetails列表
	convDetails := make([]*core_api.ConvDetail, 0, len(convs))

	for convId, convTime := range tm {
		convDetail := &core_api.ConvDetail{
			ConversationId: convId.Hex(),
			Time:           convTime,
			Digest:         "",
			Keywords:       &core_api.Keywords{},
		}
		if dgst, ok := digests[convId]; ok {
			convDetail.Digest = dgst
		}
		if kwd, ok := kwds[convId]; ok {
			convDetail.Keywords = kwd
		}
		convDetails = append(convDetails, convDetail)
	}

	// 页内按照时间新-旧排序
	sort.Slice(convDetails, func(i, j int) bool {
		return convDetails[i].Time > convDetails[j].Time // 时间戳降序排序，即最新的在前
	})

	return convDetails, pagination, nil
}

// getPagedUserConvs 获取用户对话记录（分页）
// 返回分页范围内的Conversation和分页参数
func (d *DashboardDomain) getPagedUserConvs(ctx context.Context, userOID bson.ObjectID, paginationOpts *basic.PaginationOptions) ([]*conversation.Conversation, *basic.Pagination, error) {
	convs, err := d.ConversationMapper.FindAllByUserId(ctx, userOID) // 已按对话时间排序
	if err != nil {
		logs.Errorf("get user convs error: %s", errorx.ErrorWithoutStack(err))
		return nil, nil, errorx.New(errno.ErrDashboardGetConversations)
	}
	total := int32(len(convs))

	startIdx, endIdx := util.PagedIndex(total, paginationOpts)

	// 返回分页范围内的Conversation
	pagedConvs := convs[startIdx:endIdx]
	pagination := util.PaginationRes(total, paginationOpts)

	return pagedConvs, pagination, nil
}
