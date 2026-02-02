package secrets

import (
	"errors"
	"os"
	"strings"
)

type Resolver interface{ Resolve(tokenRef string) (string, error) }

type EnvResolver struct{}

func (r EnvResolver) Resolve(tokenRef string) (string, error) {
	if !strings.HasPrefix(tokenRef, "ENV:") {
		return "", errors.New("unsupported token_ref (MVP supports ENV: only)")
	}
	key := strings.TrimPrefix(tokenRef, "ENV:")
	v := os.Getenv(key)
	if v == "" {
		return "", errors.New("missing env token: " + key)
	}
	return v, nil
}
