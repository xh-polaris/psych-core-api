package psychpost

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"testing"

	"github.com/xh-polaris/psych-core-api/biz/conf"
)

type fakeDoer struct{ request *http.Request }

func (d *fakeDoer) Do(request *http.Request) (*http.Response, error) {
	d.request = request
	return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(bytes.NewReader([]byte(`{}`)))}, nil
}

func newTestClient(doer httpDoer) *client {
	return &client{config: &conf.Config{OpenApi: &conf.OpenApi{
		ReportInternalURL: "http://psych-post/internal/v1/reports/generate", ReportInternalToken: "internal-token",
	}}, doer: doer}
}

func TestGenerateForwardsUpstream(t *testing.T) {
	doer := &fakeDoer{}
	payload := []byte(`{"request_id":"req_1"}`)
	response, err := newTestClient(doer).Generate(context.Background(), "ds-tenant-a", payload)
	if err != nil || string(response) != "{}" {
		t.Fatalf("Generate() response=%q err=%v", response, err)
	}
	req := doer.request
	if got := req.Header.Get(upstreamHeader); got != "ds-tenant-a" {
		t.Errorf("%s = %q, want %q", upstreamHeader, got, "ds-tenant-a")
	}
	if got := req.Header.Get("Authorization"); got != "Bearer internal-token" {
		t.Errorf("authorization = %q", got)
	}
	if got := req.Header.Get("Content-Type"); got != "application/json" {
		t.Errorf("content type = %q", got)
	}
	if got := req.URL.String(); got != "http://psych-post/internal/v1/reports/generate" {
		t.Errorf("url = %q", got)
	}
	body, _ := io.ReadAll(req.Body)
	if !bytes.Equal(body, payload) {
		t.Errorf("body = %s, want %s", body, payload)
	}
}

func TestGenerateOmitsUpstreamHeaderWhenEmpty(t *testing.T) {
	doer := &fakeDoer{}
	if _, err := newTestClient(doer).Generate(context.Background(), "", nil); err != nil {
		t.Fatalf("Generate() error = %v", err)
	}
	if got := doer.request.Header.Values(upstreamHeader); len(got) != 0 {
		t.Errorf("unexpected %s header: %v", upstreamHeader, got)
	}
}
