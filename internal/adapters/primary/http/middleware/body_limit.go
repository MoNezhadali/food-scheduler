package middleware

import (
	"encoding/json"
	"net/http"
)

// BodyLimit rejects request bodies larger than maxBytes with 413.
// Use this to protect against memory-exhaustion via large payloads.
func BodyLimit(maxBytes int64) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.ContentLength > maxBytes {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusRequestEntityTooLarge)
				json.NewEncoder(w).Encode(map[string]string{ //nolint:errcheck
					"code":    "PAYLOAD_TOO_LARGE",
					"message": "request body exceeds the allowed limit",
				})
				return
			}
			// Cap the stream so even chunked bodies can't exceed the limit.
			r.Body = http.MaxBytesReader(w, r.Body, maxBytes)
			next.ServeHTTP(w, r)
		})
	}
}
