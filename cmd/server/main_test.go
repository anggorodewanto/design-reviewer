package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSecurityHeaders(t *testing.T) {
	handler := securityHeaders(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	expected := map[string]string{
		"X-Content-Type-Options": "nosniff",
		"X-Frame-Options":       "DENY",
		"Referrer-Policy":       "strict-origin-when-cross-origin",
		"Permissions-Policy":    "camera=(), microphone=(), geolocation=()",
	}

	for _, method := range []string{"GET", "POST", "PUT", "DELETE"} {
		t.Run(method, func(t *testing.T) {
			rr := httptest.NewRecorder()
			req := httptest.NewRequest(method, "/", nil)
			handler.ServeHTTP(rr, req)

			for header, want := range expected {
				if got := rr.Header().Get(header); got != want {
					t.Errorf("%s: got %q, want %q", header, got, want)
				}
			}
		})
	}
}

func TestSecurityHeadersPreserveInnerHandler(t *testing.T) {
	handler := securityHeaders(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Custom", "test")
		w.WriteHeader(http.StatusTeapot)
		w.Write([]byte("hello"))
	}))

	rr := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/any-path", nil)
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusTeapot {
		t.Errorf("status: got %d, want %d", rr.Code, http.StatusTeapot)
	}
	if got := rr.Header().Get("X-Custom"); got != "test" {
		t.Errorf("X-Custom: got %q, want %q", got, "test")
	}
	if rr.Body.String() != "hello" {
		t.Errorf("body: got %q, want %q", rr.Body.String(), "hello")
	}
}
