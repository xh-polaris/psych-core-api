package service

import (
	"context"
	"github.com/hertz-contrib/websocket"
	"github.com/xh-polaris/psych-core-api/biz/domain/engine"
)

// ChatHandler 处理长对话
func ChatHandler(ctx context.Context, conn *websocket.Conn) {
	// 初始化本轮对话的engine
	e := engine.NewEngine(ctx, conn)
	// 启动
	e.Run()
}
