# Phase 34: Invite Links Never Expire

**Severity:** 🟢 Low

## Problem

The `expires_at` column in `project_invites` is nullable and defaults to NULL, so invite tokens are valid forever.

## Fix

Set a default expiration when creating invites:

```go
err := d.QueryRow(
    `INSERT INTO project_invites (id, project_id, token, created_by, expires_at)
     VALUES (?, ?, ?, ?, datetime('now', '+7 days')) RETURNING created_at`,
    inv.ID, inv.ProjectID, inv.Token, inv.CreatedBy,
).Scan(&inv.CreatedAt)
```

## Files to Change

- `internal/db/db.go` — `CreateInvite`

## Acceptance Criteria

- New invite links expire after 7 days
- Expired invite tokens are rejected when used
- Existing NULL-expiry invites are treated as expired (or migrated)
