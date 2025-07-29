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
	e.heartbeatTicker = time.NewTicker(heartbeatTimeout)
	e.heartbeatCh = core.NewChannel[struct{}](3, e.close)
	go e.heartbeat()
}

// heartbeat, 当心跳超时会heartbeatCh关闭时退出
func (e *Engine) heartbeat() {
	var ok bool
	var err error
	for {
		select {
		case <-e.heartbeatTicker.C: // 心跳超时
			e.heartbeatTicker.Stop()
			_ = e.Close()
			return
		case _, ok = <-e.heartbeatCh.C: // 收到心跳消息
			if !ok {
				return
			}
			e.heartbeatTicker.Reset(heartbeatTimeout)
			if err = e.wsx.Pong(); err != nil {
				logx.CondError(!wsx.IsNormal(err), "[engine] %s error %s", core.APong, err)
			}
		}
	}
}
