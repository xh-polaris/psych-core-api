package controller

import (
	"context"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/hertz-contrib/websocket"
	"github.com/xh-polaris/psych-core-api/biz/domain/engine"
	"github.com/xh-polaris/psych-core-api/pkg/logs"
	"github.com/xh-polaris/psych-core-api/pkg/wsx"
	"github.com/xh-polaris/psych-core-api/provider"
)

// Chat 对话接口
// @router /chat [GET]
func Chat(ctx context.Context, c *app.RequestContext) {
	handler := func(wsCtx context.Context, conn *websocket.Conn) {
		p := provider.Get()
		engine.NewEngine(wsCtx, conn, &p.UserService, &p.ConfigService).Run()
	}
	if err := wsx.UpgradeWs(ctx, c, handler); err != nil {
		logs.Error("[controller] [Chat] websocket upgrade error:", err)
	}
}
