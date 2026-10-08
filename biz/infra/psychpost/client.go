// psychpost 适配对 psych-post 私有报告接口的调用。
package psychpost

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/xh-polaris/psych-core-api/biz/conf"
)

const (
	upstreamHeader = "X-Psych-Upstream"
	maxResponseLen = 4 << 20
)

var ErrTimeout = errors.New("psych-post report timeout")

// Client 是面向应用层的私有报告服务调用边界。
type Client interface {
	Generate(context.Context, string, []byte) ([]byte, error)
}

type httpDoer interface {
	Do(*http.Request) (*http.Response, error)
}

type client struct {
	config *conf.Config
	doer   httpDoer
}

var _ Client = (*client)(nil)

func NewClient(config *conf.Config) Client {
	return &client{config: config}
}

func (c *client) Generate(ctx context.Context, upstream string, body []byte) ([]byte, error) {
	if c == nil || c.config == nil || c.config.OpenApi == nil || c.config.OpenApi.ReportInternalURL == "" || c.config.OpenApi.ReportInternalToken == "" {
		return nil, errors.New("psych-post report configuration is unavailable")
	}
	oa := c.config.OpenApi
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, oa.ReportInternalURL, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create psych-post report request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+oa.ReportInternalToken)
	req.Header.Set("Content-Type", "application/json")
	if upstream != "" {
		req.Header.Set(upstreamHeader, upstream)
	}
	doer := c.doer
	if doer == nil {
		timeout := 90 * time.Second
		if oa.ReportTimeoutSeconds > 0 {
			timeout = time.Duration(oa.ReportTimeoutSeconds) * time.Second
		}
		doer = &http.Client{Timeout: timeout}
	}
	resp, err := doer.Do(req)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			return nil, ErrTimeout
		}
		return nil, fmt.Errorf("call psych-post report: %w", err)
	}
	defer resp.Body.Close()
	payload, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseLen))
	if err != nil {
		return nil, fmt.Errorf("read psych-post report response: %w", err)
	}
	if resp.StatusCode == http.StatusGatewayTimeout {
		return nil, ErrTimeout
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("psych-post report status %d", resp.StatusCode)
	}
	return payload, nil
}
