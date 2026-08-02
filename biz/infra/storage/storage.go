package storage

import (
	"context"
	"io"
)

type Provider interface {
	GetAccessURL(ctx context.Context, key string) (accessURL string, err error)
	GenPresignUploadURL(ctx context.Context, key string) (presignedURL string, err error)
	Upload(ctx context.Context, key string, r io.Reader, size int64) (accessURL string, err error)
	Delete(ctx context.Context, key string) error
}
