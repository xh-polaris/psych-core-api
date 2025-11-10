package engine

import (
	"context"
	"errors"
	"io"
	"sync"
	"time"

	"github.com/hertz-contrib/websocket"
	"github.com/xh-polaris/psych-core-api/biz/domain/his"
	"github.com/xh-polaris/psych-core-api/biz/infra/mq"
	"github.com/xh-polaris/psych-core-api/biz/infra/utils"
	"github.com/xh-polaris/psych-core-api/pkg/errorx"
	"github.com/xh-polaris/psych-pkg/app"
	"github.com/xh-polaris/psych-pkg/core"
	"github.com/xh-polaris/psych-pkg/util"
	"github.com/xh-polaris/psych-pkg/util/logx"
	"github.com/xh-polaris/psych-pkg/wsx"
)

// 目前当websocket层出现问题, engine会直接结束, 并未处理可恢复错误而是强制由客户端尝试重连

var meta = &core.Meta{
	Version:       core.Version,
	Serialization: core.JSON,
	Compression:   core.GZIP,
}

type Engine struct {
	ctx    context.Context    // ctx 上下文
	cancel context.CancelFunc // cancel 关闭所有的task线程
	once   sync.Once          // once 保证close操作只执行一次
	errs   chan error         // errs task线程错误收集

	// 应用
	asr app.ASRApp          // asr 管理文字转语音
	tts app.TTSApp          // tts 管理语言转文字
	llm app.ChatApp         // llm 管理大模型
	his *his.HistoryManager // his 管理历史记录

	// ws 与前端的websocket链接
	wsx             *wsx.HZWSClient // wsx 是与前端的连接
	meta            *core.Meta      // meta 是协议的元消息
	heartbeatTicker *time.Ticker    // heartbeatTicker 是心跳计时器

	// 记录
	start    time.Time      // 开始时间
	count    int            //对话轮数
	info     map[string]any // 基本信息
	isAuth   bool           // 是否认证
	uSession string         // uSession 对话标识
	conf     *core.Config
}

// NewEngine 创建一个新的对话引擎
func NewEngine(ctx context.Context, conn *websocket.Conn) *Engine {
	ctx, cancel := util.NNCtxWithCancel(ctx)
	e := &Engine{ctx: ctx, cancel: cancel, wsx: wsx.NewHZWSClient(conn), uSession: util.NewUID(),
		start: time.Now(), meta: meta, info: make(map[string]any), errs: make(chan error, 3)}
	//e.wsx.SetCloseHandler(func(code int, text string) (err error) { // 处理close消息
	//	if err = e.wsx.ControlClose(websocket.FormatCloseMessage(code, text)); err != nil { // 给客户端写回一个close消息
	//		logx.Error("[engine] [close] err: %s", err)
	//	}
	//	return e.Close()
	//})
	utils.DPrint("[engine] [new] with session %s\n", e.uSession) // debug
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
				logx.CondError(!wsx.IsNormal(err), "[engine] close by read error %s", err)
				e.unexpected(err, "read err")
				return
			}
			switch mt {
			case websocket.PingMessage: // Ping消息会直接在heartbeatHandler种处理
			case websocket.TextMessage: // 文本消息
				logx.Info("[engine] receive text message:", string(data)) // 正常情况下不应该收到文本消息
			case websocket.BinaryMessage: // 二进制消息
				e.unexpected(e.handle(data), "handler")
			case websocket.CloseMessage: // Close消息会直接在closeHandler中处理
			}
		}
	}
}

// init, 初始化, 主要与前端协商协议信息
func (e *Engine) init() (err error) {
	utils.DPrint("[engine] [init] meta: %+v\n", e.meta) //debug
	if err = e.wsx.WriteJSON(e.meta); err != nil {
		logx.CondError(!wsx.IsNormal(err), "[engine] protocol init error: %s\n", err) //debug
	}
	return err
}

// unexpected engine进程中的错误处理
func (e *Engine) unexpected(err error, cause string) {
	var custom errorx.StatusError
	if err != nil && (!errors.As(err, &custom) || custom.IsAffectStability()) && !wsx.IsNormal(err) { // 错误或影响稳定性
		utils.DPrint("[engine] [unexpected] err: %s,cause: %s\n", err, cause)
		_ = e.Close()
	}
	return
}

// Read 读取输入并适时地记录日志
func (e *Engine) Read() (mt int, data []byte, err error) {
	if mt, data, err = e.wsx.Read(); err != nil {
		logx.CondError(!wsx.IsNormal(err), "[engine] %s error %s", core.ARead, err)
	}
	return
}

// Write 写入编码后响应并适时地记录日志
func (e *Engine) Write(msg []byte) (err error) {
	if err = e.wsx.WriteBytes(msg); err != nil {
		logx.CondError(!wsx.IsNormal(err), "[engine] close by write error: %s", err)
	}
	return err
}

// MWrite 编码消息并写入响应
func (e *Engine) MWrite(t core.MType, payload any) (err error) {
	var data []byte
	var m *core.Message

	if m, err = core.EncodeMessage(t, payload); err != nil {
		logx.Error("[engine] encode message error: %s", err)
		return e.Write(core.EncodeMsgErr)
	}
	if data, err = core.MMarshal(m, e.meta.Compression, e.meta.Serialization); err != nil {
		logx.Info("[engine] Marshal message error: %s", err)
		return e.Write(core.EncodeMsgErr)
	}
	return e.Write(data)
}

// Close 释放engine的资源
func (e *Engine) Close() (err error) {
	e.once.Do(func() {
		// 关闭各个应用
		appClose(e.asr, e.llm, e.tts)
		// 关闭子线程
		e.cancel()
		// 关闭主线程的ws连接
		_ = e.wsx.Close()
		if err = mq.GetPostProducer().Produce(e.ctx, e.uSession, e.info, e.start, time.Now(), e.conf); err != nil {
			// 发送失败需要详细记录日志, 以进行后续托底
			logx.Error("[engine] produce notify error: %s with such state: session:%s start: %d end:%d info:%+v config:%+v", err, e.uSession, e.start, time.Now(), e.info, e.conf)
			return
		}
	})
	return err
}

func appClose(closers ...io.Closer) {
	for _, closer := range closers {
		if closer != nil {
			if err := closer.Close(); err != nil && !wsx.IsNormal(err) {
				logx.Error("[engine] close error: %v", err)
			}
		}
	}
}
