# Phase 15: Validate CLI Login Port Parameter

## Problem

`handleCLILogin` in `internal/api/auth_handlers.go` accepts a user-controlled `port` query parameter that is embedded into a redirect URL without validation:

```go
port := state[idx+1:]
redirectURL := fmt.Sprintf("http://localhost:%s/callback?token=%s&name=%s", port, apiToken, ...)
```

A crafted port value like `9876@evil.com/steal#` could redirect the API token to an attacker-controlled server.

## Fix

Validate that the port parameter is a numeric value within the valid port range (1–65535) before using it in the redirect URL. Reject non-numeric values.

```go
portNum, err := strconv.Atoi(port)
if err != nil || portNum < 1 || portNum > 65535 {
    http.Error(w, "invalid port", http.StatusBadRequest)
    return
}
```

## Files to Change

- `internal/api/auth_handlers.go`

## Acceptance Criteria

- Only numeric port values (1–65535) are accepted
- Non-numeric or out-of-range values return 400
- Token is never redirected to a non-localhost URL
