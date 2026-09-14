package engine

import (
	"time"

	"github.com/xh-polaris/psych-core-api/pkg/core"
	"github.com/xh-polaris/psych-core-api/pkg/errorx"
	"github.com/xh-polaris/psych-core-api/pkg/logs"
	"github.com/xh-polaris/psych-core-api/pkg/wsx"
	"github.com/xh-polaris/psych-core-api/types/errno"
)

var (
	heartbeatTimeout  = time.Second * 30
	idleTimeout       = time.Minute * 10 // 空闲自动截断：超过该时长无对话活动即结束本次会话
	idleCheckInterval = time.Second * 30 // 空闲检查周期
)

// touchActive 刷新最近一次对话活动时间（用户命令发出 / 模型流式输出 / TTS 音频帧 / 模型回复结束）
func (e *Engine) touchActive() {
	e.lastActive.Store(time.Now().UnixNano())
}

// startIdleWatch 启动空闲看门狗，仅在认证成功后启动一次。
// 超过 idleTimeout 无对话活动时关闭连接，由 Close 完成会话收尾（写 start/end + 触发报表）。
func (e *Engine) startIdleWatch() {
	e.idleOnce.Do(func() {
		go e.watchIdle()
	})
}

func (e *Engine) watchIdle() {
	ticker := time.NewTicker(idleCheckInterval)
	defer ticker.Stop()
	for {
		select {
		case <-e.ctx.Done():
			return
		case <-ticker.C:
			if idle := time.Since(time.Unix(0, e.lastActive.Load())); idle >= idleTimeout {
				logs.Infof("[engine] %s idle for %s, auto close", e.uSession, idle)
				_ = e.Close()
				return
			}
		}
	}
}

// 应用层模拟的心跳消息
func (e *Engine) mockHeartbeat(ping *core.Ping) (err error) {
	if ping.Data != "" {
		logs.Infof("[engine] mock heartbeat: %s", ping.Data)
	}
	logs.Infof("[engine] [heartbeat] receive ping")
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
