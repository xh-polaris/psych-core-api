package core_api

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
	"github.com/xh-polaris/psych-core-api/biz/application/service"
	"github.com/xh-polaris/psych-core-api/pkg/httpx"
	"github.com/xh-polaris/psych-core-api/pkg/logs"
	"github.com/xh-polaris/psych-core-api/provider"
)

func DashboardChatReport(ctx context.Context, c *app.RequestContext) {
	var req struct {
		ConversationId string `json:"conversationId"`
		SessionId      string `json:"sessionId"`
		Message        string `json:"message"`
	}
	if err := c.BindAndValidate(&req); err != nil {
		c.String(consts.StatusBadRequest, err.Error())
		return
	}

	if req.ConversationId == "" {
		c.JSON(consts.StatusBadRequest, map[string]any{"code": 1, "msg": "conversationId 为空"})
		return
	}
	if req.Message == "" {
		c.JSON(consts.StatusBadRequest, map[string]any{"code": 1, "msg": "message 为空"})
		return
	}

	p := provider.Get()
	stream, err := p.ChatReportService.ChatReportStream(ctx, &service.ChatReportReq{
		ConversationId: req.ConversationId,
		SessionId:      req.SessionId,
		Message:        req.Message,
	})
	if err != nil {
		httpx.PostProcess(ctx, c, &req, nil, err)
		return
	}

	c.SetStatusCode(http.StatusOK)
	c.Response.Header.Set("Content-Type", "text/event-stream")
	c.Response.Header.Set("Cache-Control", "no-cache")
	c.Response.Header.Set("Connection", "keep-alive")

	first := true
	for {
		delta, finish, recvErr := stream.Recv()
		if recvErr != nil {
			logs.CtxErrorf(ctx, "[DashboardChatReport] stream recv error: %v", recvErr)
			data, _ := json.Marshal(service.ChatReportDelta{Finish: true, Delta: ""})
			c.Write([]byte("data: " + string(data) + "\n\n"))
			c.Flush()
			return
		}
		if first {
			delta.SessionId = req.ConversationId
			first = false
		}
		data, _ := json.Marshal(delta)
		c.Write([]byte("data: " + string(data) + "\n\n"))
		c.Flush()
		if finish {
			return
		}
	}
}
