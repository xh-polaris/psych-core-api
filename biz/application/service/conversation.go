package service

import (
	"context"
	"sort"
	"time"

	"github.com/google/wire"
	"github.com/xh-polaris/psych-core-api/biz/application/dto/core_api"
	"github.com/xh-polaris/psych-core-api/biz/cst"
	"github.com/xh-polaris/psych-core-api/biz/domain/auth"
	"github.com/xh-polaris/psych-core-api/biz/domain/his"
	"github.com/xh-polaris/psych-core-api/biz/infra/mapper/conversation"
	"github.com/xh-polaris/psych-core-api/biz/infra/mapper/message"
	"github.com/xh-polaris/psych-core-api/biz/infra/util"
	"github.com/xh-polaris/psych-core-api/pkg/errorx"
	"github.com/xh-polaris/psych-core-api/types/enum"
	"github.com/xh-polaris/psych-core-api/types/errno"
	"go.mongodb.org/mongo-driver/v2/bson"
)

type IConversationService interface {
	CreateConversation(ctx context.Context, req *core_api.CreateConversationReq) (resp *core_api.CreateConversationResp, err error)
	ListConversations(ctx context.Context, req *core_api.ListConversationsReq) (resp *core_api.ListConversationsResp, err error)
	GetSingleConv(ctx context.Context, req *core_api.GetSingleConvReq) (resp *core_api.GetSingleConvResp, err error)
	GetConvByDate(ctx context.Context, req *core_api.GetConvByDateReq, characterId string) (resp *core_api.GetConvByDateResp, err error)
	ArchiveConversation(ctx context.Context, conversationId string) error
}

type ConversationService struct {
	AuthDomain         auth.IAuthDomain
	MessageMapper      message.IMongoMapper
	ConversationMapper conversation.IMongoMapper
}

var ConversationServiceSet = wire.NewSet(
	wire.Struct(new(ConversationService), "*"),
	wire.Bind(new(IConversationService), new(*ConversationService)),
)

func (c *ConversationService) CreateConversation(ctx context.Context, req *core_api.CreateConversationReq) (resp *core_api.CreateConversationResp, err error) {
	userMeta, err := c.AuthDomain.ExtraUserMeta(ctx)
	if err != nil {
		return nil, err
	}

	userOID, err := bson.ObjectIDFromHex(userMeta.UserId)
	if err != nil {
		return nil, errorx.New(errno.ErrInvalidParams)
	}

	temp := bson.NewObjectID()
	if err := c.ConversationMapper.Insert(ctx, &conversation.Conversation{
		ID:         temp,
		UserID:     userOID,
		Status:     enum.ConversationStatusDeleted,
		CreateTime: time.Now(),
		UpdateTime: time.Now(),
	}); err != nil {
		return nil, errorx.New(errno.ErrCreateConversation)
	}

	return &core_api.CreateConversationResp{
		ConversationId: temp.Hex(),
		Code:           0,
		Msg:            "success",
	}, nil
}

