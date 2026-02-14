# Phase 29: No Request Body Size Limit on JSON Endpoints

**Severity:** 🟡 Medium

## Problem

Comment bodies, replies, and status updates accept unbounded request bodies. An attacker can send arbitrarily large payloads to exhaust memory.

## Fix

Add `http.MaxBytesReader` as the first line in each JSON handler:

```go
r.Body = http.MaxBytesReader(w, r.Body, 1<<20) // 1 MB
```

## Files to Change

- `internal/api/comments.go`
- `internal/api/projects.go`
- `internal/api/auth_handlers.go`

## Acceptance Criteria

- JSON request bodies larger than 1 MB are rejected with 413
- Normal-sized requests still work
