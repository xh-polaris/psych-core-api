package service

import (
	"context"
	"time"

	"github.com/google/wire"
	"github.com/xh-polaris/psych-core-api/biz/application/dto/core_api"
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
	GetConvByDate(ctx context.Context, req *core_api.GetConvByDateReq) (resp *core_api.GetConvByDateResp, err error)
	FinishConversation(ctx context.Context)
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
	// 起止时间，默认今日-30天前
	endDate := req.EndDate
	if endDate == "" {
		endDate = util.FormatDateUTC8(time.Now())
	}
	startDate := req.StartDate
	if startDate == "" {
		end, _, _ := util.DayToUTCRange(endDate)
		startDate = util.FormatDateUTC8(end.Add(-30 * 24 * time.Hour))
	}
	// 默认一次显示5天对话
	limit := int(5)
	if req.PaginationOptions != nil && req.PaginationOptions.Limit != nil {
		limit = int(*req.PaginationOptions.Limit)
	}
	page := int(1)
	if req.PaginationOptions != nil && req.PaginationOptions.Page != nil {
		page = int(*req.PaginationOptions.Page)
	}

	// 限定起止时间，默认为30天内
	start, end, err := util.ParseDayRange(startDate, endDate)
	if err != nil {
		return nil, errorx.New(errno.ErrInvalidParams)
	}

	dates, err := c.ConversationMapper.FindDistinctDatesByUserId(ctx, userId, start, end)
	if err != nil {
		return nil, errorx.New(errno.ErrListConversation)
	}

	if len(dates) == 0 {
		return &core_api.ListConversationsResp{
			Pagination: util.PaginationRes(0, req.PaginationOptions),
			Code:       0,
			Msg:        "success",
		}, nil
	}

	startIdx := (page - 1) * limit
	if startIdx >= len(dates) {
		return &core_api.ListConversationsResp{
			Pagination: util.PaginationRes(int32(len(dates)), req.PaginationOptions),
			Code:       0,
			Msg:        "success",
		}, nil
	}
	endIdx := startIdx + limit
	if endIdx > len(dates) {
		endIdx = len(dates)
	}
	pageDates := dates[startIdx:endIdx]

	dateSet := make(map[string]bool, len(pageDates))
	for _, d := range pageDates {
		dateSet[d] = true
	}

	convs, err := c.ConversationMapper.FindByUserIdAndTimeRange(ctx, userId, start, end)
	if err != nil {
		return nil, errorx.New(errno.ErrListConversation)
	}

	result := make([]*core_api.ConversationVO, 0)
	for _, conv := range convs {
		convDate := util.FormatDateUTC8(conv.CreateTime)
		if !dateSet[convDate] {
			continue
		}
		result = append(result, &core_api.ConversationVO{
			ConversationId: conv.ID.Hex(),
			Brief:          conv.Title,
			CreateTime:     conv.CreateTime.Unix(),
			UpdateTime:     conv.UpdateTime.Unix(),
			Date:           convDate,
		})
	}

	return &core_api.ListConversationsResp{
		Pagination:       util.PaginationRes(int32(len(dates)), req.PaginationOptions),
		ConversationList: result,
		Code:             0,
		Msg:              "success",
	}, nil
}

// GetConvByDate 返回用户指定日期的对话消息
func (c *ConversationService) GetConvByDate(ctx context.Context, req *core_api.GetConvByDateReq) (resp *core_api.GetConvByDateResp, err error) {
	userMeta, err := c.AuthDomain.ExtraUserMeta(ctx)
	if err != nil {
		return nil, err
	}

	userId := userMeta.UserId
	if req.UserId != "" && req.UserId != userId {
		if userMeta.Role < enum.UserRoleUnitAdmin {
			return nil, errorx.New(errno.ErrInsufficientAuth)
		}
		userId = req.UserId
	}

	if req.Date == "" {
		req.Date = util.FormatDateUTC8(time.Now())
	}

	msgs, err := his.Mgr.GetUserDailyMessages(ctx, userId, req.Date)
	if err != nil {
		return nil, errorx.New(errno.ErrFetchMessages)
	}

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
	_, err = c.AuthDomain.ExtraUserMeta(ctx)
	if err != nil {
		return nil, err
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

func (c *ConversationService) FinishConversation(ctx context.Context) {}
