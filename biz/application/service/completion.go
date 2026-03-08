package service

import (
	"context"
	"github.com/google/wire"
	"github.com/xh-polaris/psych-core-api/biz/application/dto/core_api"
	"github.com/xh-polaris/psych-core-api/biz/cst"
	"github.com/xh-polaris/psych-core-api/biz/infra/mapper/conversation"
	"github.com/xh-polaris/psych-core-api/biz/infra/mapper/message"
	"github.com/xh-polaris/psych-core-api/biz/infra/util"
	"github.com/xh-polaris/psych-core-api/pkg/errorx"
	"github.com/xh-polaris/psych-core-api/types/errno"
	"go.mongodb.org/mongo-driver/v2/bson"
	"time"
)

type IConversationService interface {
	CreateConversation(ctx context.Context, req *core_api.CreateConversationReq) (resp *core_api.CreateConversationResp, err error)
	ListConversations(ctx context.Context, req *core_api.ListConversationsReq) (resp *core_api.ListConversationsResp, err error)
	GetConversation(ctx context.Context, req *core_api.GetConversationReq) (resp *core_api.GetConversationResp, err error)
}

type ConversationService struct {
	MessageMapper      message.MongoMapper
	ConversationMapper conversation.IMongoMapper
}

var ConversationServiceSet = wire.NewSet(
	wire.Struct(new(ConversationService), "*"),
	wire.Bind(new(IConversationService), new(*ConversationService)),
)

func (c *ConversationService) CreateConversation(ctx context.Context, req *core_api.CreateConversationReq) (resp *core_api.CreateConversationResp, err error) {
	//userMeta, err := util.ExtraUserMeta(ctx)
	//if err != nil {
	//	return nil, err
	//}

	temp := bson.NewObjectID()
	if err := c.ConversationMapper.Insert(ctx, &conversation.Conversation{
		ID: temp,
		//UserID: userMeta.UserId, // TODO 鉴权时从token获得
		CreateTime: time.Now(),
		UpdateTime: time.Now(),
	}); err != nil {
		return nil, errorx.New(errno.ErrCreateConversation)
	}

	return &core_api.CreateConversationResp{
		ConversationId: temp.Hex(),
		Code:           200,
		Msg:            "success",
	}, nil
}

func (c *ConversationService) ListConversations(ctx context.Context, req *core_api.ListConversationsReq) (resp *core_api.ListConversationsResp, err error) {
	//userMeta, err := util.ExtraUserMeta(ctx)
	//if err != nil {
	//	return nil, err
	//}

	userId := bson.NewObjectID() // TODO

	total, err := c.ConversationMapper.CountByUser(ctx, userId)
	if err != nil {
		return nil, errorx.New(errno.ErrListConversation)
	}

	if total == 0 {
		return &core_api.ListConversationsResp{
			Pagination: util.PaginationRes(0, req.PaginationOptions),
			Code:       200,
			Msg:        "success",
		}, nil
	}

	findOpt := util.PagedFindOpt(req.PaginationOptions).SetSort(bson.D{{cst.UpdateTime, -1}})
	dbConvs, err := c.ConversationMapper.FindManyByUserId(ctx, userId, findOpt)
	if err != nil {
		return nil, errorx.New(errno.ErrListConversation)
	}

	convs := make([]*core_api.Conversation, len(dbConvs))
	for _, dbConv := range dbConvs {
		conv := &core_api.Conversation{
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
		Code:             200,
		Msg:              "success",
	}, nil
}

func (c *ConversationService) GetConversation(ctx context.Context, req *core_api.GetConversationReq) (resp *core_api.GetConversationResp, err error) {
	return nil, errorx.New(errno.ErrUnImplement)
}
