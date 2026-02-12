# Phase 2: File Storage + Upload API + Static Serving

## Goal
Build the storage layer for uploaded design files, the upload API endpoint, and static file serving so uploaded designs can be viewed in a browser.

## Prerequisites
Phase 1 complete — project scaffold, go.mod, and `internal/db` package exist with all CRUD methods.

## What to Build

### 1. Storage Layer (`internal/storage/storage.go`)

A `Storage` struct that manages uploaded design files on disk.

```go
type Storage struct {
    BasePath string // e.g., "./data/uploads"
}
```

Methods:
- `New(basePath string) *Storage` — creates base directory if not exists
- `SaveUpload(versionID string, zipData io.Reader) error`
  - Creates directory at `{BasePath}/{versionID}/`
  - Extracts zip contents into that directory
  - Validates zip isn't empty and contains at least one `.html` file
- `GetFilePath(versionID, filepath string) string` — returns full disk path for a file
- `ListHTMLFiles(versionID string) ([]string, error)` — returns list of `.html` filenames in the version's directory (for multi-page navigation)

### 2. HTTP Server Setup (`cmd/server/main.go`)

Update the placeholder server to be a real HTTP server:
- Accept `--port` flag (default 8080)
- Accept `--db` flag for SQLite path (default `./data/design-reviewer.db`)
- Accept `--uploads` flag for upload directory (default `./data/uploads`)
- Initialize `db.New()` and `storage.New()`
- Pass both to API handlers
- Register routes and start listening

### 3. Upload API (`internal/api/upload.go`)

`POST /api/upload`
- Accepts multipart form data with fields:
  - `file` — the zip file
  - `name` — project name
- Logic:
  1. Read the zip file from the request
  2. Look up project by name (`db.GetProjectByName`)
  3. If not found, create new project (`db.CreateProject`)
  4. Create new version (`db.CreateVersion`)
  5. Save zip to storage (`storage.SaveUpload`)
  6. Update project's `updated_at`
  7. Return JSON: `{"project_id": "...", "version_id": "...", "version_num": N, "url": "/projects/{project_id}"}`
- No auth check yet (added in Phase 8)

### 4. Static File Serving (`internal/api/designs.go`)

`GET /designs/{version_id}/{filepath...}`
- Serves the uploaded file from storage
- Sets appropriate Content-Type headers
- Returns 404 if file doesn't exist
- Security: prevent path traversal (reject paths with `..`)

### 5. API Handler Struct (`internal/api/api.go`)

Create a handler struct that holds dependencies:
```go
type Handler struct {
    DB      *db.DB
    Storage *storage.Storage
}
```

Provide a method to register all routes on an `http.ServeMux`:
```go
func (h *Handler) RegisterRoutes(mux *http.ServeMux)
```

## Verification
- Server starts with `go run ./cmd/server`
- `curl -F "file=@test.zip" -F "name=test-project" http://localhost:8080/api/upload` creates a project and version
- `curl http://localhost:8080/designs/{version_id}/index.html` serves the uploaded HTML file
- Uploading to the same project name increments version_num
