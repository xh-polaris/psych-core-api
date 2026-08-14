package dashboard

import (
	"context"
	"errors"
	"sort"
	"sync"
	"time"

	"github.com/xh-polaris/psych-core-api/biz/application/dto/basic"
	"github.com/xh-polaris/psych-core-api/biz/application/dto/core_api"
	"github.com/xh-polaris/psych-core-api/biz/cst"
	"github.com/xh-polaris/psych-core-api/biz/domain/his"
	"github.com/xh-polaris/psych-core-api/biz/domain/wordcld"
	"github.com/xh-polaris/psych-core-api/biz/infra/mapper/alarm"
	"github.com/xh-polaris/psych-core-api/biz/infra/mapper/conversation"
	"github.com/xh-polaris/psych-core-api/biz/infra/mapper/report"
	"github.com/xh-polaris/psych-core-api/biz/infra/mapper/user"
	"github.com/xh-polaris/psych-core-api/biz/infra/util"
	"github.com/xh-polaris/psych-core-api/pkg/errorx"
	"github.com/xh-polaris/psych-core-api/pkg/logs"
	"github.com/xh-polaris/psych-core-api/types/enum"
	"github.com/xh-polaris/psych-core-api/types/errno"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
)

// ==================== 工具类辅助（纯函数/数据，不涉及存储） ====================

// buildTrendPoints 将按星期聚合的计数转为固定 7 个趋势点（1=Mon ... 7=Sun，缺失补 0）
func buildTrendPoints(cnt map[int32]int32) []*core_api.TrendPoint {
	points := make([]*core_api.TrendPoint, 0, 7)
	for w := int32(1); w <= 7; w++ {
		points = append(points, &core_api.TrendPoint{Week: w, Hour: 0, Count: cnt[w]})
	}
	return points
}

// fullDayWindow 将起讫时间规整为整日窗口（起始日 0 点 ~ 结束日 24 点）
func fullDayWindow(start, end time.Time) (time.Time, time.Time) {
	days := dayRange(start, end)
	return days[0], days[len(days)-1].AddDate(0, 0, 1)
}

func dayRange(start, end time.Time) []time.Time {
	s := time.Date(start.Year(), start.Month(), start.Day(), 0, 0, 0, 0, start.Location())
	e := time.Date(end.Year(), end.Month(), end.Day(), 0, 0, 0, 0, end.Location())
	if s.After(e) {
		e = s
	}
	days := 0
	for d := s; !d.After(e); d = d.AddDate(0, 0, 1) {
		days++
	}
	out := make([]time.Time, 0, days)
	for d := s; !d.After(e); d = d.AddDate(0, 0, 1) {
		out = append(out, d)
	}
	return out
}

// durationBucketDefs 对话时长分桶边界（max=-1 表示无上限）
var durationBucketDefs = []struct{ min, max float64 }{
	{0, 5}, {6, 10}, {11, 20}, {21, 30}, {31, 60}, {61, 120}, {121, -1},
}

// ==================== 共享业务逻辑（多角色复用，涉及存储） ====================

// dataTrend 超管/单位管理共用逻辑
func (d *DashboardDomain) dataTrend(ctx context.Context, unitOID *bson.ObjectID, startGrade int, start, end time.Time) (*core_api.DashboardGetDataTrendResp, error) {
	// 活跃趋势（按星期聚合，起讫时间内所有周一/二/... 之和）
	activeCnt, err := d.ConversationMapper.CountActiveUsersByWeekday(ctx, unitOID, start, end)
	if err != nil {
		return nil, errorx.WrapByCode(err, errno.ErrDashboardActiveUserStat)
	}
	activePoints := buildTrendPoints(activeCnt)

	// 对话频率趋势（按星期聚合）
	convCnt, err := d.ConversationMapper.CountUnitConvByWeekday(ctx, unitOID, start, end)
	if err != nil {
		return nil, errorx.WrapByCode(err, errno.ErrDashboardConversationStat)
	}
	convPoints := buildTrendPoints(convCnt)

	// 对话时长分布
	convDurations, err := d.durationBuckets(ctx, unitOID, start, end)
	if err != nil {
		return nil, err
	}

	// 各年级高风险用户数分布
	riskDistribution, err := d.riskDistrbByGrade(ctx, unitOID, startGrade, start, end)
	if err != nil {
		return nil, err
	}

	return &core_api.DashboardGetDataTrendResp{
		ActivePoints:          activePoints,
		ConversationPoints:    convPoints,
		ConversationDurations: convDurations,
		RiskDistribution:      riskDistribution,
		Code:                  0,
		Msg:                   "success",
	}, nil
}

