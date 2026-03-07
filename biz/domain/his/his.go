package his

import (
	"context"
	"sort"
	"strconv"
	"time"

	"github.com/bytedance/sonic"
	"github.com/xh-polaris/psych-core-api/biz/infra/cache"
	"github.com/xh-polaris/psych-core-api/biz/infra/mapper/conversation"
	"github.com/xh-polaris/psych-core-api/biz/infra/mapper/message"
	"github.com/xh-polaris/psych-core-api/pkg/errorx"
	"github.com/xh-polaris/psych-core-api/pkg/logs"
	"go.mongodb.org/mongo-driver/v2/bson"
)

var Mgr *HistoryManager

const cachePrefix = "psych:msg:"

// HistoryManager 历史记录管理, 所有的历史记录都按照从旧到新排序
type HistoryManager struct {
	cache      cache.Cmdable
	msgMapper  message.MongoMapper
	convMapper conversation.IMongoMapper
}

// New 创建一个新的历史记录管理器
func New(cache cache.Cmdable, msgMapper message.MongoMapper, convMapper conversation.IMongoMapper) {
	Mgr = &HistoryManager{
		cache:      cache,
		msgMapper:  msgMapper,
		convMapper: convMapper,
	}
}

// RetrieveMessage 获取消息, size 小于等于0时取出所有
func (h *HistoryManager) RetrieveMessage(ctx context.Context, id string, size int) (msgs []*message.Message, err error) {
	// retrieve cache
	if msgs, err = h.RetrieveMessageFromCache(ctx, cachePrefix+id); err == nil {
		if size >= 0 && len(msgs) > size {
			return msgs[:size], nil
		}
		return msgs, nil
	}
	// retrieve storage
	if msgs, err = h.msgMapper.RetrieveMessage(ctx, id, size); err != nil {
		return nil, err
	}
	// build cache
	if len(msgs) > 0 {
		if err = h.CacheMessage(ctx, cachePrefix+id, msgs); err != nil {
			logs.Errorf("cache msgs err: %s", err)
		}
	}
	return msgs, nil
}

// RetrieveMessageFromCache 从内存中获取
func (h *HistoryManager) RetrieveMessageFromCache(ctx context.Context, key string) ([]*message.Message, error) {
	result, err := h.cache.HGetAll(ctx, key).Result()
	if err != nil {
		return nil, err
	} else if len(result) == 0 {
		return nil, cache.Nil
	}

	msgs := make([]*message.Message, 0, len(result))
	for _, data := range result {
		var msg message.Message
		if err = sonic.Unmarshal([]byte(data), &msg); err != nil {
			logs.Errorf("[message mapper] listAllMsg: json.Unmarshal err:%s", errorx.ErrorWithoutStack(err))
			return nil, err
		}
		msgs = append(msgs, &msg)
	}
	if len(msgs) > 0 {
		sort.Slice(msgs, func(i, j int) bool { return msgs[i].Index > msgs[j].Index }) // 倒序
	}
	return msgs, nil
}

// CacheMessage 缓存一批历史记录
func (h *HistoryManager) CacheMessage(ctx context.Context, key string, msgs []*message.Message) (err error) {
	fields := make(map[string]string, len(msgs))
	for _, msg := range msgs {
		var data []byte
		if data, err = sonic.Marshal(msg); err != nil {
			return err
		}
		fields[key+strconv.Itoa(int(msg.Index))] = string(data)
	}
	p := h.cache.Pipeline()
	p.HSet(ctx, key, fields)
	p.Expire(ctx, key, time.Hour*6)

	_, err = p.Exec(ctx)
	return
}

// AddMessage 新增消息
func (h *HistoryManager) AddMessage(ctx context.Context, id string, msg *message.Message) (err error) {
	// add to storage
	if err = h.msgMapper.Insert(ctx, msg); err != nil {
		logs.Errorf("add message err: %s", err)
		return
	}

	// upsert conversation meta
	if msg != nil && h.convMapper != nil {
		convID := msg.ConversationId
		userID := msg.UserId
		if !convID.IsZero() && !userID.IsZero() {
			now := msg.CreateTime

			exists, errExist := h.convMapper.Exists(ctx, convID)
			if errExist != nil {
				logs.Errorf("check conversation exists err: %s", errorx.ErrorWithoutStack(errExist))
			} else if !exists {
				conv := &conversation.Conversation{
					ID:         convID,
					UserID:     userID,
					StartTime:  now,
					EndTime:    now,
					CreateTime: now,
					UpdateTime: now,
					Status:     0,
				}
				if errIns := h.convMapper.Insert(ctx, conv); errIns != nil {
					logs.Errorf("insert conversation err: %s", errorx.ErrorWithoutStack(errIns))
				}
			} else {
				update := bson.M{
					"endTime":    now,
					"updateTime": now,
				}
				if errUpd := h.convMapper.UpdateFields(ctx, convID, update); errUpd != nil {
					logs.Errorf("update conversation err: %s", errorx.ErrorWithoutStack(errUpd))
				}
			}
		}
	}

	// add to cache
	if err = h.CacheMessage(ctx, cachePrefix+id, []*message.Message{msg}); err != nil {
		logs.Errorf("cache msgs err: %s", err)
	}
	return
}
