# Phase 6: Version History + Comment Carry-Over

## Goal
Add version history sidebar to the design viewer and implement comment carry-over logic so unresolved comments persist across versions.

## Prerequisites
Phases 1-5 complete — design viewer with annotation system works for a single version.

## What to Build

### 1. Version List API (`internal/api/versions.go`)

**`GET /api/projects/{id}/versions`**
- Returns all versions for a project, ordered by version_num DESC (newest first)
- Response format:
```json
[
  {
    "id": "uuid",
    "version_num": 3,
    "created_at": "2026-01-15T10:30:00Z"
  },
  {
    "id": "uuid",
    "version_num": 2,
    "created_at": "2026-01-14T09:00:00Z"
  }
]
```

### 2. Version Sidebar (in viewer page)

Populate the sidebar placeholder from Phase 4:
- Title: "Versions"
- List of versions with version number and date
- Current version is highlighted
- Clicking a version switches the viewer to that version:
  - Updates iframe src to the new version's files
  - Reloads comments for the new version
  - Resets page tabs to the new version's HTML files
  - Updates URL query param: `/projects/{id}?version={version_id}`

### 3. Comment Carry-Over Logic

This is the core logic. When viewing a version, the comments shown should include:

1. Comments created directly on this version
2. **Unresolved** comments from ALL previous versions of the same project

The DB method `GetUnresolvedCommentsUpTo(versionID)` should:
- Find the project and version_num for the given version
- Return all comments where:
  - `version_id` matches the given version, OR
  - The comment is unresolved AND belongs to a version of the same project with a lower version_num
- This means: once a comment is resolved on any version, it stops appearing on newer versions

SQL approach:
```sql
SELECT c.* FROM comments c
JOIN versions v ON c.version_id = v.id
WHERE v.project_id = ?
  AND v.version_num <= ?
  AND (c.version_id = ? OR c.resolved = 0)
ORDER BY c.created_at ASC
```

Note: Comments that were resolved on the current version should still show (as resolved). Only comments resolved on *previous* versions are hidden.

Refined logic:
```sql
SELECT c.* FROM comments c
JOIN versions v ON c.version_id = v.id
WHERE v.project_id = ?
  AND (
    c.version_id = ?
    OR (c.resolved = 0 AND v.version_num < ?)
  )
ORDER BY c.created_at ASC
```

### 4. Version Switching (JavaScript)

In `web/static/viewer.js`, add version switching:
- Fetch version list from API on page load
- Render version list in sidebar
- On version click:
  1. Fetch HTML file list for the new version (or include in version API response)
  2. Update iframe src
  3. Fetch comments for the new version
  4. Re-render pins and page tabs
  5. Update browser URL with `?version=` param (use `history.replaceState`)

### 5. Register Route

Add to `Handler.RegisterRoutes()`:
- `GET /api/projects/{id}/versions`

## Verification
- Upload 2+ versions of the same project
- Version sidebar shows all versions, newest first
- Clicking a version switches the iframe to that version's files
- Create a comment on version 1, leave it unresolved
- Upload version 2 → the unresolved comment appears on version 2
- Resolve the comment on version 2
- Upload version 3 → the resolved comment does NOT appear on version 3
- Resolved comments remain visible on the version where they were resolved
