# Phase 23: Sessions Never Expire

**Severity:** 🟠 High

## Problem

The signed session payload is `{"name":"…","email":"…"}` with no timestamp. A captured cookie value is valid forever as long as the session secret doesn't change.

## Fix

Add an `exp` field to `User` and validate it:

```go
type User struct {
    Name      string `json:"name"`
    Email     string `json:"email"`
    ExpiresAt int64  `json:"exp,omitempty"`
}

// In SignSession, before marshalling:
u.ExpiresAt = time.Now().Add(24 * time.Hour).Unix()

// In VerifySession, after unmarshalling:
if u.ExpiresAt > 0 && time.Now().Unix() > u.ExpiresAt {
    return User{}, errors.New("session expired")
}
```

## Files to Change

- `internal/auth/auth.go` — `User` struct, `SignSession`, `VerifySession`

## Acceptance Criteria

- New sessions include an expiration timestamp
- Sessions older than 24 hours are rejected by `VerifySession`
- Existing sessions without `exp` are treated as expired (or given a grace period)
