# Phase 24: Logout Doesn't Invalidate Sessions Server-Side

**Severity:** 🟠 High

## Problem

`ClearSessionCookie` only removes the cookie from the browser. The signed value remains valid if previously captured (e.g., via network sniffing over HTTP). Combined with the lack of session expiration (phase 23), a stolen session is usable indefinitely.

## Fix

Adding session expiration (phase 23) mitigates this. For full revocation, either:

1. Switch to server-side session storage (store session IDs in the DB, delete on logout), or
2. Maintain a revocation list checked in `VerifySession`

The minimal approach is to ensure phase 23 is implemented first, which bounds the window of exposure. A server-side session store is the stronger fix if needed.

## Files to Change

- `internal/api/auth_handlers.go` — `handleLogout`
- `internal/auth/auth.go` — if adding server-side session tracking
- `internal/db/db.go` — if adding a sessions table

## Acceptance Criteria

- After logout, the session cookie is cleared
- With phase 23 in place, captured sessions expire within 24 hours
- (Optional) Server-side revocation immediately invalidates logged-out sessions
