package engine

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"sync"
	"time"

	"github.com/xh-polaris/psych-core-api/biz/application/service"
	"github.com/xh-polaris/psych-core-api/biz/domain/his"
	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/cloudwego/eino/components/model"
	"github.com/hertz-contrib/websocket"
	"github.com/xh-polaris/psych-core-api/biz/conf"
	"github.com/xh-polaris/psych-core-api/biz/cst"
	"github.com/xh-polaris/psych-core-api/biz/infra/lock"
	"github.com/xh-polaris/psych-core-api/biz/infra/mapper/conversation"
	"github.com/xh-polaris/psych-core-api/biz/infra/mq"
	"github.com/xh-polaris/psych-core-api/biz/infra/util"
	"github.com/xh-polaris/psych-core-api/pkg/app"
	"github.com/xh-polaris/psych-core-api/pkg/core"
	"github.com/xh-polaris/psych-core-api/pkg/errorx"
	"github.com/xh-polaris/psych-core-api/pkg/logs"
	"github.com/xh-polaris/psych-core-api/pkg/wsx"
	"github.com/xh-polaris/psych-core-api/types/errno"
)

// 目前当websocket层出现问题, engine会直接结束, 并未处理可恢复错误而是强制由客户端尝试重连

var meta = &core.Meta{
	Version:       core.Version,
	Serialization: core.JSON,
	Compression:   core.GZIP,
}

const must = "[must]"

type Engine struct {
	ctx    context.Context       // ctx 上下文
	cancel context.CancelFunc    // cancel 关闭所有的task线程
	once   sync.Once             // once 保证close操作只执行一次
	errs   chan error            // errs task线程错误收集
	lock   lock.DistributionLock // 分布式锁, 确保一个用户只有一个进行中对话

	// 应用
	asr       app.ASRApp                 // asr 管理文字转语音
	tts       app.TTSApp                 // tts 管理语言转文字
	llm       model.ToolCallingChatModel // llm 管理大模型
	llmCancel context.CancelFunc         // 用于中断大模型输出
	llmWg     sync.WaitGroup             // llmWg 中断时等待各子线程退出

	// ws 与前端的websocket链接
	wsx             *wsx.HZWSClient // wsx 是与前端的连接
	meta            *core.Meta      // meta 是协议的元消息
	heartbeatTicker *time.Ticker    // heartbeatTicker 是心跳计时器

	// 记录
	start        time.Time      // 开始时间
	count        int            // 对话轮数 (当前会话新产生的)
	initialCount int            // 初始消息总数
	info         map[string]any // 基本信息
	isAuth       bool           // 是否认证
	uSession     string         // uSession 对话ID
	usage        *core.Usage    // 用量
	conf         *core.Config

	usrSvc     *service.UserService
	cfgSvc     *service.ConfigService
	unitSvc    *service.UnitService
	convMapper conversation.IMongoMapper
}

// NewEngine 创建一个新的对话引擎
func NewEngine(ctx context.Context, conn *websocket.Conn, usrSvc *service.UserService, cfgSvc *service.ConfigService, convMapper conversation.IMongoMapper) *Engine {
	ctx, cancel := context.WithCancel(ctx)
	e := &Engine{
		ctx: ctx, cancel: cancel, wsx: wsx.NewHZWSClient(conn), usage: &core.Usage{}, heartbeatTicker: time.NewTicker(heartbeatTimeout),
		start: time.Now(), meta: meta, info: make(map[string]any), errs: make(chan error, 3),
		usrSvc: usrSvc, cfgSvc: cfgSvc, convMapper: convMapper,
	}
	//e.wsx.SetCloseHandler(func(code int, text string) (err error) { // 处理close消息
	//	if err = e.wsx.ControlClose(websocket.FormatCloseMessage(code, text)); err != nil { // 给客户端写回一个close消息
	//		logs.Error("[engine] [close] err: %s", err)
	//	}
	//	return e.Close()
	//})
	util.DPrint("[engine] [new] with session %s\n", e.uSession) // debug
	return e
}