// psychTrend 超管/单位管理共用逻辑
func (d *DashboardDomain) psychTrend(ctx context.Context, unitOID *bson.ObjectID, start, end time.Time) (*core_api.DashboardGetPsychTrendResp, error) {
	// 情绪/风险等级来自时间段内每个用户最后一份报表
	stats, err := d.ReportMapper.GetUserPsychStats(ctx, unitOID, start, end, nil, nil)
	if err != nil {
		return nil, errorx.WrapByCode(err, errno.ErrDashboardRiskDistribution)
	}
	keywords, err := d.getKeywords(ctx, unitOID, start, end)
	if err != nil {
		return nil, errorx.WrapByCode(err, errno.ErrDashboardGetUserKeywords)
	}
	return &core_api.DashboardGetPsychTrendResp{
		EmotionRatio: buildEmotionRatio(stats),
		Risks:        buildRiskDistribution(stats),
		Keywords:     keywords,
		Code:         0,
		Msg:          "success",
	}, nil
}

// durationBuckets 对话时长分桶统计
func (d *DashboardDomain) durationBuckets(ctx context.Context, unitOID *bson.ObjectID, start, end time.Time) ([]*core_api.ConversationDuration, error) {
	result := make([]*core_api.ConversationDuration, 0, len(durationBucketDefs))
	for i, b := range durationBucketDefs {
		cnt, err := d.ConversationMapper.CountByDurationBucket(ctx, unitOID, b.min, b.max, start, end)
		if err != nil {
			return nil, errorx.WrapByCode(err, errno.ErrDashboardConversationStat)
		}
		result = append(result, &core_api.ConversationDuration{Key: int32(i + 1), Count: cnt})
	}
	return result, nil
}

// riskDistrbByGrade 各年级高风险用户数分布
func (d *DashboardDomain) riskDistrbByGrade(ctx context.Context, unitOID *bson.ObjectID, startGrade int, start, end time.Time) (*core_api.RiskDistributionByGrade, error) {
	if unitOID == nil {
		return &core_api.RiskDistributionByGrade{Ratio: make(map[int32]int32), Total: 0}, nil
	}
	riskMap, total, err := d.AlarmMapper.CountAlarmUsersByGrade(ctx, *unitOID, startGrade, start, end)
	if err != nil {
		return nil, errorx.WrapByCode(err, errno.ErrDashboardConversationStat)
	}
	return &core_api.RiskDistributionByGrade{Ratio: util.RiskDistributionCnt2Ratio(riskMap, total), Total: total}, nil
}

func (d *DashboardDomain) getKeywords(ctx context.Context, unitOID *bson.ObjectID, start, end time.Time) (*core_api.Keywords, error) {
	if unitOID != nil {
		return wordcld.Extractor.FromUnitKWs(ctx, *unitOID, start, end)
	}
	return wordcld.Extractor.FromAllUnitsKWs(ctx, start, end)
}

// updateRemark 更新用户备注（单位管理/班主任共用）
func (d *DashboardDomain) updateRemark(ctx context.Context, userOID bson.ObjectID, remark string) error {
	update := bson.M{
		cst.Remark: &user.Remark{
			Content:    remark,
			CreateTime: time.Now(),
		},
	}
	if err := d.UserMapper.UpdateFields(ctx, userOID, update); err != nil {
		logs.Errorf("update user error: %s", errorx.ErrorWithoutStack(err))
		return err
	}
	return nil
}

