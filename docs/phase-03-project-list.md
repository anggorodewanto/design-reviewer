# Phase 3: Project List — API + Web Page

## Goal
Build the project list API and the home page that displays all design projects.

## Prerequisites
Phases 1-2 complete — DB layer, storage, upload API, and server routing exist.

## What to Build

### 1. Project List API (`internal/api/projects.go`)

`GET /api/projects`
- Returns JSON array of all projects
- Each project includes: id, name, status, version_count, last updated
- Query joins projects with versions to get version count
- Add a `ListProjectsWithVersionCount()` method to the DB if needed, or compute in the handler
- Response format:
```json
[
  {
    "id": "uuid",
    "name": "homepage-redesign",
    "status": "in_review",
    "version_count": 3,
    "updated_at": "2026-01-15T10:30:00Z"
  }
]
```

### 2. HTML Template Engine

Set up Go's `html/template` for rendering pages:
- Load templates from `web/templates/`
- Use a base layout pattern (layout.html with a content block)

Create `web/templates/layout.html`:
- HTML5 boilerplate
- Link to `/static/style.css`
- A `{{template "content" .}}` block

### 3. Home Page (`web/templates/home.html`)

`GET /` — renders the project list page

Content:
- Page title: "Design Reviewer"
- Table/card list of projects showing:
  - Project name (links to `/projects/{id}`)
  - Status badge (Draft / In Review / Approved / Handed Off)
  - Version count
  - Last updated (human-readable relative time like "2 hours ago")
- Empty state message when no projects exist

### 4. Static File Serving for Web Assets

`GET /static/{filepath...}`
- Serves files from `web/static/` directory
- This is for the web app's own CSS/JS, separate from uploaded designs

### 5. Base Stylesheet (`web/static/style.css`)

Minimal clean CSS:
- Clean sans-serif font
- Simple table or card layout for project list
- Status badge colors:
  - Draft: gray
  - In Review: blue
  - Approved: green
  - Handed Off: purple
- Responsive basics

### 6. Register Routes

Add to `Handler.RegisterRoutes()`:
- `GET /` → home page handler
- `GET /api/projects` → JSON API
- `GET /static/` → static file server

## Verification
- `curl http://localhost:8080/api/projects` returns JSON array (empty initially)
- Upload a project via Phase 2's endpoint, then verify it appears in the list
- Browser at `http://localhost:8080/` shows the project list page with correct data
- Status badges display with correct colors
