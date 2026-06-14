package middleware

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/MoNezhadali/foodscheduler/internal/infrastructure/auth"
)

type contextKey string

const claimsKey contextKey = "auth_claims"

func Auth(tokenSvc auth.Service) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			header := r.Header.Get("Authorization")
			if !strings.HasPrefix(header, "Bearer ") {
				writeUnauthorized(w, "MISSING_TOKEN", "Authorization header required")
				return
			}
			tokenStr := strings.TrimPrefix(header, "Bearer ")
			claims, err := tokenSvc.Validate(tokenStr, auth.TokenTypeAccess)
			if err != nil {
				writeUnauthorized(w, "INVALID_TOKEN", "Invalid or expired token")
				return
			}
			ctx := context.WithValue(r.Context(), claimsKey, claims)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func ClaimsFromContext(ctx context.Context) (*auth.Claims, bool) {
	c, ok := ctx.Value(claimsKey).(*auth.Claims)
	return c, ok
}

func writeUnauthorized(w http.ResponseWriter, code, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusUnauthorized)
	json.NewEncoder(w).Encode(map[string]string{"code": code, "message": message}) //nolint:errcheck
}
