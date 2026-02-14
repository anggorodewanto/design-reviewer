# Phase 31: Static File Server Exposes Directory Listings

**Severity:** 🟡 Medium

## Problem

`http.FileServer` returns directory listings when no `index.html` exists, exposing the file structure of the static directory.

## Fix

Wrap the file system to reject directory opens:

```go
type noDirFS struct{ http.FileSystem }

func (n noDirFS) Open(name string) (http.File, error) {
    f, err := n.FileSystem.Open(name)
    if err != nil {
        return nil, err
    }
    if s, _ := f.Stat(); s.IsDir() {
        f.Close()
        return nil, os.ErrNotExist
    }
    return f, nil
}

// In RegisterRoutes:
mux.Handle("GET /static/", http.StripPrefix("/static/",
    http.FileServer(noDirFS{http.Dir(h.StaticDir)})))
```

## Files to Change

- `internal/api/api.go`

## Acceptance Criteria

- Requesting `/static/` returns 404 instead of a directory listing
- Individual static files (CSS, JS) still serve correctly
