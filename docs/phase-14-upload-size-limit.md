# Phase 14: Add Upload Size Limit

## Problem

`handleUpload` in `internal/api/upload.go` reads the entire uploaded zip into memory with no size cap:

```go
var buf bytes.Buffer
if _, err := io.Copy(&buf, file); err != nil { ... }
```

An authenticated user can upload a multi-GB file and exhaust the server's memory (256MB on Fly.io), crashing the process.

## Fix

Wrap `r.Body` with `http.MaxBytesReader` at the start of `handleUpload` to enforce a maximum upload size (e.g. 50MB).

```go
r.Body = http.MaxBytesReader(w, r.Body, 50<<20) // 50MB
```

When exceeded, `r.FormFile` will return an error and the handler can return a 413 status.

## Files to Change

- `internal/api/upload.go`

## Acceptance Criteria

- Uploads exceeding the limit are rejected with HTTP 413
- Server memory is not exhausted by large uploads
