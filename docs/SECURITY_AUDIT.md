# Security Audit Report

**Date:** 2026-02-14
**Scope:** Full codebase review — all Go source, JavaScript, templates, Dockerfile, deployment config

---

## Summary

| Severity | Count |
|----------|-------|
| 🔴 Critical | 2 |
| 🟠 High | 4 |
| 🟡 Medium | 5 |
| 🟢 Low | 3 |

---

## 🔴 Critical

### 1. IDOR — Comment endpoints lack project access checks

**Files:** `internal/api/api.go` lines 100–102

Any authenticated user can reply to, resolve, or move ANY comment in the system by guessing/knowing a comment ID. The comment-level API routes only use `apiMiddleware` (which checks authentication) but skip `projectAccess` or `versionAccess`.

```go
// VULNERABLE — no project/version access check
mux.Handle("POST /api/comments/{id}/replies", h.apiMiddleware(apiCreateReply))
mux.Handle("PATCH /api/comments/{id}/resolve", h.apiMiddleware(apiToggleResolve))
mux.Handle("PATCH /api/comments/{id}/move", h.apiMiddleware(apiMoveComment))
```

**Fix:** Add a `commentAccess` middleware that resolves comment → version → project and calls `CanAccessProject`:

```go
// internal/api/middleware.go
func (h *Handler) commentAccess(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        _, email := auth.GetUserFromContext(r.Context())
        if email == "" {
            http.NotFound(w, r)
            return
        }
        commentID := r.PathValue("id")
        c, err := h.DB.GetComment(commentID)
        if err != nil {
            http.NotFound(w, r)
            return
        }
        v, err := h.DB.GetVersion(c.VersionID)
        if err != nil {
            http.NotFound(w, r)
            return
        }
        ok, err := h.DB.CanAccessProject(v.ProjectID, email)
        if err != nil || !ok {
            http.NotFound(w, r)
            return
        }
        next.ServeHTTP(w, r)
    })
}
```

Add `GetComment(id string) (*Comment, error)` to the DB layer and `DataStore` interface, then wire the middleware:

```go
mux.Handle("POST /api/comments/{id}/replies", h.apiMiddleware(h.commentAccess(apiCreateReply)))
mux.Handle("PATCH /api/comments/{id}/resolve", h.apiMiddleware(h.commentAccess(apiToggleResolve)))
mux.Handle("PATCH /api/comments/{id}/move", h.apiMiddleware(h.commentAccess(apiMoveComment)))
```

---

### 2. Zip bomb — No decompression size limit

**File:** `internal/storage/storage.go` — `SaveUpload`

The upload handler enforces a 50 MB limit on the compressed zip, but there is no limit on decompressed output. A crafted 50 MB zip can decompress to gigabytes, exhausting disk space.

There is also no limit on the number of files extracted.

```go
// VULNERABLE — unbounded copy
_, err = io.Copy(out, rc)
```

**Fix:**

```go
const maxDecompressedSize = 500 << 20 // 500 MB
const maxFileCount = 1000

// Inside SaveUpload, before extraction loop:
if len(zr.File) > maxFileCount {
    return fmt.Errorf("zip contains too many files (max %d)", maxFileCount)
}

// Replace the io.Copy call:
var totalWritten int64
// ...per file:
n, err := io.Copy(out, io.LimitReader(rc, maxDecompressedSize-totalWritten))
totalWritten += n
if totalWritten > maxDecompressedSize {
    return fmt.Errorf("decompressed size exceeds limit")
}
```

---

## 🟠 High

### 3. Sessions never expire

**File:** `internal/auth/auth.go` — `SignSession` / `VerifySession`

The signed session payload is `{"name":"…","email":"…"}` with no timestamp. A captured cookie value is valid forever as long as the session secret doesn't change.

**Fix:** Add an `exp` field to `User`:

```go
type User struct {
    Name      string `json:"name"`
    Email     string `json:"email"`
    ExpiresAt int64  `json:"exp,omitempty"`
}

// In SignSession, before marshalling:
u.ExpiresAt = time.Now().Add(24 * time.Hour).Unix()

// In VerifySession, after unmarshalling:
if u.ExpiresAt > 0 && time.Now().Unix() > u.ExpiresAt {
    return User{}, errors.New("session expired")
}
```

### 4. Logout doesn't invalidate sessions server-side

**File:** `internal/api/auth_handlers.go` — `handleLogout`

`ClearSessionCookie` only removes the cookie from the browser. The signed value remains valid if previously captured (e.g., via network sniffing over HTTP).

Combined with finding #3, this means a stolen session is usable indefinitely. Adding session expiration (finding #3) mitigates this. For full revocation, switch to server-side session storage or maintain a revocation list.

### 5. API tokens stored in plaintext

**File:** `internal/db/db.go` — `CreateToken` / `GetUserByToken`

If the SQLite database is compromised, all API tokens are immediately usable.

**Fix:** Store a SHA-256 hash instead of the raw token:

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

