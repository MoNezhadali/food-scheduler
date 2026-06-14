package middleware

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"
)

// RateLimit returns a middleware that limits requests to max per window per
// client IP. Clients that exceed the limit receive 429 Too Many Requests.
func RateLimit(max int, window time.Duration) func(http.Handler) http.Handler {
	rl := &rateLimiter{
		clients: make(map[string]*windowState),
		max:     max,
		window:  window,
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ip := clientIP(r)
			if !rl.allow(ip) {
				w.Header().Set("Content-Type", "application/json")
				w.Header().Set("Retry-After", fmt.Sprintf("%.0f", window.Seconds()))
				w.WriteHeader(http.StatusTooManyRequests)
				json.NewEncoder(w).Encode(map[string]string{ //nolint:errcheck
					"code":    "RATE_LIMITED",
					"message": "too many requests, please slow down",
				})
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// ── internals ─────────────────────────────────────────────────────────────────

type windowState struct {
	count     int
	windowEnd time.Time
}

type rateLimiter struct {
	mu      sync.Mutex
	clients map[string]*windowState
	max     int
	window  time.Duration
	tick    int // lazy-cleanup counter
}

func (rl *rateLimiter) allow(ip string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()

	// Lazy cleanup every 256 requests to prevent unbounded map growth.
	rl.tick++
	if rl.tick >= 256 {
		rl.tick = 0
		for k, s := range rl.clients {
			if now.After(s.windowEnd) {
				delete(rl.clients, k)
			}
		}
	}

	s, ok := rl.clients[ip]
	if !ok || now.After(s.windowEnd) {
		rl.clients[ip] = &windowState{count: 1, windowEnd: now.Add(rl.window)}
		return true
	}
	if s.count >= rl.max {
		return false
	}
	s.count++
	return true
}

// clientIP extracts the real client IP, honouring X-Forwarded-For when present.
func clientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		// First entry is the originating client.
		if idx := strings.IndexByte(xff, ','); idx >= 0 {
			return strings.TrimSpace(xff[:idx])
		}
		return strings.TrimSpace(xff)
	}
	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		return strings.TrimSpace(xri)
	}
	// Strip port from RemoteAddr.
	if idx := strings.LastIndex(r.RemoteAddr, ":"); idx >= 0 {
		return r.RemoteAddr[:idx]
	}
	return r.RemoteAddr
}