// Run 运行对话引擎, 获取输入并派发消息
func (e *Engine) Run() {
	var mt int      // 消息类型
	var data []byte // 前端传入数据
	var err error
	// 协议协商
	if err = e.init(); err != nil {
		return
	}
	for {
		select {
		case <-e.ctx.Done():
			return
		default:
			// 从客户端读取信息
			if mt, data, err = e.Read(); err != nil {
				logs.CondError(!wsx.IsNormal(err), "[engine] close by read error %s", err)
				e.unexpected(err, "[must] read err")
				return
			}
			switch mt {
			case websocket.PingMessage: // Ping消息会直接在heartbeatHandler种处理
			case websocket.TextMessage: // 文本消息
				logs.Info("[engine] receive text message:", string(data)) // 正常情况下不应该收到文本消息
			case websocket.BinaryMessage: // 二进制消息
				e.unexpected(e.handle(data), "handler")
			case websocket.CloseMessage: // Close消息会直接在closeHandler中处理
			}
		}
	}
}

// init, 初始化, 主要与前端协商协议信息
func (e *Engine) init() (err error) {
	util.DPrint("[engine] [init] meta: %+v\n", e.meta) //debug
	if err = e.wsx.WriteJSON(e.meta); err != nil {
		logs.CondError(!wsx.IsNormal(err), "[engine] protocol init error: %s\n", err) //debug
	}
	return err
}

// unexpected engine进程中的错误处理
func (e *Engine) unexpected(err error, cause string) bool {
	var custom errorx.StatusError
	if errors.As(err, &custom) {
		if err = e.MWrite(core.MErr, core.ToErr(custom)); err != nil {
			e.unexpected(err, cause)
		}
		if custom.IsAffectStability() {
			util.DPrint("%s [engine] [unexpected] at: %s err: %s, cause: %s\n", time.Now().String(),
				util.CallerInfo(2), err, cause)
			_ = e.Close()
			return true
		}
	} else if (err != nil && !wsx.IsNormal(err)) || strings.HasPrefix(cause, must) { // 错误或影响稳定性
		util.DPrint("%s [engine] [unexpected] at: %s err: %s,cause: %s\n", time.Now().String(),
			util.CallerInfo(2), err, cause)
		fmt.Println(err)
		_ = e.Close()
		return true
	}
	return false
}

// Read 读取输入并适时地记录日志
func (e *Engine) Read() (mt int, data []byte, err error) {
	if mt, data, err = e.wsx.Read(); err != nil {
		logs.CondError(!wsx.IsNormal(err), "[engine] %s error %s", core.ARead, err)
	}
	return
}

// Write 写入编码后响应并适时地记录日志
func (e *Engine) Write(msg []byte) (err error) {
	if err = e.wsx.WriteBytes(msg); err != nil {
		logs.CondError(!wsx.IsNormal(err), "[engine] close by write error: %s", err)
	}
	return err
}

// MWrite 编码消息并写入响应
func (e *Engine) MWrite(t core.MType, payload any) (err error) {
	var data []byte
	var m *core.Message

	if m, err = core.EncodeMessage(t, payload); err != nil {
		logs.Error("[engine] encode message error: %s", err)
		return e.Write(core.EncodeMsgErr)
	}
	if data, err = core.MMarshal(m, e.meta.Compression, e.meta.Serialization); err != nil {
		logs.Info("[engine] Marshal message error: %s", err)
		return e.Write(core.EncodeMsgErr)
	}
	return e.Write(data)
}

// Lock 锁定
func (e *Engine) Lock() error {
	if conf.GetConfig().State == "test" {
		return nil
	}
	if e.lock == nil {
		userId := e.getID(e.info, cst.JsonUserID)
		if userId == "" {
			return errorx.New(errno.ErrUnAuth)
		}
		e.lock = lock.Mgr.NewLock(userId)
	}
	if ok, err := e.lock.TryLock(e.ctx, time.Minute*3, time.Second*90, time.Minute*2); err != nil {
		return errorx.WrapByCode(err, errno.UnKnown)
	} else if !ok {
		return errorx.New(errno.ExistConn)
	}
	return nil
}

