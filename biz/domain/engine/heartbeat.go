package engine

import (
	"time"

	"github.com/xh-polaris/psych-core-api/biz/infra/util"
	"github.com/xh-polaris/psych-core-api/pkg/core"
	"github.com/xh-polaris/psych-core-api/pkg/errorx"
	"github.com/xh-polaris/psych-core-api/pkg/logs"
	"github.com/xh-polaris/psych-core-api/pkg/wsx"
	"github.com/xh-polaris/psych-core-api/types/errno"
)

var heartbeatTimeout = time.Second * 30

// 应用层模拟的心跳消息
func (e *Engine) mockHeartbeat(ping *core.Ping) (err error) {
	if ping.Data != "" {
		logs.Infof("[engine] mock heartbeat: %s", ping.Data)
	}
	util.DPrint("[engine] [heartbeat] receive ping\n")
	if err = e.wsx.Pong(nil); err != nil {
		logs.CondError(!wsx.IsNormal(err), "[engine] %s error %s", core.APong, err)
		return errorx.WrapByCode(err, errno.PongErr)
	}
	e.heartbeatTicker.Reset(heartbeatTimeout)
	return
}

// buildHeartbeat
//func buildHeartbeat(e *Engine) {
//e.wsx.SetPingHandler(func(appData string) (err error) { // 收到心跳消息的处理
//	if err = e.wsx.Pong(nil); err != nil {
//		logs.CondError(!wsx.IsNormal(err), "[engine] %s error %s", core.APong, err)
//	}
//	e.heartbeatTicker.Reset(heartbeatTimeout)
//	return nil
//})
//e.heartbeatTicker = time.NewTicker(heartbeatTimeout)
//go e.heartbeat()
//}

// heartbeat, 当心跳超时会heartbeatCh关闭时退出
//func (e *Engine) heartbeat() {
//	for {
//		select {
//		case <-e.ctx.Done(): // 其他原因结束
//			e.heartbeatTicker.Stop()
//			return
//		case <-e.heartbeatTicker.C: // 心跳超时
//			e.heartbeatTicker.Stop()
//			logs.Info("[engine] close by heartbeat")
//			_ = e.Close()
//			return
//		}
//	}
//}
