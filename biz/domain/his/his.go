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
)

var Mgr *HistoryManager

const (
	msgCachePrefix = "psych:msg:conv:"
	msgCacheTTL    = time.Hour * 6
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

// msgCacheKey 消息缓存 key：单会话维度（conversationId 唯一确定 用户+日期+老师角色）
func msgCacheKey(convId string) string {
	return msgCachePrefix + convId
}

func convDateCacheKey(convId string) string {
	return convDatePrefix + convId
}

func hashField(index int) string {
	return strconv.Itoa(index)
}

func (h *HistoryManager) GetUserDailyMessages(ctx context.Context, convId string, size int) ([]*message.Message, error) {
	key := msgCacheKey(convId)
	if msgs, err := h.RetrieveMessageFromCache(ctx, key); err == nil {
		if size > 0 && len(msgs) > size {
			return msgs[:size], nil
		}
		return msgs, nil
	}

	msgs, err := h.RetrieveMessage(ctx, convId, size)
	if err != nil {
		return nil, err
	}
	if len(msgs) > 0 && size <= 0 {
		if err = h.CacheMessages(ctx, convId, msgs); err != nil {
			logs.Errorf("[his] backfill msg cache err: %s", err)
			if derr := h.cache.Del(ctx, msgCacheKey(convId)).Err(); derr != nil {
				logs.Errorf("[his] invalidate msg cache err: %s", derr)
			}
		}
	}
	return msgs, nil
}

// RetrieveMessage 获取某个 conversation 的历史消息, 按 create_time 倒序
func (h *HistoryManager) RetrieveMessage(ctx context.Context, convId string, size int) ([]*message.Message, error) {
	if _, err := bson.ObjectIDFromHex(convId); err != nil {
		return nil, err
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
	// 显式排序：按时间倒序，越新的在前
	sort.Slice(msgs, func(i, j int) bool { return msgs[i].CreateTime.After(msgs[j].CreateTime) })
	return msgs, nil
}

// CacheMessages 缓存一批消息到 Redis（单会话维度）
func (h *HistoryManager) CacheMessages(ctx context.Context, convId string, msgs []*message.Message) error {
	key := msgCacheKey(convId)
	fields := make(map[string]string, len(msgs))
	for _, msg := range msgs {
		data, err := sonic.Marshal(msg)
		if err != nil {
			return err
		}
		fields[hashField(int(msg.Index))] = string(data)
	}
	p := h.cache.Pipeline()
	p.HSet(ctx, key, fields)
	p.Expire(ctx, key, msgCacheTTL)
	_, err := p.Exec(ctx)
	return err
}

// CacheConvDate 缓存 conversationId → userId:date 映射
func (h *HistoryManager) CacheConvDate(ctx context.Context, convId, userId string) error {
	date := util.FormatDateUTC8(time.Now())
	val := fmt.Sprintf("%s:%s", userId, date)
	return h.cache.Set(ctx, convDateCacheKey(convId), val, 7*24*time.Hour).Err()
}

// AddMessage 新增消息到存储并维护会话消息统计与消息缓存
func (h *HistoryManager) AddMessage(ctx context.Context, msg *message.Message) error {
	writable, err := h.convMapper.Exists(ctx, msg.ConversationId)
	if err != nil {
		return err
	}
	if !writable {
		return fmt.Errorf("conversation is deleted or does not exist: %s", msg.ConversationId.Hex())
	}

	if err := h.msgMapper.Insert(ctx, msg); err != nil {
		logs.Errorf("[his] add message err: %s", err)
		return err
	}
	if err := h.convMapper.RecordMessage(ctx, msg.ConversationId, msg.CreateTime); err != nil {
		logs.Errorf("[his] record conversation message err: %s", err)
		return err
	}

	convIdHex := msg.ConversationId.Hex()

	// 消息序号按会话内历史递增，不能用 Index==0 判断是否为新会话。
	// 任一消息成功写入后，只允许将 Pending 会话变为 Active；Deleted 会话不会被恢复
	_ = h.CacheConvDate(ctx, convIdHex, msg.UserId.Hex())
	if err := h.convMapper.ActivatePending(ctx, msg.ConversationId); err != nil {
		logs.Errorf("[his] activate conversation err: %s", err)
	}

	if err := h.CacheMessages(ctx, convIdHex, []*message.Message{msg}); err != nil {
		logs.Errorf("[his] cache message err: %s", err)
		if derr := h.cache.Del(ctx, msgCacheKey(convIdHex)).Err(); derr != nil {
			logs.Errorf("[his] invalidate msg cache err: %s", derr)
		}
	}

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
