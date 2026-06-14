package middleware

import (
	"net/http"
)

// CORS sets cross-origin response headers. Pass ["*"] to allow any origin.
// Non-matching origins receive 403 Forbidden. Preflight OPTIONS requests
// are handled and short-circuited with 204 No Content.
func CORS(allowedOrigins []string) func(http.Handler) http.Handler {
	allowAll := len(allowedOrigins) == 1 && allowedOrigins[0] == "*"
	allowed := make(map[string]bool, len(allowedOrigins))
	for _, o := range allowedOrigins {
		allowed[o] = true
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")
			if origin == "" {
				next.ServeHTTP(w, r)
				return
			}

			if !allowAll && !allowed[origin] {
				http.Error(w, "CORS origin not allowed", http.StatusForbidden)
				return
			}

			if allowAll {
				w.Header().Set("Access-Control-Allow-Origin", "*")
			} else {
				w.Header().Set("Access-Control-Allow-Origin", origin)
				w.Header().Add("Vary", "Origin")
			}
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
			w.Header().Set("Access-Control-Max-Age", "86400")

			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusNoContent)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