func (e *Engine) Unlock() error {
	if conf.GetConfig().State == "test" {
		return nil
	}
	if err := e.lock.TryUnlock(e.ctx); err != nil {
		return errorx.WrapByCode(err, errno.UnKnown)
	}
	return nil
}

// Close 释放engine的资源
func (e *Engine) Close() (err error) {
	util.DPrint("[engine] %s closed by %s", e.uSession, util.CallerInfo(2))
	e.once.Do(func() {
		// 关闭各个应用, llm无需关闭
		appClose(e.asr, e.tts)
		if err = e.Unlock(); err != nil {
			logs.Error("[engine] unlock err: %s", err)
		}
		// 关闭子线程并等待归档任务完成
		e.cancel()
		e.llmWg.Wait()

		// 关闭主线程的ws连接
		_ = e.wsx.Close()

		// 只有认证过的连接才进行后处理
		if !e.isAuth {
			return
		}

		// 使用背景上下文进行最后的操作, 避免受到e.ctx被cancel的影响
		pCtx, pCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer pCancel()

		// 检查数据库中会话是否依然有效
		if oid, err := bson.ObjectIDFromHex(e.uSession); err == nil {
			active, _ := e.convMapper.IsActive(pCtx, oid)
			if !active {
				logs.Infof("[engine] %s is not active in DB, skip post process", e.uSession)
				return
			}

			// 再次查询最新的消息总数以确定是否有变化
			latestMsgs, _ := his.Mgr.RetrieveMessage(pCtx, e.uSession, 0)
			currentTotal := len(latestMsgs)

			// 只有消息数增加了，才执行更新和 MQ
			if currentTotal <= e.initialCount && e.count == 0 {
				logs.Infof("[engine] %s message count no change (%d -> %d), skip post process", e.uSession, e.initialCount, currentTotal)
				return
			}

			// 更新会话信息 (时间、消息数)
			update := bson.M{
				cst.StartTime:    e.start,
				cst.EndTime:      time.Now(),
				cst.MessageCount: currentTotal,
			}
			if err = e.convMapper.UpdateFields(pCtx, oid, update); err != nil {
				logs.Error("[engine] update conversation time err: %v", err)
			}
		}

		// 发布 MQ 通知
		if err = mq.GetPostProducer().Produce(pCtx, e.buildPostNotify(time.Now())); err != nil {
			// 发送失败需要详细记录日志, 以进行后续托底
			logs.Error("[engine] produce notify error: %s with such state: session:%s start: %d end:%d info:%+v config:%+v", err, e.uSession, e.start, time.Now(), e.info, e.conf)
			return
		}
	})
	return nil
}

// buildPostNotify 构造后处理消息体
func (e *Engine) buildPostNotify(end time.Time) *core.PostNotify {
	return &core.PostNotify{
		Session: e.uSession,
		// 根层级的 UserId 和 UnitId 留空，因为它们已经存在于 Info 字典中了
		Usage:  e.usage,
		Info:   e.info,
		Start:  e.start.Unix(),
		End:    end.Unix(),
		Config: e.conf,
	}
}

// getID 强效提取 ID 逻辑 (兼容 string 和 ObjectID)
func (e *Engine) getID(m map[string]any, key string) string {
	val, ok := m[key]
	if !ok || val == nil {
		return ""
	}
	switch v := val.(type) {
	case string:
		// 只有长度为 24 且是合法 Hex 的字符串才接受
		if len(v) == 24 {
			return v
		}
	case bson.ObjectID:
		return v.Hex()
	case *bson.ObjectID:
		if v != nil {
			return v.Hex()
		}
	}
	return ""
}

func appClose(closers ...io.Closer) {
	for _, closer := range closers {
		if closer != nil {
			if err := closer.Close(); err != nil && !wsx.IsNormal(err) {
				logs.Error("[engine] close error: %v", err)
			}
		}
	}
}
