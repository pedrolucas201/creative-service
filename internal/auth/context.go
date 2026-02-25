package auth

import "context"

type contextKey string

const identityKey contextKey = "auth.identity"

func WithIdentity(ctx context.Context, identity *Identity) context.Context {
	return context.WithValue(ctx, identityKey, identity)
}

func IdentityFromContext(ctx context.Context) (*Identity, bool) {
	v := ctx.Value(identityKey)
	identity, ok := v.(*Identity)
	return identity, ok
}