func (c *ConversationService) ListConversations(ctx context.Context, req *core_api.ListConversationsReq) (resp *core_api.ListConversationsResp, err error) {
	userMeta, err := c.AuthDomain.ExtraUserMeta(ctx)
	if err != nil {
		return nil, err
	}

	userId, err := bson.ObjectIDFromHex(userMeta.UserId)
	if err != nil {
		return nil, errorx.New(errno.ErrInvalidParams)
	}

	pg := util.EnsurePaginationOptions(req.PaginationOptions)

	var convs []*conversation.Conversation
	if req.StartDate == "" && req.EndDate == "" {
		// 学生历史记录默认返回全部会话；日期参数仅作为显式筛选条件。
		convs, err = c.ConversationMapper.FindAllByUserId(ctx, userId)
	} else {
		endDate := req.EndDate
		if endDate == "" {
			endDate = util.FormatDateUTC8(time.Now())
		}
		startDate := req.StartDate
		if startDate == "" {
			end, _, rangeErr := util.DayToUTCRange(endDate)
			if rangeErr != nil {
				return nil, errorx.New(errno.ErrInvalidParams)
			}
			startDate = util.FormatDateUTC8(end.Add(-30 * 24 * time.Hour))
		}
		start, end, rangeErr := util.ParseDayRange(startDate, endDate)
		if rangeErr != nil {
			return nil, errorx.New(errno.ErrInvalidParams)
		}
		convs, err = c.ConversationMapper.FindByUserIdAndTimeRange(ctx, userId, start, end)
	}
	if err != nil {
		return nil, errorx.New(errno.ErrListConversation)
	}

	// 学生历史按「日期 + 对话角色」组织。同一天与同一老师的多场会话
	// 在列表中只占一项，详情通过 get_by_date 合并展示。
	type dailyConversation struct {
		date        string
		characterId string
		first       *conversation.Conversation
	}
	dailyMap := make(map[string]*dailyConversation)
	for _, conv := range convs {
		date := util.FormatDateUTC8(conv.CreateTime)
		characterId := conv.CharacterID.Hex()
		key := date + ":" + characterId
		group := dailyMap[key]
		if group == nil {
			dailyMap[key] = &dailyConversation{date: date, characterId: characterId, first: conv}
			continue
		}
		if conv.UpdateTime.After(group.first.UpdateTime) {
			group.first = conv
		}
	}
	daily := make([]*dailyConversation, 0, len(dailyMap))
	for _, group := range dailyMap {
		daily = append(daily, group)
	}
	sort.Slice(daily, func(i, j int) bool {
		return daily[i].first.UpdateTime.After(daily[j].first.UpdateTime)
	})

	total := int32(len(daily))
	if total == 0 {
		return &core_api.ListConversationsResp{
			Pagination: util.PaginationRes(0, pg),
			Code:       0,
			Msg:        "success",
		}, nil
	}

	startIdx, endIdx := util.PagedIndex(total, pg)
	result := make([]*core_api.ConversationVO, 0, endIdx-startIdx)
	for _, group := range daily[startIdx:endIdx] {
		conv := group.first
		result = append(result, &core_api.ConversationVO{
			ConversationId: conv.ID.Hex(),
			Brief:          group.date,
			CreateTime:     conv.CreateTime.Unix(),
			UpdateTime:     conv.UpdateTime.Unix(),
			Date:           group.date,
			CharacterId:    group.characterId,
		})
	}

	return &core_api.ListConversationsResp{
		Pagination:       util.PaginationRes(total, pg),
		ConversationList: result,
		Code:             0,
		Msg:              "success",
	}, nil
}

// GetConvByDate 返回用户指定日期的对话消息
func (c *ConversationService) GetConvByDate(ctx context.Context, req *core_api.GetConvByDateReq, characterId string) (resp *core_api.GetConvByDateResp, err error) {
	// 提取当前用户userId
	userMeta, err := c.AuthDomain.ExtraUserMeta(ctx)
	if err != nil {
		return nil, err
	}
	userId := userMeta.UserId

	// 传入其他userId的场景：管理员查看学生的对话内容
	if req.UserId != "" && req.UserId != userId {
		if userMeta.Role < enum.UserRoleUnitAdmin {
			return nil, errorx.New(errno.ErrInsufficientAuth)
		}
		userId = req.UserId
	}

	// 日期默认为当天
	if req.Date == "" {
		req.Date = util.FormatDateUTC8(time.Now())
	}

	// 获取当日消息
	msgs, err := his.Mgr.GetUserDailyMessages(ctx, userId, req.Date)
	if err != nil {
		return nil, errorx.New(errno.ErrFetchMessages)
	}

	// 学生端按老师隔离每日记录，防止同一天切换老师后消息混在一起。
	if characterId != "" {
		characterOID, parseErr := bson.ObjectIDFromHex(characterId)
		if parseErr != nil {
			return nil, errorx.New(errno.ErrInvalidParams)
		}
		userOID, parseErr := bson.ObjectIDFromHex(userId)
		if parseErr != nil {
			return nil, errorx.New(errno.ErrInvalidParams)
		}
		start, end, rangeErr := util.DayToUTCRange(req.Date)
		if rangeErr != nil {
			return nil, errorx.New(errno.ErrInvalidParams)
		}
		convs, findErr := c.ConversationMapper.FindByUserIdAndTimeRange(ctx, userOID, start, end)
		if findErr != nil {
			return nil, errorx.New(errno.ErrFetchMessages)
		}
		allowed := make(map[bson.ObjectID]struct{})
		for _, conv := range convs {
			if conv.CharacterID == characterOID {
				allowed[conv.ID] = struct{}{}
			}
		}
		filtered := make([]*message.Message, 0, len(msgs))
		for _, msg := range msgs {
			if _, ok := allowed[msg.ConversationId]; ok {
				filtered = append(filtered, msg)
			}
		}
		msgs = filtered
	}

	// 分页并重新排序
	total := int32(len(msgs))
	startIdx, endIdx := util.PagedIndex(total, req.PaginationOptions)

	result := make([]*core_api.ConvByDateMessage, 0, endIdx-startIdx)
	for i, msg := range msgs[startIdx:endIdx] {
		result = append(result, &core_api.ConvByDateMessage{
			ConversationId: msg.ConversationId.Hex(),
			Content:        msg.Content,
			Role:           int32(msg.Role),
			Index:          int32(total) - 1 - int32(startIdx+i),
			CreateTime:     msg.CreateTime.Unix(),
		})
	}

	return &core_api.GetConvByDateResp{
		Pagination:  util.PaginationRes(total, req.PaginationOptions),
		MessageList: result,
		Code:        0,
		Msg:         "success",
	}, nil
}

