package httpx

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"strings"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/protocol"
	hertz "github.com/cloudwego/hertz/pkg/protocol/consts"
	"github.com/xh-polaris/psych-core-api/pkg/errorx"
	"github.com/xh-polaris/psych-core-api/pkg/logs"
	"go.opentelemetry.io/contrib/propagators/b3"
	"go.opentelemetry.io/otel/propagation"
)

const hertzContext = "hertz_context"

// httpx 是hertz框架的http处理package

type temErr struct {
	Code int32  `json:"code"`
	Msg  string `json:"msg"`
}

// PostProcess 处理http响应, resp要求指针或接口类型
// 在日志中记录本次调用详情, 同时向响应头中注入符合b3规范的链路信息, 主要是trace_id
// 最佳实践:
// - 在controller中调用业务处理, 处理结束后调用PostProcess
func PostProcess(ctx context.Context, c *app.RequestContext, req, resp any, err error) {
	b3.New().Inject(ctx, &headerProvider{headers: &c.Response.Header})
	logs.CtxInfof(ctx, "[%s] req=%s, resp=%s, err=%v", c.Path(), JSONF(req), JSONF(resp), err)

	// 无错, 正常响应
	if err == nil {
		response := makeResponse(resp)
		c.JSON(hertz.StatusOK, response)
		return
	}

	var custom errorx.StatusError
	if errors.As(err, &custom) { // errorx错误
		StatusCode := hertz.StatusOK
		c.JSON(StatusCode, &temErr{
			Code: custom.Code(),
			Msg:  custom.Msg(),
		})
	} else { // 常规错误, 状态码500
		logs.CtxErrorf(ctx, "internal error, err=%s", err.Error())
		code := hertz.StatusInternalServerError
		c.String(code, err.Error())
	}
}

// makeResponse 通过反射构造嵌套格式的响应体
func makeResponse(resp any) map[string]any {
	v := reflect.ValueOf(resp)
	if v.Kind() != reflect.Ptr || v.Elem().Kind() != reflect.Struct {
		return nil
	}
	// 构建返回数据
	v = v.Elem()
	response := map[string]any{
		"code": v.FieldByName("Code").Int(),
		"msg":  v.FieldByName("Msg").String(),
	}
	data := make(map[string]any)
	for i := 0; i < v.NumField(); i++ {
		field := v.Type().Field(i)
		if jsonTag := field.Tag.Get("json"); jsonTag != "" && field.Name != "Code" && field.Name != "Msg" {
			if fieldValue := v.Field(i).Interface(); !reflect.ValueOf(fieldValue).IsZero() || !strings.Contains(jsonTag, "omitempty") {
				data[strings.Split(jsonTag, ",")[0]] = fieldValue
			}
		}
	}
	if len(data) > 0 {
		response["data"] = data
	}
	return response
}

var _ propagation.TextMapCarrier = &headerProvider{}

type headerProvider struct {
	headers *protocol.ResponseHeader
}

// Get 按键读取响应头元数据。
func (m *headerProvider) Get(key string) string {
	return m.headers.Get(key)
}

// Set 按键值写入响应头元数据。
func (m *headerProvider) Set(key, value string) {
	m.headers.Set(key, value)
}

// Keys 遍历获取全部响应头元数据键。
func (m *headerProvider) Keys() []string {
	out := make([]string, 0)

	m.headers.VisitAll(func(key, value []byte) {
		out = append(out, string(key))
	})

	return out
}

func InjectContext(ctx context.Context, c *app.RequestContext) context.Context {
	return context.WithValue(ctx, hertzContext, c)
}

func ExtractContext(ctx context.Context) (*app.RequestContext, error) {
	c, ok := ctx.Value(hertzContext).(*app.RequestContext)
	if !ok {
		return nil, errors.New("hertz context not found")
	}
	return c, nil
}

// JSONF 将对象序列化成json格式字符串
func JSONF(v any) string {
	data, err := json.Marshal(v)
	if err != nil {
		logs.Error("JSONF fail, v=%v, err=%v", v, err)
	}
	return string(data)
}

// OpenAPIStatusError 是公开 OpenAPI 协议的错误契约。
// 它与 StatusError 分离，因为第三方 OpenAPI 客户端要求 HTTP 状态码和
// {error: ...} 格式的响应体。
type OpenAPIStatusError interface {
	HTTPStatus() int
	APIErrorCode() string
	APIErrorMessage() string
}

type openAPIErrorResponse struct {
	Error struct {
		Code      string `json:"code"`
		Message   string `json:"message"`
		Type      string `json:"type"`
		RequestID string `json:"request_id"`
	} `json:"error"`
}

// OpenAPIPostProcess 是 OpenAPI 对应的响应处理方法。
// 公开 OpenAPI 接口的响应契约固定，不能使用管理端的 {code,msg,data} 包装格式。
func OpenAPIPostProcess(c *app.RequestContext, requestID string, resp any, err error) {
	if requestID != "" {
		c.Response.Header.Set("X-Request-ID", requestID)
	}
	if err == nil {
		c.JSON(hertz.StatusOK, resp)
		return
	}
	var openAPIErr OpenAPIStatusError
	if !errors.As(err, &openAPIErr) {
		c.JSON(hertz.StatusInternalServerError, map[string]any{
			"error": map[string]string{
				"code":       "internal_error",
				"message":    "服务暂时不可用，请稍后重试",
				"type":       "api_error",
				"request_id": requestID,
			},
		})
		return
	}
	errType := "invalid_request_error"
	if openAPIErr.HTTPStatus() >= hertz.StatusInternalServerError || strings.HasPrefix(openAPIErr.APIErrorCode(), "upstream") {
		errType = "api_error"
	}
	result := openAPIErrorResponse{}
	result.Error.Code = openAPIErr.APIErrorCode()
	result.Error.Message = openAPIErr.APIErrorMessage()
	result.Error.Type = errType
	result.Error.RequestID = requestID
	c.JSON(openAPIErr.HTTPStatus(), result)
}
