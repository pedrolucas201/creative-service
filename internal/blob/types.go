package blob

import "context"

type Store interface {
	Save(ctx context.Context, key string, data []byte) (string, error)
	Load(ctx context.Context, path string) ([]byte, error)
}
