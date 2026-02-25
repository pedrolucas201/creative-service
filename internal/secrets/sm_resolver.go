package secrets

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	secretmanager "cloud.google.com/go/secretmanager/apiv1"
	smpb "cloud.google.com/go/secretmanager/apiv1/secretmanagerpb"
)

type cacheItem struct {
	val string
	exp time.Time
}

type SMResolver struct {
	projectID string
	client    *secretmanager.Client

	mu    sync.Mutex
	cache map[string]cacheItem
	ttl   time.Duration
}

func NewSMResolver(ctx context.Context, projectID string, ttl time.Duration) (*SMResolver, error) {
	if projectID == "" {
		return nil, errors.New("missing projectID for SMResolver")
	}
	c, err := secretmanager.NewClient(ctx)
	if err != nil {
		return nil, err
	}
	if ttl <= 0 {
		ttl = 2 * time.Minute
	}
	return &SMResolver{
		projectID: projectID,
		client:    c,
		cache:     make(map[string]cacheItem),
		ttl:       ttl,
	}, nil
}

func (r *SMResolver) Close() error {
	return r.client.Close()
}

// Resolve suporta:
// - "SM:<secret_name>"  (projeto = r.projectID)
// - "<secret_name>"     (fallback: assume SM)
// - "SM:<project>/<secret_name>" (opcional, se você quiser guardar projeto no token_ref)
func (r *SMResolver) Resolve(tokenRef string) (string, error) {
	// normalize
	ref := strings.TrimSpace(tokenRef)
	if ref == "" {
		return "", errors.New("empty token_ref")
	}

	project := r.projectID
	secretName := ref

	if strings.HasPrefix(ref, "SM:") {
		secretName = strings.TrimPrefix(ref, "SM:")
	}

	// opcional: permitir "project/secret"
	if parts := strings.Split(secretName, "/"); len(parts) == 2 {
		project = parts[0]
		secretName = parts[1]
	}

	secretName = strings.TrimSpace(secretName)
	if secretName == "" {
		return "", errors.New("empty secret name in token_ref")
	}

	// cache curto pra reduzir latência/custo
	cacheKey := project + "/" + secretName
	now := time.Now()

	r.mu.Lock()
	if it, ok := r.cache[cacheKey]; ok && now.Before(it.exp) {
		v := it.val
		r.mu.Unlock()
		return v, nil
	}
	r.mu.Unlock()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	fullName := fmt.Sprintf("projects/%s/secrets/%s/versions/latest", project, secretName)
	res, err := r.client.AccessSecretVersion(ctx, &smpb.AccessSecretVersionRequest{
		Name: fullName,
	})
	if err != nil {
		return "", err
	}

	val := string(res.Payload.Data)
	val = strings.TrimPrefix(val, "\uFEFF")
	val = strings.TrimSpace(val)
	if val == "" {
		return "", errors.New("empty secret payload for " + cacheKey)
	}

	r.mu.Lock()
	r.cache[cacheKey] = cacheItem{val: val, exp: now.Add(r.ttl)}
	r.mu.Unlock()

	return val, nil
}
