package service

import (
	"context"
	"errors"
	"sort"
	"time"

	"github.com/google/wire"
	"github.com/xh-polaris/psych-core-api/biz/application/dto/core_api"
	"github.com/xh-polaris/psych-core-api/biz/conf"
	"github.com/xh-polaris/psych-core-api/biz/domain/auth"
	"github.com/xh-polaris/psych-core-api/biz/domain/his"
	"github.com/xh-polaris/psych-core-api/biz/infra/mapper/conversation"
	"github.com/xh-polaris/psych-core-api/biz/infra/mapper/message"
	"github.com/xh-polaris/psych-core-api/biz/infra/mapper/user"
	"github.com/xh-polaris/psych-core-api/biz/infra/util"
	"github.com/xh-polaris/psych-core-api/pkg/errorx"
	"github.com/xh-polaris/psych-core-api/types/enum"
	"github.com/xh-polaris/psych-core-api/types/errno"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
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
	UserMapper         user.IMongoMapper
	Config             *conf.Config
}

var ConversationServiceSet = wire.NewSet(
	wire.Struct(new(ConversationService), "*"),
	wire.Bind(new(IConversationService), new(*ConversationService)),
)

// CreateConversation 获取或创建「当天 + 指定老师」的会话
// 会话唯一维度为 (user_id, chat_date, character_id)，同一学生同一天与不同老师各自拥有独立会话
func (c *ConversationService) CreateConversation(ctx context.Context, req *core_api.CreateConversationReq) (resp *core_api.CreateConversationResp, err error) {
	userMeta, err := c.AuthDomain.ExtraUserMeta(ctx)
	if err != nil {
		return nil, err
	}

	userOID, err := bson.ObjectIDFromHex(userMeta.UserId)
	if err != nil {
		return nil, errorx.New(errno.ErrInvalidParams)
	}

	if req.CharacterId == "" {
		return nil, errorx.New(errno.ErrInvalidParams, errorx.KV("field", "characterId"))
	}
	charOID, err := bson.ObjectIDFromHex(req.CharacterId)
	if err != nil {
		return nil, errorx.New(errno.ErrInvalidParams, errorx.KV("field", "characterId"))
	}

	now := time.Now()
	chatDate := util.FormatDateUTC8(now)
	if existing, findErr := c.ConversationMapper.FindWritableByUserDateAndCharacter(ctx, userOID, chatDate, charOID); findErr == nil {
		return &core_api.CreateConversationResp{
			ConversationId: existing.ID.Hex(),
			Code:           0,
			Msg:            "success",
		}, nil
	} else if !errors.Is(findErr, mongo.ErrNoDocuments) {
		return nil, errorx.New(errno.ErrCreateConversation)
	}
	created := &conversation.Conversation{
		ID:          bson.NewObjectID(),
		UserID:      userOID,
		ChatDate:    chatDate,
		CharacterID: charOID,
		Status:      enum.ConversationStatusPending,
		CreateTime:  now,
		UpdateTime:  now,
	}
	if err := c.ConversationMapper.Insert(ctx, created); err != nil {
		// 唯一索引决定唯一会话后，返回已创建的会话
		if mongo.IsDuplicateKeyError(err) {
			existing, findErr := c.ConversationMapper.FindWritableByUserDateAndCharacter(ctx, userOID, chatDate, charOID)
			if findErr == nil {
				return &core_api.CreateConversationResp{
					ConversationId: existing.ID.Hex(),
					Code:           0,
					Msg:            "success",
				}, nil
			}
		}
		return nil, errorx.New(errno.ErrCreateConversation)
	}

	return &core_api.CreateConversationResp{
		ConversationId: created.ID.Hex(),
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

	// 限定起止时间，默认为30天内
	start, end, err := util.ParseDayRange(startDate, endDate)
	if err != nil {
		return nil, errorx.New(errno.ErrInvalidParams)
	}

	pg := util.EnsurePaginationOptions(req.PaginationOptions)

	dates, err := c.ConversationMapper.FindDistinctDatesByUserId(ctx, userId, start, end)
	if err != nil {
		return nil, errorx.New(errno.ErrListConversation)
	}

	total := int32(len(dates))
	if total == 0 {
		return &core_api.ListConversationsResp{
			Pagination: util.PaginationRes(0, pg),
			Code:       0,
			Msg:        "success",
		}, nil
	}

	startIdx, endIdx := util.PagedIndex(total, pg)
	pageDates := dates[startIdx:endIdx]

	dateSet := make(map[string]bool, len(pageDates))
	for _, d := range pageDates {
		dateSet[d] = true
	}

	convs, err := c.ConversationMapper.FindByUserIdAndTimeRange(ctx, userId, start, end)
	if err != nil {
		return nil, errorx.New(errno.ErrListConversation)
	}

	// 同一日期下可能并存多位老师的会话，需逐一返回
	dateConvs := make(map[string][]*conversation.Conversation, len(pageDates))
	for _, conv := range convs {
		convDate := util.FormatDateUTC8(conv.CreateTime)
		if !dateSet[convDate] {
			continue
		}
		dateConvs[convDate] = append(dateConvs[convDate], conv)
	}

	result := make([]*core_api.ConversationVO, 0, len(pageDates))
	for _, date := range pageDates {
		dayConvs := dateConvs[date]
		// 同一天内按最近更新时间倒序
		sort.Slice(dayConvs, func(i, j int) bool { return dayConvs[i].UpdateTime.After(dayConvs[j].UpdateTime) })
		for _, conv := range dayConvs {
			result = append(result, &core_api.ConversationVO{
				ConversationId: conv.ID.Hex(),
				Brief:          conv.Title,
				CreateTime:     conv.CreateTime.Unix(),
				UpdateTime:     conv.UpdateTime.Unix(),
				Date:           date,
				CharacterId:    conv.CharacterID.Hex(),
			})
		}
	}

	return &core_api.ListConversationsResp{
		Pagination:       util.PaginationRes(total, pg),
		ConversationList: result,
		Code:             0,
		Msg:              "success",
	}, nil
}

// GetConvByDate 返回用户指定日期和角色的对话消息
func (c *ConversationService) GetConvByDate(ctx context.Context, req *core_api.GetConvByDateReq) (resp *core_api.GetConvByDateResp, err error) {
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

	userOID, err := bson.ObjectIDFromHex(userId)
	if err != nil {
		return nil, errorx.New(errno.ErrInvalidParams)
	}

	// 老师角色必填: 历史以「某位老师当天会话」为维度返回，不能跨老师混合
	if req.CharacterId == "" {
		return nil, errorx.New(errno.ErrInvalidParams, errorx.KV("field", "characterId"))
	}
	charOID, err := bson.ObjectIDFromHex(req.CharacterId)
	if err != nil {
		return nil, errorx.New(errno.ErrInvalidParams, errorx.KV("field", "characterId"))
	}

	// 日期默认为当天
	if req.Date == "" {
		req.Date = util.FormatDateUTC8(time.Now())
	}

	// 定位该老师当天的会话；该老师当天尚无会话时返回空列表
	conv, err := c.ConversationMapper.FindWritableByUserDateAndCharacter(ctx, userOID, req.Date, charOID)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return &core_api.GetConvByDateResp{
			Pagination:  util.PaginationRes(0, req.PaginationOptions),
			MessageList: []*core_api.ConvByDateMessage{},
			Code:        0,
			Msg:         "success",
		}, nil
	}
	if err != nil {
		return nil, errorx.New(errno.ErrGetConversation)
	}

	// 按 conversation_id 读取该会话的消息
	msgs, err := his.Mgr.RetrieveMessage(ctx, conv.ID.Hex(), -1)
	if err != nil {
		return nil, errorx.New(errno.ErrFetchMessages)
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
