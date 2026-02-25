package storage

import (
	"context"
	"io"
)

// StorageClient é a interface comum pra S3 e GCS
// Permite trocar entre eles sem mudar código
type StorageClient interface {
	Upload(ctx context.Context, key string, data io.Reader, contentType string) (string, error)
	GetURL(key string) string
	Download(ctx context.Context, key string) ([]byte, error)
}
