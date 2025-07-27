package engine

import (
	"context"
	"github.com/hertz-contrib/websocket"
	"github.com/xh-polaris/psych-pkg/app"
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

	// AI Apps
	chat app.ChatApp
	tts  app.TTSApp
	asr  app.ASRApp
	core core.WorkFlow

	// uSession 对话标识
	uSession string

	// 消息派发
	broadcast   []core.CloseChannel
	heartbeatCh *core.Channel[struct{}]
	messageCh   *core.Channel[[]byte]
	cmdCh       *core.Channel[*core.Cmd]

	// 记录
	start time.Time         // 开始时间
	info  map[string]string // 基本信息
}

// NewEngine 创建一个新的对话引擎
func NewEngine(ctx context.Context, conn *websocket.Conn) *Engine {
	ctx, cancel := util.NNCtxWithCancel(ctx)
	e := &Engine{ctx: ctx, cancel: cancel, wsx: wsx.NewHZWSClient(conn), uSession: util.NewUID(), start: time.Now()}
	e.broadcast, e.close = make([]core.CloseChannel, 0, 3), make(chan struct{})
	buildHeartbeat(e)
	buildHandle(e)
	return e
}

// Run 运行对话引擎, 获取输入并派发消息
func (e *Engine) Run() {
	var mt int      // 消息类型
	var err error   // 错误
	var data []byte // 前端传入数据
	defer func() { _ = e.Close() }()

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
			if mt, data, err = e.read(); err != nil {
				return
			}
			switch mt {
			case websocket.PingMessage: // Ping消息
				e.heartbeatCh.Send(struct{}{})
			case websocket.TextMessage: // 文本消息
				logx.Info("[engine] receive text message:", string(data)) // 正常情况下不应该收到文本消息
			case websocket.BinaryMessage: // 二进制消息
				e.messageCh.Send(data)
			case websocket.CloseMessage: // TODO 关闭消息
			}
		}
	}
}

// init, 初始化, 主要与前端协商协议信息
func (e *Engine) init() (err error) {
	if err = e.wsx.WriteJSON(e.meta); err != nil {
		logx.CondError(!wsx.IsNormal(err), "[engine] protocol init error: %s", err)
	}
	return err
}

// read 读取输入并适时地记录日志
func (e *Engine) read() (mt int, data []byte, err error) {
	if mt, data, err = e.wsx.Read(); err != nil {
		logx.CondError(!wsx.IsNormal(err), "[engine] %s error %s", core.Read, err)
	}
	return
}

// write 写入编码后响应并适时地记录日志
func (e *Engine) write(msg []byte) {
	var err error
	if err = e.wsx.WriteBytes(msg); err != nil {
		logx.CondError(!wsx.IsNormal(err), "[engine] WriteBytes error: %s", err)
	}
	return
}

// mWrite 编码消息并写入响应
func (e *Engine) mWrite(t core.MType, payload any) {
	var err error
	var data []byte
	var m *core.Message

	if m, err = core.EncodeMessage(t, payload); err != nil {
		logx.Error("[engine] encode message error: %s", err)
		e.write(core.EncodeMsgErr)
		return
	}
	if data, err = core.MMarshal(m, e.meta.Compression, e.meta.Serialization); err != nil {
		logx.Info("[engine] Marshal message error: %s", err)
		e.write(core.EncodeMsgErr)
		return
	}
	e.write(data)
}

// Close 释放engine的资源
func (e *Engine) Close() (err error) {
	e.once.Do(func() {
		close(e.close)                   // 关闭channel, 避免再有消息写入
		for _, ch := range e.broadcast { // 关闭所有channel
			ch.Close()
		}
		e.cancel()
	})
	return err
}
