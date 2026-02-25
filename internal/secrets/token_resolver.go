package secrets

import (
	"errors"
	"os"
	"strings"
)

type Resolver interface {
	Resolve(tokenRef string) (string, error)
}

type EnvResolver struct{}

func (r EnvResolver) Resolve(tokenRef string) (string, error) {
	if !strings.HasPrefix(tokenRef, "ENV:") {
		return "", errors.New("unsupported token_ref for EnvResolver")
	}
	key := strings.TrimPrefix(tokenRef, "ENV:")
	v := os.Getenv(key)
	if v == "" {
		return "", errors.New("missing env token: " + key)
	}
	return v, nil
}

type MultiResolver struct {
	Env EnvResolver
	SM  *SMResolver
}

// Regras:
// - "ENV:XXX" => env
// - "SM:xxx"  => secret manager
// - sem prefixo => assume secret manager (compat com o que você salvou no JSON)
func (r MultiResolver) Resolve(tokenRef string) (string, error) {
	if strings.HasPrefix(tokenRef, "ENV:") {
		return r.Env.Resolve(tokenRef)
	}
	if r.SM == nil {
		return "", errors.New("SM resolver not configured")
	}
	return r.SM.Resolve(tokenRef)
}