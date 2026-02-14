# Phase 27: No Security Headers

**Severity:** 🟡 Medium

## Problem

The server sets no security headers. Missing: `X-Content-Type-Options`, `X-Frame-Options`, `Referrer-Policy`, `Permissions-Policy`.

## Fix

Wrap the mux in a security headers middleware in `main.go`:

```go
func securityHeaders(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        w.Header().Set("X-Content-Type-Options", "nosniff")
        w.Header().Set("X-Frame-Options", "DENY")
        w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
        w.Header().Set("Permissions-Policy", "camera=(), microphone=(), geolocation=()")
        next.ServeHTTP(w, r)
    })
}

// In main():
log.Fatal(http.ListenAndServe(addr, securityHeaders(mux)))
```

## Files to Change

- `cmd/server/main.go`

## Acceptance Criteria

- All responses include `X-Content-Type-Options: nosniff`
- All responses include `X-Frame-Options: DENY`
- All responses include `Referrer-Policy: strict-origin-when-cross-origin`
- All responses include `Permissions-Policy: camera=(), microphone=(), geolocation=()`
