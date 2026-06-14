package middleware

import "net/http"

// SecureHeaders sets defensive HTTP response headers on every request.
func SecureHeaders() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			h := w.Header()
			h.Set("X-Content-Type-Options", "nosniff")
			h.Set("X-Frame-Options", "DENY")
			h.Set("X-XSS-Protection", "1; mode=block")
			h.Set("Referrer-Policy", "strict-origin-when-cross-origin")
			// Pure API — no browser content is served, so a restrictive CSP is safe.
			h.Set("Content-Security-Policy", "default-src 'none'")
			next.ServeHTTP(w, r)
		})
	}
}
