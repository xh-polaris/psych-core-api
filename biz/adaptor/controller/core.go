package controller

import (
	"context"
	"github.com/cloudwego/hertz/pkg/app"
	"github.com/xh-polaris/psych-pkg/biz/application/service"
	"github.com/xh-polaris/psych-pkg/util/logx"
	"github.com/xh-polaris/psych-pkg/wsx"
)

// Chat 对话接口
// @router /chat [GET]
func Chat(ctx context.Context, c *app.RequestContext) {
	if err := wsx.UpgradeWs(ctx, c, service.ChatHandler); err != nil {
		logx.Error("[controller] [Chat] websocket upgrade error:", err)
	}
}
