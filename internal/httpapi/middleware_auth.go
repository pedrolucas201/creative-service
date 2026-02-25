package httpapi

import (
	"net/http"
	"strings"

	"creative-service/internal/auth"
	"creative-service/internal/storage"
)

func AuthMiddleware(verifier auth.Verifier) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authz := strings.TrimSpace(r.Header.Get("Authorization"))
			if authz == "" {
				writeErr(w, http.StatusUnauthorized, "missing_authorization_header")
				return
			}

			parts := strings.SplitN(authz, " ", 2)
			if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
				writeErr(w, http.StatusUnauthorized, "invalid_authorization_header")
				return
			}

			token := strings.TrimSpace(parts[1])
			if token == "" {
				writeErr(w, http.StatusUnauthorized, "missing_bearer_token")
				return
			}

			identity, err := verifier.VerifyIDToken(r.Context(), token)
			if err != nil {
				writeErr(w, http.StatusUnauthorized, "invalid_or_expired_token")
				return
			}

			ctx := auth.WithIdentity(r.Context(), identity)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func EnsureAppUserMiddleware(store *storage.Store) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			identity, ok := auth.IdentityFromContext(r.Context())
			if !ok || identity == nil || identity.UID == "" {
				writeErr(w, http.StatusUnauthorized, "missing_identity")
				return
			}

			if err := store.EnsureAppUser(r.Context(), identity.UID, identity.Email); err != nil {
				writeErr(w, http.StatusInternalServerError, "failed_to_sync_user")
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
