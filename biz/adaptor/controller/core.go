package controller

import (
	"context"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/xh-polaris/psych-core-api/biz/application/service"
	"github.com/xh-polaris/psych-core-api/pkg/logs"
	"github.com/xh-polaris/psych-core-api/pkg/wsx"
)

// Chat 对话接口
// @router /chat [GET]
func Chat(ctx context.Context, c *app.RequestContext) {
	if err := wsx.UpgradeWs(ctx, c, service.ChatHandler); err != nil {
		logs.Error("[controller] [Chat] websocket upgrade error:", err)
	}
}
