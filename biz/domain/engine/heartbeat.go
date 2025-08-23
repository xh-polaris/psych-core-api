package engine

import (
	"github.com/xh-polaris/psych-pkg/core"
	"github.com/xh-polaris/psych-pkg/util/logx"
	"github.com/xh-polaris/psych-pkg/wsx"
	"time"
)

var heartbeatTimeout = time.Second * 30

// buildHeartbeat
func buildHeartbeat(e *Engine) {
	e.wsx.SetPingHandler(func(appData string) (err error) { // 收到心跳消息的处理
		//utils.DPrint("[engine] heartbeat\n") // Debug
		if err = e.wsx.Pong(nil); err != nil {
			logx.CondError(!wsx.IsNormal(err), "[engine] %s error %s", core.APong, err)
		}
		e.heartbeatTicker.Reset(heartbeatTimeout)
		return nil
	})
	e.heartbeatTicker = time.NewTicker(heartbeatTimeout)
	go e.heartbeat()
}

// heartbeat, 当心跳超时会heartbeatCh关闭时退出
func (e *Engine) heartbeat() {
	for {
		select {
		case <-e.ctx.Done(): // 其他原因结束
			e.heartbeatTicker.Stop()
			return
		case <-e.heartbeatTicker.C: // 心跳超时
			e.heartbeatTicker.Stop()
			logx.Info("[engine] close by heartbeat")
			_ = e.Close()
			return
		}
	}
}

// 应用层模拟的心跳消息
func (e *Engine) mockHeartbeat(payload []byte) {
	if payload != nil && len(payload) > 0 {
		logx.Info("[engine] mock heartbeat: %s", string(payload))
	}
	if err := e.wsx.Pong(nil); err != nil {
		logx.CondError(!wsx.IsNormal(err), "[engine] %s error %s", core.APong, err)
	}
	e.heartbeatTicker.Reset(heartbeatTimeout)
}
