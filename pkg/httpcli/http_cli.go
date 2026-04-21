package httpcli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
)

const (
	defaultErrorBodyLimit = 8 * 1024 // 8KB
)

// Client 是统一 HTTP 基座。
// - 复用 Transport / 连接池
// - 通过 context 传播 trace / timeout / cancel
type Client struct {
	httpClient *http.Client
}

// Option 用于自定义 Client
type Option func(*Client)

// WithHTTPClient 注入自定义 http.Client
// 注意：如果你自己传入的 client 没有 otel transport，那 trace 自动注入就由你自己负责
func WithHTTPClient(hc *http.Client) Option {
	return func(c *Client) {
		if hc != nil {
			c.httpClient = hc
		}
	}
}

// WithTransport 设置底层 Transport
func WithTransport(rt http.RoundTripper) Option {
	return func(c *Client) {
		if rt == nil {
			return
		}
		c.httpClient.Transport = rt
	}
}

// WithBaseTransport 设置底层 Transport，并自动添加 OpenTelemetry instrumentation
func WithBaseTransport(rt http.RoundTripper) Option {
	return func(c *Client) {
		if rt == nil {
			rt = http.DefaultTransport
		}
		c.httpClient.Transport = otelhttp.NewTransport(rt)
	}
}

var (
	defaultClient *Client
	defaultOnce   sync.Once
)

// Default 返回全局单例，适合大多数工具直接复用。
func Default() *Client {
	defaultOnce.Do(func() {
		defaultClient = New()
	})
	return defaultClient
}

// DefaultHttpClient 兼容旧命名。
func DefaultHttpClient() *Client {
	return Default()
}

