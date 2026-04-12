package service

import (
	"context"
	"time"

	"github.com/google/wire"
	"github.com/xh-polaris/psych-core-api/biz/application/dto/core_api"
	"github.com/xh-polaris/psych-core-api/biz/cst"
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
	GetConversation(ctx context.Context, req *core_api.GetConversationReq) (resp *core_api.GetConversationResp, err error)
	FinishConversation(ctx context.Context)
}

type ConversationService struct {
	MessageMapper      message.IMongoMapper
	ConversationMapper conversation.IMongoMapper
}

var ConversationServiceSet = wire.NewSet(
	wire.Struct(new(ConversationService), "*"),
	wire.Bind(new(IConversationService), new(*ConversationService)),
)

func (c *ConversationService) CreateConversation(ctx context.Context, req *core_api.CreateConversationReq) (resp *core_api.CreateConversationResp, err error) {
	userMeta, err := util.ExtraUserMeta(ctx)
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
	userMeta, err := util.ExtraUserMeta(ctx)
	if err != nil {
		return nil, err
	}

	userId, err := bson.ObjectIDFromHex(userMeta.UserId)
	if err != nil {
		return nil, errorx.New(errno.ErrInvalidParams)
	}
	total, err := c.ConversationMapper.CountByUser(ctx, userId)
	if err != nil {
		return nil, errorx.New(errno.ErrListConversation)
	}

	if total == 0 {
		return &core_api.ListConversationsResp{
			Pagination: util.PaginationRes(0, req.PaginationOptions),
			Code:       0,
			Msg:        "success",
		}, nil
	}

	findOpt := util.PagedFindOpt(req.PaginationOptions).SetSort(bson.D{{cst.UpdateTime, -1}})
	dbConvs, err := c.ConversationMapper.FindManyByUserId(ctx, userId, findOpt)
	if err != nil {
		return nil, errorx.New(errno.ErrListConversation)
	}

	convs := make([]*core_api.ConversationVO, 0, len(dbConvs))
	for _, dbConv := range dbConvs {
		conv := &core_api.ConversationVO{
			ConversationId: dbConv.ID.Hex(),
			Brief:          dbConv.Title,
			CreateTime:     dbConv.CreateTime.Unix(),
			UpdateTime:     dbConv.UpdateTime.Unix(),
		}
		convs = append(convs, conv)
	}

	return &core_api.ListConversationsResp{
		Pagination:       util.PaginationRes(total, req.PaginationOptions),
		ConversationList: convs,
		Code:             0,
		Msg:              "success",
	}, nil
}

func (c *ConversationService) GetConversation(ctx context.Context, req *core_api.GetConversationReq) (resp *core_api.GetConversationResp, err error) {
	_, err = util.ExtraUserMeta(ctx)
	if err != nil {
		return nil, err
	}

	// RetrieveMessage仅返回意外异常 消息搜索结果为空时返回空切片，和nil err
	// 非空时，返回index倒序的列表
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

	return &core_api.GetConversationResp{
		Pagination:  util.PaginationRes(total, req.PaginationOptions),
		MessageList: msgs,
		Code:        0,
		Msg:         "success",
	}, nil
}

func (c *ConversationService) FinishConversation(ctx context.Context) {}
