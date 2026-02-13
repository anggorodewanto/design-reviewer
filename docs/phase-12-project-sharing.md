# Phase 12: Project Sharing & Access Control

## Goal
Add project ownership and invite-link sharing so users only see their own projects and projects they've been invited to. The seed project remains visible to everyone.

## Prerequisites
Phase 11 complete — CLI and web app fully functional with auth.

## What to Build

### 1. Database Schema Changes (`internal/db/db.go`)

Add `owner_email` column to `projects` table:
```sql
ALTER TABLE projects ADD COLUMN owner_email TEXT;
```

Create new tables:
```sql
CREATE TABLE IF NOT EXISTS project_invites (
    id TEXT PRIMARY KEY,
    project_id TEXT NOT NULL REFERENCES projects(id),
    token TEXT NOT NULL UNIQUE,
    created_by TEXT NOT NULL,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    expires_at DATETIME
);

CREATE TABLE IF NOT EXISTS project_members (
    project_id TEXT NOT NULL REFERENCES projects(id),
    user_email TEXT NOT NULL,
    added_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (project_id, user_email)
);
```

### 2. Database Queries (`internal/db/db.go`)

Update `Project` struct to include `OwnerEmail *string`.

**Modified queries:**
- `CreateProject(name, ownerEmail string)` — accept and store owner email
- `ListProjectsForUser(email string)` — replace `ListProjects()` for user-facing queries:
  ```sql
  SELECT p.* FROM projects p
  WHERE p.owner_email IS NULL
     OR p.owner_email = ?
     OR EXISTS (SELECT 1 FROM project_members pm WHERE pm.project_id = p.id AND pm.user_email = ?)
  ORDER BY p.updated_at DESC
  ```
- `CanAccessProject(projectID, email string) bool` — same logic for single project check

**New queries:**
- `CreateInvite(projectID, createdBy string) (*ProjectInvite, error)` — generate UUID + random 32-byte hex token
- `GetInviteByToken(token string) (*ProjectInvite, error)` — lookup, check expiry
- `DeleteInvite(id string) error`
- `AddMember(projectID, email string) error` — INSERT OR IGNORE
- `ListMembers(projectID string) ([]ProjectMember, error)`
- `RemoveMember(projectID, email string) error`
- `GetProjectOwner(projectID string) (string, error)`

### 3. Access Control Middleware (`internal/api/middleware.go`)

Add `projectAccessMiddleware` that wraps project-specific routes:
- Extract project ID from URL (or version ID → look up project)
- Get user email from session/token
- Call `CanAccessProject()` — return 404 if denied (not 403, to avoid leaking project existence)

Apply to all routes under `/api/projects/:id/*`, `/api/versions/:id/*`, `/projects/:id`, and `/designs/:version_id/*`.

### 4. Owner-Only Middleware

Add `ownerOnlyMiddleware` for sharing management routes:
- Check `GetProjectOwner()` matches current user
- Return 403 if not owner

Apply to: `POST /api/projects/:id/invites`, `DELETE /api/projects/:id/invites/:invite_id`, `DELETE /api/projects/:id/members/:email`.

### 5. API Handlers (`internal/api/sharing.go`)

**`POST /api/projects/:id/invites`** (owner only)
- Call `CreateInvite(projectID, userEmail)`
- Return JSON: `{ "invite_url": "{BASE_URL}/invite/{token}" }`

**`DELETE /api/projects/:id/invites/:invite_id`** (owner only)
- Call `DeleteInvite(id)`

**`GET /api/projects/:id/members`**
- Call `ListMembers(projectID)`
- Return JSON array of `{ "email": "...", "added_at": "..." }`

**`DELETE /api/projects/:id/members/:email`** (owner only)
- Call `RemoveMember(projectID, email)`
- Cannot remove self (owner)

**`GET /invite/:token`** (any authenticated user)
- Look up invite by token, check not expired
- If user not logged in, redirect to `/auth/google/login?redirect=/invite/{token}`
- Add user to `project_members`
- Redirect to `/projects/:id`

### 6. Upload Handler Update (`internal/api/upload.go`)

In `handleUpload`, when creating a new project:
- Extract user email from request context
- Pass to `CreateProject(name, email)`

When pushing a new version to an existing project:
- Check `CanAccessProject()` before allowing upload

### 7. Project List Update (`internal/api/projects.go`)

- `handleHome` and `handleListProjects`: use `ListProjectsForUser(email)` instead of `ListProjects()`
- Extract user email from session context

### 8. UI Changes

**Project page** — add "Share" button (visible only to owner):
- Clicking generates an invite link via `POST /api/projects/:id/invites`
- Display the link in a copyable text field
- Show current members list with remove buttons

**Invite acceptance page** (`web/templates/invite.html`):
- Simple page shown briefly: "Joining project..." then redirect
- Error state if token is invalid/expired

### 9. Seed Project (`internal/seed/seed.go`)

No changes needed. `CreateProject` is called without an owner email for the seed, so `owner_email` remains NULL, making it visible to all users.

## Route Registration

```go
// Sharing routes (inside authenticated group)
mux.Handle("POST /api/projects/{id}/invites", ownerOnly(handleCreateInvite))
mux.Handle("DELETE /api/projects/{id}/invites/{inviteID}", ownerOnly(handleDeleteInvite))
mux.Handle("GET /api/projects/{id}/members", projectAccess(handleListMembers))
mux.Handle("DELETE /api/projects/{id}/members/{email}", ownerOnly(handleRemoveMember))

// Invite acceptance (authenticated but no project access check)
mux.Handle("GET /invite/{token}", webMiddleware(handleAcceptInvite))
```

## Verification

1. New user signs in → sees only the seed project
2. User pushes a design via CLI → project created with their email as owner
3. Owner clicks "Share" → gets an invite link
4. Another user visits the invite link → gets added as member, redirected to project
5. Member can view, comment, but cannot share or change status
6. Owner can remove members
7. Seed project remains visible to everyone
8. Non-member visiting `/projects/:id` directly gets 404
9. CLI `push` to a project the user doesn't have access to returns error
