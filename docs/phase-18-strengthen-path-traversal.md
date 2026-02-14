# Phase 18: Strengthen Path Traversal Check in Design File Serving

## Problem

`handleDesignFile` in `internal/api/designs.go` uses a simple string check to prevent path traversal:

```go
if strings.Contains(filePath, "..") {
    http.Error(w, "invalid path", http.StatusBadRequest)
    return
}
```

While Go's `PathValue` decodes the path before this check (mitigating encoded bypasses), the approach is fragile. A stronger check would validate the resolved path stays within the expected storage directory.

## Fix

After constructing the full path with `filepath.Join`, verify it is still within the version's storage directory using `filepath.Clean` and `strings.HasPrefix` — the same pattern already used in `SaveUpload`:

```go
fullPath := h.Storage.GetFilePath(versionID, filePath)
baseDir := filepath.Clean(h.Storage.GetFilePath(versionID, "")) + string(os.PathSeparator)
if !strings.HasPrefix(fullPath, baseDir) {
    http.Error(w, "invalid path", http.StatusBadRequest)
    return
}
```

## Files to Change

- `internal/api/designs.go`

## Acceptance Criteria

- Requests with path traversal attempts return 400
- Legitimate nested file paths (e.g. `images/logo.png`) still work
- Validation is based on resolved path, not string pattern matching
