package blob

import (
	"context"
	"os"
	"path/filepath"
)

type LocalFS struct{ Dir string }

func NewLocalFS(dir string) *LocalFS { return &LocalFS{Dir: dir} }

func (s *LocalFS) Save(ctx context.Context, key string, data []byte) (string, error) {
	p := filepath.Join(s.Dir, key)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return "", err
	}
	if err := os.WriteFile(p, data, 0o644); err != nil {
		return "", err
	}
	return p, nil
}

func (s *LocalFS) Load(ctx context.Context, path string) ([]byte, error) {
	return os.ReadFile(path)
}
