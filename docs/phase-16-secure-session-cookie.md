# Phase 16: Secure Session Cookie

## Problem

`SetSessionCookie` in `internal/auth/auth.go` does not set the `Secure` flag on the session cookie:

```go
http.SetCookie(w, &http.Cookie{
    Name:     "session",
    Value:    val,
    Path:     "/",
    HttpOnly: true,
    SameSite: http.SameSiteLaxMode,
})
```

Without `Secure: true`, the cookie can be transmitted over plain HTTP, exposing it to interception.

## Fix

Add `Secure: true` to the session cookie. This should be conditional on the environment — enabled when `BASE_URL` starts with `https://`, disabled for local development over `http://localhost`.

Pass a `secure` boolean to `SetSessionCookie` derived from the server's base URL configuration.

## Files to Change

- `internal/auth/auth.go` — add `secure` parameter to `SetSessionCookie`
- `internal/api/auth_handlers.go` — pass the secure flag when setting cookies

## Acceptance Criteria

- Session cookie has `Secure` flag when served over HTTPS
- Local development over HTTP still works