### 6. Attribute injection XSS in sharing.js

**File:** `web/static/sharing.js` line 50, `web/static/annotations.js`

The `esc()` helper escapes `<`, `>`, `&` but NOT `"` or `'`. When the result is placed inside an HTML attribute, a value containing a double quote breaks out of the attribute:

```js
// VULNERABLE — esc() doesn't escape quotes
'<button class="btn-remove" data-email="' + esc(m.email) + '">'
```

**Fix** — Update `esc()` in both `sharing.js` and `annotations.js`:

```js
function esc(s) {
    var d = document.createElement('div');
    d.textContent = s || '';
    return d.innerHTML.replace(/"/g, '&quot;').replace(/'/g, '&#39;');
}
```

---

## 🟡 Medium

### 7. No security headers

**File:** `cmd/server/main.go`

The server sets no security headers. Missing: `X-Content-Type-Options`, `X-Frame-Options`, `Referrer-Policy`, `Permissions-Policy`.

**Fix** — Wrap the mux in `main.go`:

```go
func securityHeaders(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        w.Header().Set("X-Content-Type-Options", "nosniff")
        w.Header().Set("X-Frame-Options", "DENY")
        w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
        w.Header().Set("Permissions-Policy", "camera=(), microphone=(), geolocation=()")
        next.ServeHTTP(w, r)
    })
}

// In main():
log.Fatal(http.ListenAndServe(addr, securityHeaders(mux)))
```

### 8. No rate limiting

No rate limiting on any endpoint. Brute-force attacks on invite tokens, comment spam, and upload abuse are all possible.

**Fix:** Add per-IP rate limiting using `golang.org/x/time/rate` or a middleware like `httprate`.

### 9. No request body size limit on JSON endpoints

**Files:** `internal/api/comments.go`, `projects.go`, `auth_handlers.go`

Comment bodies, replies, and status updates accept unbounded request bodies.

**Fix** — Add as the first line in each JSON handler:

```go
r.Body = http.MaxBytesReader(w, r.Body, 1<<20) // 1 MB
```

### 10. Dockerfile runs as root

**File:** `Dockerfile`

The runtime container has no `USER` directive and runs as root.

**Fix:**

```dockerfile
FROM alpine:3.21
RUN apk add --no-cache ca-certificates && adduser -D -u 1001 appuser
WORKDIR /app
COPY --from=builder /app/server .
COPY --from=builder /app/design-reviewer .
COPY web/ ./web/
USER appuser
EXPOSE 8080
CMD ["./server", "--port", "8080", "--db", "/data/design-reviewer.db", "--uploads", "/data/uploads"]
```

### 11. Static file server exposes directory listings

**File:** `internal/api/api.go` line 68

`http.FileServer` returns directory listings when no `index.html` exists.

**Fix:**

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

---

## 🟢 Low

### 12. CLI uses fixed callback port

**File:** `internal/cli/login.go` line 42

The CLI always listens on port 9876 for the OAuth callback, making it predictable for local attackers.

**Fix:** Use `localhost:0` and extract the assigned port:

```go
listener, err := net.Listen("tcp", "localhost:0")
port := listener.Addr().(*net.TCPAddr).Port
```

### 13. OAuth state cookie missing Secure flag

**File:** `internal/api/auth_handlers.go` lines 62–67

The `oauth_state` cookie does not set `Secure: true` when the server runs over HTTPS.

**Fix:**

```go
http.SetCookie(w, &http.Cookie{
    Name:     "oauth_state",
    Value:    state,
    Path:     "/",
    HttpOnly: true,
    Secure:   strings.HasPrefix(h.Auth.BaseURL, "https://"),
    SameSite: http.SameSiteLaxMode,
})
```

Apply the same fix in `handleCLILogin`.

### 14. Invite links never expire

**File:** `internal/db/db.go` — `CreateInvite`

The `expires_at` column is nullable and defaults to NULL, so invite tokens are valid forever.

**Fix:**

```go
err := d.QueryRow(
    `INSERT INTO project_invites (id, project_id, token, created_by, expires_at)
     VALUES (?, ?, ?, ?, datetime('now', '+7 days')) RETURNING created_at`,
    inv.ID, inv.ProjectID, inv.Token, inv.CreatedBy,
).Scan(&inv.CreatedAt)
```

---

## ✅ Already Secure

- All SQL queries use parameterized statements — no SQL injection
- Go `html/template` auto-escapes template variables — no server-side XSS
- Path traversal checks in `handleDesignFile` and `SaveUpload`
- OAuth state parameter validated against HttpOnly cookie
- Session cookie: HttpOnly, SameSite:Lax, Secure (conditional on HTTPS)
- HMAC-SHA256 with constant-time comparison (`hmac.Equal`)
- API tokens generated with 32 bytes of `crypto/rand`
- `.env` is gitignored
- CLI config file written with 0600 permissions
- Iframe uses `sandbox="allow-same-origin"` — no script execution in uploaded designs
- Fly.io config uses `force_https: true`
