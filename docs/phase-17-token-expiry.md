# Phase 17: Add API Token Expiry

## Problem

API tokens in the `tokens` table have no expiration. A leaked or stolen token grants permanent access to the user's account and projects.

## Fix

1. Add an `expires_at` column to the `tokens` table (default: 90 days from creation)
2. Update `GetUserByToken` to check `expires_at > CURRENT_TIMESTAMP`
3. Update `CreateToken` to set the expiry
4. Handle the migration for existing databases (ALTER TABLE)

## Files to Change

- `internal/db/db.go` — schema, `CreateToken`, `GetUserByToken`

## Acceptance Criteria

- New tokens are created with a 90-day expiry
- Expired tokens are rejected with 401
- Existing databases are migrated with the new column
