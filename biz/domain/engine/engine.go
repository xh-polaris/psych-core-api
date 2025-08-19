package engine

import (
	"context"
	"github.com/hertz-contrib/websocket"
	"github.com/xh-polaris/psych-core-api/biz/domain/mq"
	"github.com/xh-polaris/psych-core-api/biz/infra/utils"
	"github.com/xh-polaris/psych-pkg/core"
	"github.com/xh-polaris/psych-pkg/util"
	"github.com/xh-polaris/psych-pkg/util/logx"
	"github.com/xh-polaris/psych-pkg/wsx"
	"sync"
	"time"
)

// 目前当websocket层出现问题, engine会直接结束, 并未处理可恢复错误而是强制由客户端尝试重连

var _ core.Engine = (*Engine)(nil)

type Engine struct {
	// ctx 上下文
	ctx    context.Context
	cancel context.CancelFunc
	once   sync.Once
	close  chan struct{}

	// ws 与前端的websocket链接
	wsx             *wsx.HZWSClient
	meta            *core.Meta
	heartbeatTicker *time.Ticker

	// 工作流
	workflow core.WorkFlow

	// uSession 对话标识
	uSession string

	// 消息派发
	broadcast []core.CloseChannel
	messageCh *core.Channel[[]byte]
	cmdCh     *core.Channel[*core.Cmd]

	// 记录
	start time.Time      // 开始时间
	info  map[string]any // 基本信息
	conf  *core.Config
}

// NewEngine 创建一个新的对话引擎
func NewEngine(ctx context.Context, conn *websocket.Conn) *Engine {
	ctx, cancel := util.NNCtxWithCancel(ctx)
	e := &Engine{ctx: ctx, cancel: cancel, wsx: wsx.NewHZWSClient(conn), uSession: util.NewUID(), start: time.Now()}
	e.close = make(chan struct{})
	buildHeartbeat(e)
	buildHandle(e)
	buildAuth(e)
	buildCmd(e)
	e.wsx.SetCloseHandler(func(code int, text string) (err error) { // 处理close消息
		if err = e.wsx.ControlClose(websocket.FormatCloseMessage(code, text)); err != nil { // 给客户端写回一个close消息
			logx.Error("[engine] [close] err: %s", err)
		}
		return e.Close()
	})
	utils.DPrint("[engine] [new] with session %s\n", e.uSession) // debug
	return e
}

// Run 运行对话引擎, 获取输入并派发消息
func (e *Engine) Run() {
	var mt int      // 消息类型
	var err error   // 错误
	var data []byte // 前端传入数据

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
				return
			}
			switch mt {
			case websocket.PingMessage: // Ping消息会直接在heartbeatHandler种处理
			case websocket.TextMessage: // 文本消息
				logx.Info("[engine] receive text message:", string(data)) // 正常情况下不应该收到文本消息
			case websocket.BinaryMessage: // 二进制消息
				e.messageCh.Send(data)
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

// Read 读取输入并适时地记录日志
func (e *Engine) Read() (mt int, data []byte, err error) {
	if mt, data, err = e.wsx.Read(); err != nil {
		logx.CondError(!wsx.IsNormal(err), "[engine] %s error %s", core.ARead, err)
	}
	return
}

// Write 写入编码后响应并适时地记录日志
func (e *Engine) Write(msg []byte) {
	var err error
	if err = e.wsx.WriteBytes(msg); err != nil {
		if !wsx.IsNormal(err) {
			logx.Info("[engine] close by write error: %s", err)
			_ = e.Close()
		}
	}
	return
}

// MWrite 编码消息并写入响应
func (e *Engine) MWrite(t core.MType, payload any) {
	var err error
	var data []byte
	var m *core.Message

	if m, err = core.EncodeMessage(t, payload); err != nil {
		logx.Error("[engine] encode message error: %s", err)
		e.Write(core.EncodeMsgErr)
		return
	}
	if data, err = core.MMarshal(m, e.meta.Compression, e.meta.Serialization); err != nil {
		logx.Info("[engine] Marshal message error: %s", err)
		e.Write(core.EncodeMsgErr)
		return
	}
	e.Write(data)
}

// Close 释放engine的资源
func (e *Engine) Close() (err error) {
	e.once.Do(func() {
		e.cancel()
		close(e.close) // 关闭channel, 避免再有消息写入
		for _, ch := range e.broadcast {
			ch.Close() // 关闭子channel, 虽然这里send时也会自动关闭, 但是为了避免ch中无消息时一直空闲导致goroutine泄露, 还是手动关闭一次
		}
		_ = e.workflow.Close() // 关闭workflow
		_ = e.wsx.Close()
		if err = mq.GetPostProducer().Produce(e.ctx, e.uSession, e.info, e.start, time.Now(), e.conf); err != nil {
			// 发送失败需要详细记录日志, 以进行后续托底
			logx.Error("[engine] produce notify error: %s with such state: session:%s start: %d end:%d info:%+v config:%+v", err, e.uSession, e.start, time.Now(), e.info, e.conf)
			return
		}
	})
	return err
}

func (e *Engine) GetClose() chan struct{} {
	return e.close
}

func (e *Engine) Session() string {
	return e.uSession
}
