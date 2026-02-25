package httpapi

import (
	"log"
	"net/http"
	"net/url"
	"strings"
)

func Recoverer(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				log.Println("panic:", rec)
				writeErr(w, http.StatusInternalServerError, "internal_error")
			}
		}()
		next.ServeHTTP(w, r)
	})
}

func AccessLog(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		next.ServeHTTP(w, r)
	})
}

func CORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := strings.TrimSpace(r.Header.Get("Origin"))
		if origin == "" {
			next.ServeHTTP(w, r)
			return
		}

		if !isAllowedOrigin(origin) {
			if r.Method == http.MethodOptions {
				writeErr(w, http.StatusForbidden, "cors_origin_not_allowed")
				return
			}
			next.ServeHTTP(w, r)
			return
		}

		w.Header().Set("Access-Control-Allow-Origin", origin)
		w.Header().Set("Vary", "Origin")
		w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PATCH, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Max-Age", "600")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func isAllowedOrigin(origin string) bool {
	u, err := url.Parse(origin)
	if err != nil {
		return false
	}

	host := strings.ToLower(u.Hostname())
	if host == "" {
		return false
	}

	if host == "localhost" || host == "127.0.0.1" {
		return true
	}

	if strings.HasSuffix(host, ".web.app") || strings.HasSuffix(host, ".firebaseapp.com") {
		return true
	}

	return false
}
