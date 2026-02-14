# Phase 25: API Tokens Stored in Plaintext

**Severity:** 🟠 High

## Problem

API tokens are stored as plaintext in the SQLite database. If the database is compromised, all tokens are immediately usable.

## Fix

Store a SHA-256 hash instead of the raw token:

```go
func hashToken(token string) string {
    h := sha256.Sum256([]byte(token))
    return hex.EncodeToString(h[:])
}

func (d *DB) CreateToken(token, userName, userEmail string) error {
    _, err := d.Exec(`INSERT INTO tokens (token, user_name, user_email, expires_at)
        VALUES (?, ?, ?, datetime('now', '+90 days'))`,
        hashToken(token), userName, userEmail)
    return err
}

func (d *DB) GetUserByToken(token string) (name, email string, err error) {
    err = d.QueryRow(`SELECT user_name, user_email FROM tokens
        WHERE token = ? AND expires_at > CURRENT_TIMESTAMP`,
        hashToken(token)).Scan(&name, &email)
    return
}
```

The plaintext token is returned to the user once; only the hash is persisted.

## Files to Change

- `internal/db/db.go` — `CreateToken`, `GetUserByToken`, add `hashToken`

## Acceptance Criteria

- Tokens in the database are SHA-256 hashes, not plaintext
- CLI authentication still works (token is hashed on lookup)
- Existing plaintext tokens will need a one-time migration or re-generation
