package middleware

import (
	"encoding/json"
	"net/http"
)

func RequireRole(role string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			claims, ok := ClaimsFromContext(r.Context())
			if !ok {
				writeUnauthorized(w, "MISSING_TOKEN", "Authorization required")
				return
			}
			if claims.Role != role {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusForbidden)
				json.NewEncoder(w).Encode(map[string]string{ //nolint:errcheck
					"code":    "FORBIDDEN",
					"message": "insufficient permissions",
				})
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
