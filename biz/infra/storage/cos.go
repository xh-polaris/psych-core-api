package storage

import (
	"context"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
	"path/filepath"
	"strings"
	"time"

	"github.com/tencentyun/cos-go-sdk-v5"
	"github.com/xh-polaris/psych-core-api/biz/conf"
)

var _ Provider = (*cosProvider)(nil)

type cosProvider struct {
	client    *cos.Client
	bucketURL string
	cdnURL    string
}

func NewCOS(cfg *conf.Config) Provider {
	c := cfg.COS
	if c == nil {
		panic("COS config is nil")
	}
	b := &cos.BaseURL{
		BucketURL: mustParseURL(c.BucketURL),
	}
	authTransport := &cos.AuthorizationTransport{
		SecretID:  c.SecretID,
		SecretKey: c.SecretKey,
		Transport: http.DefaultTransport,
	}
	client := cos.NewClient(b, &http.Client{Transport: authTransport})
	return &cosProvider{
		client:    client,
		bucketURL: c.BucketURL,
		cdnURL:    c.CDN,
	}
}

func (p *cosProvider) GetAccessURL(_ context.Context, key string) (string, error) {
	if p.cdnURL != "" {
		return p.cdnURL + "/" + key, nil
	}
	return p.client.Object.GetObjectURL(key).String(), nil
}

func (p *cosProvider) Upload(ctx context.Context, key string, r io.Reader, size int64) (string, error) {
	_, err := p.client.Object.Put(ctx, key, r, &cos.ObjectPutOptions{
		ObjectPutHeaderOptions: &cos.ObjectPutHeaderOptions{
			ContentLength: size,
			ContentType:   contentTypeForKey(key),
		},
	})
	if err != nil {
		return "", fmt.Errorf("cos upload failed key=%s: %w", key, err)
	}
	if p.cdnURL != "" {
		return p.cdnURL + "/" + key, nil
	}
	return p.client.Object.GetObjectURL(key).String(), nil
}

func (p *cosProvider) GenPresignUploadURL(ctx context.Context, key string) (string, error) {
	u, err := p.client.Object.GetPresignedURL2(
		ctx,
		http.MethodPut,
		key,
		5*time.Minute,
		&cos.PresignedURLOptions{},
	)
	if err != nil || u == nil {
		return "", fmt.Errorf("gen presign upload url failed key=%s: %w", key, err)
	}
	return u.String(), nil
}

func (p *cosProvider) Delete(ctx context.Context, key string) error {
	if _, err := p.client.Object.Delete(ctx, key); err != nil {
		return fmt.Errorf("cos delete failed key=%s: %w", key, err)
	}
	return nil
}

func contentTypeForKey(key string) string {
	ext := strings.ToLower(filepath.Ext(key))
	ct := mime.TypeByExtension(ext)
	if ct == "" {
		return "application/octet-stream"
	}
	return ct
}

func mustParseURL(raw string) *url.URL {
	u, err := url.Parse(raw)
	if err != nil {
		panic(fmt.Sprintf("invalid bucket url: %s", raw))
	}
	return u
}
