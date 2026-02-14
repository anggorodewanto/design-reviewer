# Phase 33: OAuth State Cookie Missing Secure Flag

**Severity:** 🟢 Low

## Problem

The `oauth_state` cookie does not set `Secure: true` when the server runs over HTTPS.

## Fix

Set `Secure` conditionally based on the base URL:

```go
http.SetCookie(w, &http.Cookie{
    Name:     "oauth_state",
    Value:    state,
    Path:     "/",
    HttpOnly: true,
    Secure:   strings.HasPrefix(h.Auth.BaseURL, "https://"),
    SameSite: http.SameSiteLaxMode,
})
```

Apply the same fix in `handleCLILogin`.

## Files to Change

- `internal/api/auth_handlers.go`

## Acceptance Criteria

- `oauth_state` cookie has `Secure` flag when base URL is HTTPS
- Cookie omits `Secure` flag for local HTTP development
- Both web and CLI login flows set the flag correctly
