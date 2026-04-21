package util

import (
	"fmt"
	"net/http"
	"net/http/httputil"

	"github.com/xh-polaris/psych-core-api/biz/conf"
)

func NewDebugTransport() http.RoundTripper {
	if conf.GetConfig().State == "debug" {
		return NewLoggingTransport()
	}
	return http.DefaultTransport
}

// LoggingTransport 是一个自定义 Transport，用于打印 HTTP 请求和响应
type LoggingTransport struct {
	Transport http.RoundTripper
}

func NewLoggingTransport(next ...http.RoundTripper) *LoggingTransport {
	rt := http.DefaultTransport
	if len(next) > 0 && next[0] != nil {
		rt = next[0]
	}
	return &LoggingTransport{
		Transport: rt,
	}
}

// RoundTrip 实现 http.RoundTripper 接口，拦截请求和响应
func (t *LoggingTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	// 打印请求
	dumpReq, err := httputil.DumpRequestOut(req, true) // true 表示包含 Body
	if err != nil {
		return nil, err
	}
	fmt.Println("===== HTTP Request =====")
	fmt.Println(string(dumpReq))
	fmt.Println("=======================")

	// 使用底层 Transport 发送请求
	resp, err := t.Transport.RoundTrip(req)
	if err != nil {
		return nil, err
	}

	// 打印响应
	dumpResp, err := httputil.DumpResponse(resp, true) // true 表示包含 Body
	if err != nil {
		return nil, err
	}
	fmt.Println("===== HTTP Response =====")
	fmt.Println(string(dumpResp))
	fmt.Println("========================")

	return resp, nil
}
