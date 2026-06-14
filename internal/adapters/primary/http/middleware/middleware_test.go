package middleware_test

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/MoNezhadali/foodscheduler/internal/adapters/primary/http/middleware"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ── helpers ───────────────────────────────────────────────────────────────────

func okHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
}

func do(t *testing.T, h http.Handler, method, path string, headers map[string]string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, nil)
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	return rr
}

// ── SecureHeaders ─────────────────────────────────────────────────────────────

func TestSecureHeaders_AllHeadersPresent(t *testing.T) {
	h := middleware.SecureHeaders()(okHandler())
	rr := do(t, h, http.MethodGet, "/", nil)

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Equal(t, "nosniff", rr.Header().Get("X-Content-Type-Options"))
	assert.Equal(t, "DENY", rr.Header().Get("X-Frame-Options"))
	assert.Equal(t, "1; mode=block", rr.Header().Get("X-XSS-Protection"))
	assert.Equal(t, "strict-origin-when-cross-origin", rr.Header().Get("Referrer-Policy"))
	assert.Equal(t, "default-src 'none'", rr.Header().Get("Content-Security-Policy"))
}

// ── CORS ──────────────────────────────────────────────────────────────────────

func TestCORS_AllowAll_SetsWildcard(t *testing.T) {
	h := middleware.CORS([]string{"*"})(okHandler())
	rr := do(t, h, http.MethodGet, "/", map[string]string{"Origin": "https://example.com"})

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Equal(t, "*", rr.Header().Get("Access-Control-Allow-Origin"))
}

func TestCORS_AllowedOrigin_ReflectsOrigin(t *testing.T) {
	h := middleware.CORS([]string{"https://app.example.com"})(okHandler())
	rr := do(t, h, http.MethodGet, "/", map[string]string{"Origin": "https://app.example.com"})

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Equal(t, "https://app.example.com", rr.Header().Get("Access-Control-Allow-Origin"))
	assert.Equal(t, "Origin", rr.Header().Get("Vary"))
}

func TestCORS_BlockedOrigin_Returns403(t *testing.T) {
	h := middleware.CORS([]string{"https://app.example.com"})(okHandler())
	rr := do(t, h, http.MethodGet, "/", map[string]string{"Origin": "https://evil.com"})

	assert.Equal(t, http.StatusForbidden, rr.Code)
}

func TestCORS_Preflight_Returns204(t *testing.T) {
	h := middleware.CORS([]string{"*"})(okHandler())
	rr := do(t, h, http.MethodOptions, "/", map[string]string{"Origin": "https://example.com"})

	assert.Equal(t, http.StatusNoContent, rr.Code)
	assert.NotEmpty(t, rr.Header().Get("Access-Control-Allow-Methods"))
	assert.NotEmpty(t, rr.Header().Get("Access-Control-Allow-Headers"))
}

func TestCORS_NoOriginHeader_PassesThrough(t *testing.T) {
	h := middleware.CORS([]string{"https://app.example.com"})(okHandler())
	rr := do(t, h, http.MethodGet, "/", nil)

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Empty(t, rr.Header().Get("Access-Control-Allow-Origin"))
}

// ── BodyLimit ─────────────────────────────────────────────────────────────────

func TestBodyLimit_SmallBody_PassesThrough(t *testing.T) {
	h := middleware.BodyLimit(100)(okHandler())
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"x":1}`))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
}

func TestBodyLimit_ExactLimit_PassesThrough(t *testing.T) {
	body := bytes.Repeat([]byte("a"), 100)
	h := middleware.BodyLimit(100)(okHandler())
	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(body))
	req.ContentLength = 100
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
}

func TestBodyLimit_OversizeContentLength_Returns413(t *testing.T) {
	body := bytes.Repeat([]byte("a"), 200)
	h := middleware.BodyLimit(100)(okHandler())
	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(body))
	req.ContentLength = 200
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusRequestEntityTooLarge, rr.Code)
	require.Contains(t, rr.Body.String(), "PAYLOAD_TOO_LARGE")
}

// ── RateLimit ─────────────────────────────────────────────────────────────────

func TestRateLimit_UnderLimit_AllowsRequests(t *testing.T) {
	h := middleware.RateLimit(5, time.Minute)(okHandler())
	for i := 0; i < 5; i++ {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.RemoteAddr = "127.0.0.1:1234"
		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, req)
		assert.Equal(t, http.StatusOK, rr.Code, "request %d should pass", i+1)
	}
}

func TestRateLimit_ExceedsLimit_Returns429(t *testing.T) {
	h := middleware.RateLimit(3, time.Minute)(okHandler())
	for i := 0; i < 3; i++ {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.RemoteAddr = "10.0.0.1:9999"
		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, req)
		assert.Equal(t, http.StatusOK, rr.Code)
	}
	// 4th request must be rejected
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "10.0.0.1:9999"
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusTooManyRequests, rr.Code)
	assert.Contains(t, rr.Body.String(), "RATE_LIMITED")
	assert.NotEmpty(t, rr.Header().Get("Retry-After"))
}

func TestRateLimit_DifferentIPs_IndependentLimits(t *testing.T) {
	h := middleware.RateLimit(2, time.Minute)(okHandler())

	send := func(ip string) int {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.RemoteAddr = ip + ":1000"
		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, req)
		return rr.Code
	}

	// Exhaust IP A's limit
	assert.Equal(t, http.StatusOK, send("192.168.1.1"))
	assert.Equal(t, http.StatusOK, send("192.168.1.1"))
	assert.Equal(t, http.StatusTooManyRequests, send("192.168.1.1"))

	// IP B should still be allowed
	assert.Equal(t, http.StatusOK, send("192.168.1.2"))
}

func TestRateLimit_XForwardedFor_UsedAsIP(t *testing.T) {
	h := middleware.RateLimit(1, time.Minute)(okHandler())

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "10.0.0.1:1234"
	req.Header.Set("X-Forwarded-For", "203.0.113.5, 10.0.0.1")
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusOK, rr.Code)

	// Second request from same XFF origin should be limited
	req2 := httptest.NewRequest(http.MethodGet, "/", nil)
	req2.RemoteAddr = "10.0.0.1:1234"
	req2.Header.Set("X-Forwarded-For", "203.0.113.5, 10.0.0.1")
	rr2 := httptest.NewRecorder()
	h.ServeHTTP(rr2, req2)
	assert.Equal(t, http.StatusTooManyRequests, rr2.Code)
}
