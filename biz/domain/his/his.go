package his

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/bytedance/sonic"
	"github.com/xh-polaris/psych-core-api/biz/infra/cache"
	"github.com/xh-polaris/psych-core-api/biz/infra/mapper/conversation"
	"github.com/xh-polaris/psych-core-api/biz/infra/mapper/message"
	"github.com/xh-polaris/psych-core-api/biz/infra/util"
	"github.com/xh-polaris/psych-core-api/pkg/errorx"
	"github.com/xh-polaris/psych-core-api/pkg/logs"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

var Mgr *HistoryManager

const (
	cachePrefix    = "psych:msg:"
	convDatePrefix = "psych:conv:date:"
)

type HistoryManager struct {
	cache      cache.Cmdable
	msgMapper  message.IMongoMapper
	convMapper conversation.IMongoMapper
}

func New(c cache.Cmdable, msgMapper message.IMongoMapper, convMapper conversation.IMongoMapper) {
	Mgr = &HistoryManager{
		cache:      c,
		msgMapper:  msgMapper,
		convMapper: convMapper,
	}
}

func dailyCacheKey(userId, date string) string {
	return cachePrefix + userId + ":" + date
}

func convDateCacheKey(convId string) string {
	return convDatePrefix + convId
}

func hashField(convId string, index int) string {
	return convId + ":" + strconv.Itoa(index)
}

// GetUserDailyMessages 获取用户某天内所有消息 (带缓存)
func (h *HistoryManager) GetUserDailyMessages(ctx context.Context, userId, date string) ([]*message.Message, error) {
	key := dailyCacheKey(userId, date)
	if msgs, err := h.RetrieveMessageFromCache(ctx, key); err == nil {
		return msgs, nil
	}

	userOid, err := bson.ObjectIDFromHex(userId)
	if err != nil {
		return nil, err
	}

	start, end, err := util.DayToUTCRange(date)
	if err != nil {
		return nil, err
	}

	convs, err := h.convMapper.FindByUserIdAndTimeRange(ctx, userOid, start, end)
	if err != nil {
		return nil, err
	}
	if len(convs) == 0 {
		return []*message.Message{}, nil
	}

	convIds := make([]bson.ObjectID, len(convs))
	for i, conv := range convs {
		convIds[i] = conv.ID
	}

	msgs, err := h.msgMapper.FindByConversationIds(ctx, convIds, options.Find().SetSort(bson.M{"create_time": 1}))
	if err != nil {
		return nil, err
	}

	if len(msgs) > 0 {
		_ = h.CacheDailyMessages(ctx, userId, date, msgs)
	}

	sort.Slice(msgs, func(i, j int) bool { return msgs[i].CreateTime.Before(msgs[j].CreateTime) })
	return msgs, nil
}

// RetrieveMessage 获取某个 conversation 的历史消息 (向后兼容, 仅用于 GetConversation)
func (h *HistoryManager) RetrieveMessage(ctx context.Context, convId string, size int) ([]*message.Message, error) {
	oid, err := bson.ObjectIDFromHex(convId)
	if err != nil {
		return nil, err
	}

	conv, err := h.convMapper.FindOneById(ctx, oid)
	if err != nil || conv == nil {
		return h.msgMapper.RetrieveMessage(ctx, convId, size)
	}

	date := util.FormatDateUTC8(conv.CreateTime)
	userId := conv.UserID.Hex()

	key := dailyCacheKey(userId, date)
	if cached, cacheErr := h.RetrieveMessageFromCache(ctx, key); cacheErr == nil {
		filtered := make([]*message.Message, 0)
		for _, m := range cached {
			if m.ConversationId.Hex() == convId {
				filtered = append(filtered, m)
			}
		}
		if size > 0 && len(filtered) > size {
			return filtered[:size], nil
		}
		return filtered, nil
	}

	return h.msgMapper.RetrieveMessage(ctx, convId, size)
}

// RetrieveMessageFromCache 从 Redis 缓存中获取消息
func (h *HistoryManager) RetrieveMessageFromCache(ctx context.Context, key string) ([]*message.Message, error) {
	result, err := h.cache.HGetAll(ctx, key).Result()
	if err != nil {
		return nil, err
	}
	if len(result) == 0 {
		return nil, cache.Nil
	}

	msgs := make([]*message.Message, 0, len(result))
	for _, data := range result {
		var msg message.Message
		if err = sonic.Unmarshal([]byte(data), &msg); err != nil {
			logs.Errorf("[his] unmarshal msg err: %s", errorx.ErrorWithoutStack(err))
			return nil, err
		}
		msgs = append(msgs, &msg)
	}
	if len(msgs) > 0 {
		sort.Slice(msgs, func(i, j int) bool { return msgs[i].CreateTime.Before(msgs[j].CreateTime) })
	}
	return msgs, nil
}

// CacheDailyMessages 缓存一批消息到 Redis (按 userId:date 维度)
func (h *HistoryManager) CacheDailyMessages(ctx context.Context, userId, date string, msgs []*message.Message) error {
	key := dailyCacheKey(userId, date)
	fields := make(map[string]string, len(msgs))
	for _, msg := range msgs {
		data, err := sonic.Marshal(msg)
		if err != nil {
			return err
		}
		fields[hashField(msg.ConversationId.Hex(), int(msg.Index))] = string(data)
	}
	p := h.cache.Pipeline()
	p.HSet(ctx, key, fields)
	p.Expire(ctx, key, time.Hour*6)
	_, err := p.Exec(ctx)
	return err
}

// CacheConvDate 缓存 conversationId → userId:date 映射
func (h *HistoryManager) CacheConvDate(ctx context.Context, convId, userId string) error {
	date := util.FormatDateUTC8(time.Now())
	val := fmt.Sprintf("%s:%s", userId, date)
	return h.cache.Set(ctx, convDateCacheKey(convId), val, 7*24*time.Hour).Err()
}

// AddMessage 新增消息到存储并更新缓存 (userId:date 维度)
func (h *HistoryManager) AddMessage(ctx context.Context, userId, date string, msg *message.Message) error {
	if err := h.msgMapper.Insert(ctx, msg); err != nil {
		logs.Errorf("[his] add message err: %s", err)
		return err
	}

	convIdHex := msg.ConversationId.Hex()

	if msg.Index == 0 {
		_ = h.CacheConvDate(ctx, convIdHex, userId)
		if err := h.convMapper.SetActive(ctx, msg.ConversationId); err != nil {
			logs.Errorf("[his] activate conversation err: %s", err)
		}
	}

	_ = h.CacheDailyMessages(ctx, userId, date, []*message.Message{msg})

	util.DPrint("[his] add message, conv=%s, index=%d\n", convIdHex, msg.Index)
	return nil
}

// GetConvDateInfo 从缓存或DB获取 conversation 对应的 userId 和 date
func (h *HistoryManager) GetConvDateInfo(ctx context.Context, convId string) (userId, date string, err error) {
	val, err := h.cache.Get(ctx, convDateCacheKey(convId)).Result()
	if err == nil && val != "" {
		parts := strings.SplitN(val, ":", 2)
		if len(parts) == 2 {
			return parts[0], parts[1], nil
		}
	}

	oid, err := bson.ObjectIDFromHex(convId)
	if err != nil {
		return "", "", err
	}
	conv, err := h.convMapper.FindOneById(ctx, oid)
	if err != nil || conv == nil {
		return "", "", fmt.Errorf("conv not found: %s", convId)
	}
	return conv.UserID.Hex(), util.FormatDateUTC8(conv.CreateTime), nil
}
