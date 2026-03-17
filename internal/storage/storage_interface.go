package storage

import (
	"context"
	"io"
)

type StorageClient interface {
	Upload(ctx context.Context, key string, data io.Reader, contentType string) (string, error)
	GetURL(key string) string
	Download(ctx context.Context, key string) ([]byte, error)
}