// New 创建独立 Client。
func New(opts ...Option) *Client {
	c := &Client{
		httpClient: &http.Client{
			Transport: otelhttp.NewTransport(http.DefaultTransport),
		},
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

// NewHttpClient 兼容旧命名。
func NewHttpClient(opts ...Option) *Client {
	return New(opts...)
}

// Request 是核心请求模型。
type Request struct {
	Method string
	URL    string

	Headers http.Header
	Query   url.Values

	// Body 和 JSON 只能二选一。
	Body io.Reader
	JSON any

	// Timeout 为单次请求超时，优先通过 context.WithTimeout 实现。
	Timeout time.Duration

	// CheckRedirect 为本次请求的 redirect hook。
	// 用于 file_read 这类需要逐跳校验安全性的场景。
	CheckRedirect func(req *http.Request, via []*http.Request) error

	// MaxResponseBytes 是 ReadAll/DecodeJSON 的默认读取上限。
	// 0 表示不限制。
	MaxResponseBytes int64

	// ExpectedStatus 自定义成功状态判断。
	// 为 nil 时默认 2xx。
	ExpectedStatus func(code int) bool

	// Retry 为可选重试策略。默认不重试。
	Retry *RetryPolicy
}

// HTTPClient 兼容 COS 直接取底层 Transport
func (c *Client) HTTPClient() *http.Client {
	if c == nil {
		return nil
	}
	return c.httpClient
}

// GetHttpClient 兼容旧命名
func GetHttpClient() *Client {
	return Default()
}

// RetryPolicy 为可选重试策略。
// 注意：
// - 默认不启用，只有明确配置才会重试。
// - 对带请求体的重试，JSON 请求没问题；原始 Body 若不可重放，则无法可靠重试。
type RetryPolicy struct {
	MaxRetries int

	// Backoff 传入第几次重试（从 1 开始），返回等待时长。
	// nil 时默认不等待。
	Backoff func(attempt int) time.Duration

	// ShouldRetry 决定当前错误/响应是否值得重试。
	// 例如：网络错误、5xx。
	ShouldRetry func(resp *http.Response, err error) bool
}

// DefaultRetryPolicy 提供一个常用默认策略：网络错误或 5xx 重试
func DefaultRetryPolicy(maxRetries int, baseBackoff time.Duration) *RetryPolicy {
	if maxRetries < 0 {
		maxRetries = 0
	}
	return &RetryPolicy{
		MaxRetries: maxRetries,
		Backoff: func(attempt int) time.Duration {
			if baseBackoff <= 0 {
				return 0
			}
			return time.Duration(attempt) * baseBackoff
		},
		ShouldRetry: func(resp *http.Response, err error) bool {
			if err != nil {
				return !errors.Is(err, context.Canceled) && !errors.Is(err, context.DeadlineExceeded)
			}
			return resp != nil && resp.StatusCode >= 500
		},
	}
}

// Response 是统一响应模型。
type Response struct {
	raw              *http.Response
	maxResponseBytes int64
	cancel           context.CancelFunc // non-nil when Do applied a Timeout; released by Close
}

// Raw 返回底层 *http.Response。
func (r *Response) Raw() *http.Response {
	return r.raw
}

// Header 返回响应头。
func (r *Response) Header() http.Header {
	if r.raw == nil {
		return nil
	}
	return r.raw.Header
}

// StatusCode 返回状态码。
func (r *Response) StatusCode() int {
	if r.raw == nil {
		return 0
	}
	return r.raw.StatusCode
}

// Body 返回底层响应体。
func (r *Response) Body() io.ReadCloser {
	if r.raw == nil {
		return nil
	}
	return r.raw.Body
}

// Close 关闭响应体并释放 timeout context（如果 Do 应用了 Timeout）
func (r *Response) Close() error {
	if r.cancel != nil {
		defer r.cancel()
	}
	if r.raw == nil || r.raw.Body == nil {
		return nil
	}
	return r.raw.Body.Close()
}

// ReadAll 读取全部响应体。
// 优先使用显式传入的 limit；若 limit<=0，则回落到 Request.MaxResponseBytes。
func (r *Response) ReadAll(limit int64) ([]byte, error) {
	if r.raw == nil || r.raw.Body == nil {
		return nil, nil
	}
	if limit <= 0 {
		limit = r.maxResponseBytes
	}
	return readAllLimited(r.raw.Body, limit)
}

// DecodeJSON 读取并反序列化 JSON。
// 若响应体为空（或全空白），直接返回 nil。
func (r *Response) DecodeJSON(v any) error {
	if v == nil {
		return fmt.Errorf("decode target is nil")
	}
	b, err := r.ReadAll(0)
	if err != nil {
		return err
	}
	if len(bytes.TrimSpace(b)) == 0 {
		return nil
	}
	if err := json.Unmarshal(b, v); err != nil {
		return fmt.Errorf("decode json: %w", err)
	}
	return nil
}

// StatusError 是统一的 HTTP 状态错误。
type StatusError struct {
	Method        string
	URL           string
	StatusCode    int
	Header        http.Header
	Body          []byte
	BodyTruncated bool
}

func (e *StatusError) Error() string {
	body := strings.TrimSpace(string(e.Body))
	if body == "" {
		return fmt.Sprintf("http status error: %s %s => %d", e.Method, e.URL, e.StatusCode)
	}
	return fmt.Sprintf("http status error: %s %s => %d, body=%s", e.Method, e.URL, e.StatusCode, body)
}

// IsStatusError 判断 err 是否为 *StatusError。
func IsStatusError(err error) bool {
	var se *StatusError
	return errors.As(err, &se)
}

// AsStatusError 取出 *StatusError。
func AsStatusError(err error) (*StatusError, bool) {
	var se *StatusError
	if errors.As(err, &se) {
		return se, true
	}
	return nil, false
}

// Do 执行请求并返回 Response。
// 成功条件由 ExpectedStatus 决定，默认 2xx。
// 若 req.Timeout > 0，Do 内部会对 ctx 应用 WithTimeout；cancel 存入返回的 Response，
// 由调用方在读完响应体后通过 Response.Close() 释放，避免 body 读取期间 ctx 被提前取消。
func (c *Client) Do(ctx context.Context, req *Request) (resp *Response, err error) {
	if req == nil {
		return nil, fmt.Errorf("request is nil")
	}
	if req.Method == "" {
		return nil, fmt.Errorf("request method is required")
	}
	if strings.TrimSpace(req.URL) == "" {
		return nil, fmt.Errorf("request url is required")
	}
	if req.Body != nil && req.JSON != nil {
		return nil, fmt.Errorf("request body and json cannot both be set")
	}

	var timeoutCancel context.CancelFunc
	if req.Timeout > 0 {
		ctx, timeoutCancel = context.WithTimeout(ctx, req.Timeout)
		// 错误路径：Do 返回 err 时由下方 defer 负责释放 timeout ctx
		// 成功路径：cancel 存入 Response，由调用方 Close() 释放（保证 body 读取期间 ctx 有效）
		defer func() {
			if err != nil && timeoutCancel != nil {
				timeoutCancel()
			}
		}()
	}

	expectedStatus := req.ExpectedStatus
	if expectedStatus == nil {
		expectedStatus = defaultExpectedStatus
	}

	attempt := 0
	for {
		httpReq, err := c.buildHTTPRequest(ctx, req, attempt)
		if err != nil {
			return nil, err
		}

		hc := c.httpClient
		if req.CheckRedirect != nil {
			clone := *hc
			clone.CheckRedirect = req.CheckRedirect
			hc = &clone
		}

		httpResp, doErr := hc.Do(httpReq)
		if doErr == nil && expectedStatus(httpResp.StatusCode) {
			return &Response{
				raw:              httpResp,
				maxResponseBytes: req.MaxResponseBytes,
				cancel:           timeoutCancel,
			}, nil
		}

		// 先判断是否重试
		if shouldRetry(req, httpResp, doErr, attempt) {
			if httpResp != nil && httpResp.Body != nil {
				_ = httpResp.Body.Close()
			}
			attempt++
			if err := waitBackoff(ctx, req.Retry, attempt); err != nil {
				if doErr != nil {
					return nil, doErr
				}
				return nil, err
			}
			continue
		}

		// 不重试，返回最终错误
		if doErr != nil {
			return nil, fmt.Errorf("do request: %w", doErr)
		}

		return nil, newStatusError(httpReq, httpResp)
	}
}

// DoStream 本质上和 Do 一样，语义上用于“调用方想自己读流”。
func (c *Client) DoStream(ctx context.Context, req *Request) (*Response, error) {
	return c.Do(ctx, req)
}

// DoJSON 执行请求并把响应反序列化到 T。
func DoJSON[T any](ctx context.Context, c *Client, req *Request) (*T, error) {
	if c == nil {
		c = Default()
	}
	if req == nil {
		return nil, fmt.Errorf("request is nil")
	}

	reqCopy := *req
	reqCopy.Headers = cloneHeader(req.Headers)
	if reqCopy.Headers == nil {
		reqCopy.Headers = make(http.Header)
	}
	if reqCopy.Headers.Get("Accept") == "" {
		reqCopy.Headers.Set("Accept", "application/json")
	}

	resp, err := c.Do(ctx, &reqCopy)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Close() }()

	var out T
	if err := resp.DecodeJSON(&out); err != nil {
		return nil, err
	}
	return &out, nil
}

// PostJSON 是常用 POST JSON helper。
func PostJSON[T any](ctx context.Context, c *Client, url string, headers http.Header, body any) (*T, error) {
	return DoJSON[T](ctx, c, &Request{
		Method:  http.MethodPost,
		URL:     url,
		Headers: headers,
		JSON:    body,
	})
}

// GetJSON 是常用 GET JSON helper。
// 注意：这里不再接收 body。GET 的参数请走 Query。
func GetJSON[T any](ctx context.Context, c *Client, url string, headers http.Header) (*T, error) {
	return DoJSON[T](ctx, c, &Request{
		Method:  http.MethodGet,
		URL:     url,
		Headers: headers,
	})
}

func (c *Client) buildHTTPRequest(ctx context.Context, req *Request, attempt int) (*http.Request, error) {
	u, err := url.Parse(req.URL)
	if err != nil {
		return nil, fmt.Errorf("parse url: %w", err)
	}

	if len(req.Query) > 0 {
		q := u.Query()
		for k, vv := range req.Query {
			for _, v := range vv {
				q.Add(k, v)
			}
		}
		u.RawQuery = q.Encode()
	}

	bodyReader, defaultHeaders, err := buildBody(req, attempt)
	if err != nil {
		return nil, err
	}

	httpReq, err := http.NewRequestWithContext(ctx, req.Method, u.String(), bodyReader)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	httpReq.Header = cloneHeader(req.Headers)
	if httpReq.Header == nil {
		httpReq.Header = make(http.Header)
	}

	for k, vv := range defaultHeaders {
		if httpReq.Header.Get(k) != "" {
			continue
		}
		for _, v := range vv {
			httpReq.Header.Add(k, v)
		}
	}

	return httpReq, nil
}

func buildBody(req *Request, attempt int) (io.Reader, http.Header, error) {
	if req.JSON != nil {
		b, err := json.Marshal(req.JSON)
		if err != nil {
			return nil, nil, fmt.Errorf("marshal request json: %w", err)
		}
		h := make(http.Header)
		h.Set("Content-Type", "application/json")
		return bytes.NewReader(b), h, nil
	}

	if req.Body == nil {
		return nil, nil, nil
	}

	// 对原始 Body，只保证第一次可读。
	// 如果调用方希望“带 body 的请求也能重试”，建议改用 JSON，
	// 或者扩展 Request 增加 BodyFactory/GetBody。
	if attempt > 0 {
		return nil, nil, fmt.Errorf("request body is not replayable for retry")
	}

	return req.Body, nil, nil
}

func shouldRetry(req *Request, resp *http.Response, err error, attempt int) bool {
	if req == nil || req.Retry == nil {
		return false
	}
	if attempt >= req.Retry.MaxRetries {
		return false
	}
	if req.Retry.ShouldRetry == nil {
		return false
	}
	return req.Retry.ShouldRetry(resp, err)
}

func waitBackoff(ctx context.Context, rp *RetryPolicy, nextAttempt int) error {
	if rp == nil || rp.Backoff == nil {
		return nil
	}
	d := rp.Backoff(nextAttempt)
	if d <= 0 {
		return nil
	}
	timer := time.NewTimer(d)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func defaultExpectedStatus(code int) bool {
	return code >= 200 && code < 300
}

func newStatusError(req *http.Request, resp *http.Response) error {
	if req == nil || resp == nil {
		return fmt.Errorf("unexpected nil request/response")
	}
	defer func() {
		if resp.Body != nil {
			_ = resp.Body.Close()
		}
	}()

	body, truncated, _ := readAllLimitedWithFlag(resp.Body, defaultErrorBodyLimit)
	return &StatusError{
		Method:        req.Method,
		URL:           req.URL.String(),
		StatusCode:    resp.StatusCode,
		Header:        resp.Header.Clone(),
		Body:          body,
		BodyTruncated: truncated,
	}
}

func readAllLimited(r io.Reader, limit int64) ([]byte, error) {
	b, _, err := readAllLimitedWithFlag(r, limit)
	return b, err
}

func readAllLimitedWithFlag(r io.Reader, limit int64) ([]byte, bool, error) {
	if r == nil {
		return nil, false, nil
	}
	if limit <= 0 {
		b, err := io.ReadAll(r)
		return b, false, err
	}

	lr := io.LimitReader(r, limit+1)
	b, err := io.ReadAll(lr)
	if err != nil {
		return nil, false, err
	}
	if int64(len(b)) > limit {
		return b[:limit], true, fmt.Errorf("response body too large: exceeds %d bytes", limit)
	}
	return b, false, nil
}

func cloneHeader(h http.Header) http.Header {
	if h == nil {
		return nil
	}
	return h.Clone()
}