func (c *ConversationService) GetSingleConv(ctx context.Context, req *core_api.GetSingleConvReq) (resp *core_api.GetSingleConvResp, err error) {
	userMeta, err := c.AuthDomain.ExtraUserMeta(ctx)
	if err != nil {
		return nil, err
	}

	conversationOID, err := bson.ObjectIDFromHex(req.ConversationId)
	if err != nil {
		return nil, errorx.New(errno.ErrInvalidParams)
	}
	conv, err := c.ConversationMapper.FindOneById(ctx, conversationOID)
	if err != nil || conv.Status == enum.ConversationStatusDeleted {
		return nil, errorx.New(errno.ErrGetConversation)
	}
	// 学生端只能查看自己的记录。管理员查看学生详情使用 dashboard 接口，
	// 避免三个端的权限环境相互混用。
	if conv.UserID.Hex() != userMeta.UserId {
		return nil, errorx.New(errno.ErrInsufficientAuth)
	}

	rawMsgs, err := his.Mgr.RetrieveMessage(ctx, req.ConversationId, -1)
	if err != nil {
		return nil, errorx.New(errno.ErrFetchMessages)
	}

	total := int32(len(rawMsgs))
	startIdx, endIdx := util.PagedIndex(total, req.PaginationOptions)

	msgs := make([]*core_api.Message, 0, len(rawMsgs))
	for _, rawMsg := range rawMsgs[startIdx:endIdx] {
		msgs = append(msgs, &core_api.Message{
			Content: rawMsg.Content,
			Role:    int32(rawMsg.Role),
			Index:   int32(rawMsg.Index),
		})
	}

	return &core_api.GetSingleConvResp{
		Pagination:  util.PaginationRes(total, req.PaginationOptions),
		MessageList: msgs,
		Code:        0,
		Msg:         "success",
	}, nil
}

func (c *ConversationService) ArchiveConversation(ctx context.Context, conversationId string) error {
	userMeta, err := c.AuthDomain.ExtraUserMeta(ctx)
	if err != nil {
		return err
	}
	conversationOID, err := bson.ObjectIDFromHex(conversationId)
	if err != nil {
		return errorx.New(errno.ErrInvalidParams, errorx.KV("field", "conversationId"))
	}
	conv, err := c.ConversationMapper.FindOneById(ctx, conversationOID)
	if err != nil || conv == nil {
		return errorx.New(errno.ErrGetConversation)
	}
	if conv.UserID.Hex() != userMeta.UserId {
		return errorx.New(errno.ErrInsufficientAuth)
	}

	messages, err := his.Mgr.RetrieveMessage(ctx, conversationId, -1)
	if err != nil {
		return errorx.New(errno.ErrFetchMessages)
	}
	if len(messages) == 0 {
		return nil
	}

	brief := conv.Title
	if brief == "" {
		// RetrieveMessage 按时间倒序返回，从后向前寻找首条学生消息作为摘要。
		for i := len(messages) - 1; i >= 0; i-- {
			if messages[i].Role == enum.MsgRoleUser && messages[i].Content != "" {
				briefRunes := []rune(messages[i].Content)
				if len(briefRunes) > 30 {
					briefRunes = briefRunes[:30]
				}
				brief = string(briefRunes)
				break
			}
		}
	}
	now := time.Now()
	update := bson.M{
		cst.Status:       enum.ConversationStatusActive,
		cst.EndTime:      now,
		cst.UpdateTime:   now,
		cst.MessageCount: len(messages),
	}
	if brief != "" {
		update["title"] = brief
	}
	return c.ConversationMapper.UpdateFields(ctx, conversationOID, update)
}
