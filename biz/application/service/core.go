package service

import (
	"context"

	"github.com/hertz-contrib/websocket"
	"github.com/xh-polaris/psych-core-api/biz/domain/engine"
)

// ChatHandler 处理长对话
func ChatHandler(ctx context.Context, conn *websocket.Conn) {
	engine.NewEngine(ctx, conn).Run()
}
