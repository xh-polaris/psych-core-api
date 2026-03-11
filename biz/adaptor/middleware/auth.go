package middleware

import (
	"context"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/xh-polaris/psych-core-api/biz/cst"
	"github.com/xh-polaris/psych-core-api/biz/infra/util"
	"github.com/xh-polaris/psych-core-api/pkg/errorx"
	"github.com/xh-polaris/psych-core-api/pkg/httpx"
	"github.com/xh-polaris/psych-core-api/types/errno"
)

func StoreToken(ctx context.Context, c *app.RequestContext, req any) {
	authHeader := c.GetHeader("Authorization")
	if len(authHeader) == 0 {
		httpx.PostProcess(ctx, c, req, nil, errorx.New(errno.ErrUnAuth))
		c.Abort()
		return
	}

	// 验证JWT的有效性
	_, err := util.ParseJwt(string(authHeader))
	if err != nil {
		httpx.PostProcess(ctx, c, req, nil, errorx.New(errno.ErrJWTPrase))
		c.Abort()
		return
	}

	// 使用context.WithValue传递token
	newCtx := context.WithValue(ctx, cst.CtxKeyToken, string(authHeader))
	c.Set(cst.CtxKeyToken, newCtx)
	c.Next(ctx)
}
