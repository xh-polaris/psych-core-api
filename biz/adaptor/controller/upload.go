package controller

import (
	"context"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/xh-polaris/psych-core-api/biz/adaptor/middleware"
	"github.com/xh-polaris/psych-core-api/pkg/httpx"
	"github.com/xh-polaris/psych-core-api/provider"
)

func UploadImage(ctx context.Context, c *app.RequestContext) {
	middleware.StoreToken(ctx, c, nil)
	if c.IsAborted() {
		return
	}

	fileHeader, err := c.FormFile("file")
	if err != nil {
		c.JSON(200, map[string]any{
			"code": 1,
			"msg":  "缺少文件参数",
		})
		return
	}
	file, err := fileHeader.Open()
	if err != nil {
		c.JSON(200, map[string]any{
			"code": 1,
			"msg":  "文件读取失败",
		})
		return
	}
	defer file.Close()

	p := provider.Get()
	resp, err := p.FileService.UploadImage(ctx, fileHeader, file)
	httpx.PostProcess(ctx, c, nil, resp, err)
}

func UploadAvatar(ctx context.Context, c *app.RequestContext) {
	middleware.StoreToken(ctx, c, nil)
	if c.IsAborted() {
		return
	}
	fileHeader, err := c.FormFile("file")
	if err != nil {
		c.JSON(200, map[string]any{"code": 1, "msg": "缺少文件参数"})
		return
	}
	file, err := fileHeader.Open()
	if err != nil {
		c.JSON(200, map[string]any{"code": 1, "msg": "文件读取失败"})
		return
	}
	defer file.Close()
	resp, err := provider.Get().FileService.UploadAvatar(ctx, fileHeader, file)
	httpx.PostProcess(ctx, c, nil, resp, err)
}
