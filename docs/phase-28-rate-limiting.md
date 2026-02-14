# Phase 28: No Rate Limiting

**Severity:** 🟡 Medium

## Problem

No rate limiting on any endpoint. Brute-force attacks on invite tokens, comment spam, and upload abuse are all possible.

## Fix

Add per-IP rate limiting using `golang.org/x/time/rate` or a middleware like `httprate`. Apply stricter limits to auth and invite endpoints, and a general limit to all other routes.

## Files to Change

- `cmd/server/main.go` or `internal/api/middleware.go` — add rate limiting middleware
- `go.mod` — add rate limiting dependency if using an external package

## Acceptance Criteria

- Requests exceeding the rate limit receive HTTP 429
- Rate limits are per-IP
- Auth and invite endpoints have stricter limits than general endpoints
