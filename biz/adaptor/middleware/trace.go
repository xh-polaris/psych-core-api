package middleware

import (
	"context"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/google/uuid"
	"go.opentelemetry.io/otel/trace"
)

func SetLogIDMW() app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		var traceID, spanID string
		spanCtx := trace.SpanContextFromContext(ctx)
		if spanCtx.IsValid() {
			traceID = spanCtx.TraceID().String()
			spanID = spanCtx.SpanID().String()
		}

		// 独立 log_id
		logID := traceID
		if logID == "" {
			logID = uuid.New().String()
		}

		// 注入 context
		ctx = context.WithValue(ctx, "log-id", logID)
		ctx = context.WithValue(ctx, "trace-id", traceID)
		ctx = context.WithValue(ctx, "span-id", spanID)

		// 设置响应头
		c.Header("X-Log-ID", logID)
		if traceID != "" {
			c.Header("X-Trace-ID", traceID)
		}

		c.Next(ctx)
	}
}
