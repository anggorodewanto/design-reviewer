# Phase 32: CLI Uses Fixed Callback Port

**Severity:** 🟢 Low

## Problem

The CLI always listens on port 9876 for the OAuth callback, making it predictable for local attackers.

## Fix

Use `localhost:0` to get a random available port:

```go
listener, err := net.Listen("tcp", "localhost:0")
port := listener.Addr().(*net.TCPAddr).Port
```

## Files to Change

- `internal/cli/login.go`

## Acceptance Criteria

- CLI OAuth callback uses a random available port
- The redirect URI sent to the server reflects the actual port
- Login flow still completes successfully
