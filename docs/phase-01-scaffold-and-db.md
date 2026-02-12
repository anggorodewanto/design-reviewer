# Phase 1: Project Scaffold + Database Layer

## Goal
Set up the Go project structure, module, and SQLite database layer with all CRUD operations.

## Context
This is a design review tool. Two components: a web server and a CLI (same repo). This phase builds the foundation — project structure and database.

## What to Build

### 1. Go Module
- Initialize `go.mod` (module name: `github.com/ab/design-reviewer` or similar)
- Add dependency: `github.com/mattn/go-sqlite3`
- Add dependency: `github.com/google/uuid`

### 2. Project Structure
Create this directory layout:
```
design-reviewer/
├── cmd/
│   ├── server/
│   │   └── main.go        # placeholder: starts HTTP server on :8080, prints "server running"
│   └── cli/
│       └── main.go        # placeholder: prints "design-reviewer CLI"
├── internal/
│   ├── api/               # empty package, placeholder file
│   ├── db/
│   │   └── db.go          # SQLite connection + schema + CRUD
│   ├── storage/           # empty package, placeholder file
│   └── auth/              # empty package, placeholder file
├── web/
│   ├── static/            # empty dir with .gitkeep
│   └── templates/         # empty dir with .gitkeep
├── go.mod
└── Dockerfile             # placeholder
```

### 3. Database Schema (`internal/db/db.go`)

Initialize SQLite with these tables:

```sql
CREATE TABLE IF NOT EXISTS projects (
    id TEXT PRIMARY KEY,
    name TEXT UNIQUE NOT NULL,
    status TEXT NOT NULL DEFAULT 'draft',
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS versions (
    id TEXT PRIMARY KEY,
    project_id TEXT NOT NULL REFERENCES projects(id),
    version_num INTEGER NOT NULL,
    storage_path TEXT NOT NULL,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS comments (
    id TEXT PRIMARY KEY,
    version_id TEXT NOT NULL REFERENCES versions(id),
    page TEXT NOT NULL,
    x_percent REAL NOT NULL,
    y_percent REAL NOT NULL,
    author_name TEXT NOT NULL,
    author_email TEXT NOT NULL,
    body TEXT NOT NULL,
    resolved BOOLEAN NOT NULL DEFAULT 0,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS replies (
    id TEXT PRIMARY KEY,
    comment_id TEXT NOT NULL REFERENCES comments(id),
    author_name TEXT NOT NULL,
    author_email TEXT NOT NULL,
    body TEXT NOT NULL,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
```

Enable WAL mode and foreign keys.

### 4. DB Functions (`internal/db/db.go`)

Expose a `DB` struct wrapping `*sql.DB` with these methods:

**Projects:**
- `CreateProject(name string) (*Project, error)` — generates UUID, inserts with status "draft"
- `GetProject(id string) (*Project, error)`
- `GetProjectByName(name string) (*Project, error)`
- `ListProjects() ([]Project, error)` — returns all projects ordered by updated_at DESC
- `UpdateProjectStatus(id, status string) error` — validates status is one of: draft, in_review, approved, handed_off

**Versions:**
- `CreateVersion(projectID, storagePath string) (*Version, error)` — auto-increments version_num per project
- `GetVersion(id string) (*Version, error)`
- `ListVersions(projectID string) ([]Version, error)` — ordered by version_num DESC
- `GetLatestVersion(projectID string) (*Version, error)`

**Comments:**
- `CreateComment(versionID, page string, xPercent, yPercent float64, authorName, authorEmail, body string) (*Comment, error)`
- `GetCommentsForVersion(versionID string) ([]Comment, error)` — returns comments created on this version
- `GetUnresolvedCommentsUpTo(versionID string) ([]Comment, error)` — returns unresolved comments from this version and all previous versions of the same project (for carry-over)
- `ToggleResolve(commentID string) error` — flips resolved boolean

**Replies:**
- `CreateReply(commentID, authorName, authorEmail, body string) (*Reply, error)`
- `GetReplies(commentID string) ([]Reply, error)` — ordered by created_at ASC

### 5. Model Structs

```go
type Project struct {
    ID        string
    Name      string
    Status    string
    CreatedAt time.Time
    UpdatedAt time.Time
}

type Version struct {
    ID         string
    ProjectID  string
    VersionNum int
    StoragePath string
    CreatedAt  time.Time
}

type Comment struct {
    ID          string
    VersionID   string
    Page        string
    XPercent    float64
    YPercent    float64
    AuthorName  string
    AuthorEmail string
    Body        string
    Resolved    bool
    CreatedAt   time.Time
}

type Reply struct {
    ID          string
    CommentID   string
    AuthorName  string
    AuthorEmail string
    Body        string
    CreatedAt   time.Time
}
```

### 6. `New(dbPath string) (*DB, error)` Constructor
- Opens SQLite at the given path
- Runs schema migration (CREATE TABLE IF NOT EXISTS)
- Returns `*DB`

## Verification
- `go build ./...` succeeds
- `cmd/server/main.go` runs and prints a message
- `cmd/cli/main.go` runs and prints a message
- DB initializes and creates tables when `New()` is called
