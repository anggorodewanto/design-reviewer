package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestClientIP(t *testing.T) {
	tests := []struct {
		name       string
		remoteAddr string
		forwarded  string
		want       string
	}{
		{"remote addr with port", "1.2.3.4:5678", "", "1.2.3.4"},
		{"remote addr no port", "1.2.3.4", "", "1.2.3.4"},
		{"x-forwarded-for single", "9.9.9.9:1234", "10.0.0.1", "10.0.0.1"},
		{"x-forwarded-for multiple", "9.9.9.9:1234", "10.0.0.1, 10.0.0.2", "10.0.0.1"},
		{"x-forwarded-for spaces", "9.9.9.9:1234", " 10.0.0.1 , 10.0.0.2", "10.0.0.1"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := httptest.NewRequest("GET", "/", nil)
			r.RemoteAddr = tt.remoteAddr
			if tt.forwarded != "" {
				r.Header.Set("X-Forwarded-For", tt.forwarded)
			}
			if got := clientIP(r); got != tt.want {
				t.Errorf("clientIP() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestIsStrictPath(t *testing.T) {
	strict := []string{
		"/auth/google/login",
		"/auth/google/callback",
		"/auth/logout",
		"/api/auth/token",
		"/invite/abc123",
		"/api/projects/123/invites",
	}
	general := []string{
		"/",
		"/api/projects",
		"/api/upload",
		"/static/style.css",
		"/projects/123",
		"/api/versions/123/comments",
	}
	for _, p := range strict {
		if !isStrictPath(p) {
			t.Errorf("isStrictPath(%q) = false, want true", p)
		}
	}
	for _, p := range general {
		if isStrictPath(p) {
			t.Errorf("isStrictPath(%q) = true, want false", p)
		}
	}
}

func TestRateLimiterMiddleware_AllowsUnderLimit(t *testing.T) {
	rl := NewRateLimiter()
	handler := rl.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// First request should pass (within burst)
	req := httptest.NewRequest("GET", "/api/projects", nil)
	req.RemoteAddr = "1.2.3.4:1234"
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("first request: got %d, want 200", w.Code)
	}
}

func TestRateLimiterMiddleware_BlocksOverLimit(t *testing.T) {
	rl := NewRateLimiter()
	handler := rl.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// Exhaust the general burst (30)
	for i := 0; i < 30; i++ {
		req := httptest.NewRequest("GET", "/api/projects", nil)
		req.RemoteAddr = "5.5.5.5:1234"
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
	}

	// Next request should be rate limited
	req := httptest.NewRequest("GET", "/api/projects", nil)
	req.RemoteAddr = "5.5.5.5:1234"
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	if w.Code != http.StatusTooManyRequests {
		t.Errorf("over-limit request: got %d, want 429", w.Code)
	}
	var body map[string]string
	json.NewDecoder(w.Body).Decode(&body)
	if body["error"] != "rate limit exceeded" {
		t.Errorf("error = %q, want %q", body["error"], "rate limit exceeded")
	}
	if w.Header().Get("Retry-After") == "" {
		t.Error("missing Retry-After header")
	}
}

func TestRateLimiterMiddleware_StrictLowerBurst(t *testing.T) {
	rl := NewRateLimiter()
	handler := rl.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// Exhaust strict burst (5)
	for i := 0; i < 5; i++ {
		req := httptest.NewRequest("GET", "/auth/google/login", nil)
		req.RemoteAddr = "6.6.6.6:1234"
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
	}

	// 6th request to auth should be blocked
	req := httptest.NewRequest("GET", "/auth/google/login", nil)
	req.RemoteAddr = "6.6.6.6:1234"
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	if w.Code != http.StatusTooManyRequests {
		t.Errorf("strict over-limit: got %d, want 429", w.Code)
	}
}

func TestRateLimiterMiddleware_PerIPIsolation(t *testing.T) {
	rl := NewRateLimiter()
	handler := rl.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// Exhaust burst for IP A
	for i := 0; i < 10; i++ {
		req := httptest.NewRequest("GET", "/api/projects", nil)
		req.RemoteAddr = "7.7.7.7:1234"
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
	}

	// IP B should still work
	req := httptest.NewRequest("GET", "/api/projects", nil)
	req.RemoteAddr = "8.8.8.8:1234"
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("different IP: got %d, want 200", w.Code)
	}
}

func TestRateLimiterMiddleware_SeparateStoresForStrictAndGeneral(t *testing.T) {
	rl := NewRateLimiter()
	handler := rl.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// Exhaust strict burst for an IP
	for i := 0; i < 5; i++ {
		req := httptest.NewRequest("POST", "/api/auth/token", nil)
		req.RemoteAddr = "9.9.9.9:1234"
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
	}

	// Same IP should still be able to hit general endpoints
	req := httptest.NewRequest("GET", "/api/projects", nil)
	req.RemoteAddr = "9.9.9.9:1234"
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("general after strict exhausted: got %d, want 200", w.Code)
	}
}