// completeRiskUser 补全风险用户信息（对话统计、关键词、风险等级）
// 高危学生（User.RiskLevel==High，少数）以最新 AlarmRecord 为准；剩余学生以最新 SimpleReport 为准
func (d *DashboardDomain) completeRiskUser(ctx context.Context, dbUsers []*user.User, startGrade int) ([]*core_api.RiskUser, error) {
	if len(dbUsers) == 0 {
		return make([]*core_api.RiskUser, 0), nil
	}

	uids := make([]bson.ObjectID, len(dbUsers))
	highRiskUids := make([]bson.ObjectID, 0, len(dbUsers))
	for i, dbUser := range dbUsers {
		uids[i] = dbUser.ID
		if dbUser.RiskLevel == enum.UserRiskLevelHigh {
			highRiskUids = append(highRiskUids, dbUser.ID)
		}
	}

	// 三路并行：对话统计（全部）、AlarmRecord（高危学生）、最新 SimpleReport（全部，作剩余学生数据源及高危回退）
	var msgStats map[bson.ObjectID]*conversation.ConvStats
	var alarmMap map[bson.ObjectID]*alarm.Alarm
	var latestReports map[bson.ObjectID]*report.Report
	var msgErr, alarmErr, rptErr error

	var wg sync.WaitGroup
	wg.Add(3)
	go func() {
		defer wg.Done()
		msgStats, msgErr = d.ConversationMapper.BatchConvStats(ctx, uids)
		if msgErr != nil {
			logs.Warnf("查询对话统计失败: %v", errorx.ErrorWithoutStack(msgErr))
		}
	}()
	go func() {
		defer wg.Done()
		alarmMap, alarmErr = d.AlarmMapper.BatchFindLatestByUserIds(ctx, highRiskUids)
		if alarmErr != nil {
			logs.Errorf("查询高危学生预警失败: %v", errorx.ErrorWithoutStack(alarmErr))
		}
	}()
	go func() {
		defer wg.Done()
		latestReports, rptErr = d.ReportMapper.BatchFindUserLatest(ctx, uids)
		if rptErr != nil {
			logs.Errorf("查询用户最新报表失败: %v", errorx.ErrorWithoutStack(rptErr))
		}
	}()
	wg.Wait()

	if msgErr != nil || msgStats == nil {
		return nil, errorx.New(errno.ErrDashboardGetUserConversationStatic)
	}
	if rptErr != nil {
		return nil, errorx.New(errno.ErrDashboardRiskDistribution)
	}
	if alarmErr != nil {
		return nil, errorx.New(errno.ErrDashboardAlarmUserStat)
	}

	riskUsers := make([]*core_api.RiskUser, len(dbUsers))
	for i, dbUser := range dbUsers {
		remark := &core_api.Remark{
			Time:    dbUser.Remark.CreateTime.Unix(),
			Content: dbUser.Remark.Content,
		}
		calculatedGrade := dbUser.CalculateGrade(startGrade)
		riskUsers[i] = &core_api.RiskUser{
			User: &core_api.UserVO{
				Id:     dbUser.ID.Hex(),
				Code:   dbUser.Code,
				Name:   dbUser.Name,
				Gender: int32(dbUser.Gender),
				Grade:  int32(calculatedGrade),
				Class:  int32(dbUser.Class),
				Remark: remark,
			},
			Level:    int32(dbUser.RiskLevel),
			Keywords: make([]string, 0),
		}
		if msgStats[dbUser.ID] != nil {
			riskUsers[i].TotalConversationRounds = msgStats[dbUser.ID].Rounds
			riskUsers[i].LastConversationTime = msgStats[dbUser.ID].LatestTime
		}

		// 高危学生：以最新 AlarmRecord 为准（情绪/关键词），无预警则回退最新报表
		if al := alarmMap[dbUser.ID]; al != nil {
			riskUsers[i].Level = int32(enum.UserRiskLevelHigh)
			if len(al.Keywords) > 0 {
				riskUsers[i].Keywords = al.Keywords
			}
			continue
		}

		// 剩余学生：最新 SimpleReport 提供 风险等级/关键词
		if rpt := latestReports[dbUser.ID]; rpt != nil {
			if rpt.SimpleReport != nil {
				riskUsers[i].Level = int32(enum.RiskLevelToInt(rpt.SimpleReport.Summary.RiskLevel))
			}
			if kw := util.KeywordsMap2Slice(rpt.Keywords); len(kw) > 0 {
				riskUsers[i].Keywords = kw
			}
		}
		if riskUsers[i].Level <= 0 {
			riskUsers[i].Level = int32(enum.UserRiskLevelNormal)
		}
	}

	return riskUsers, nil
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
