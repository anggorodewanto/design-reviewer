# Phase 21: IDOR — Comment Endpoints Lack Project Access Checks

**Severity:** 🔴 Critical

## Problem

Any authenticated user can reply to, resolve, or move ANY comment in the system by guessing/knowing a comment ID. The comment-level API routes only use `apiMiddleware` (which checks authentication) but skip `projectAccess` or `versionAccess`.

```go
// VULNERABLE — no project/version access check
mux.Handle("POST /api/comments/{id}/replies", h.apiMiddleware(apiCreateReply))
mux.Handle("PATCH /api/comments/{id}/resolve", h.apiMiddleware(apiToggleResolve))
mux.Handle("PATCH /api/comments/{id}/move", h.apiMiddleware(apiMoveComment))
```

## Fix

Add a `commentAccess` middleware that resolves comment → version → project and calls `CanAccessProject`:

```go
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

## Files to Change

- `internal/api/middleware.go` — add `commentAccess` middleware
- `internal/api/api.go` — wire middleware on comment routes
- `internal/db/db.go` — add `GetComment` method
- `internal/db/interface.go` — add `GetComment` to `DataStore`

## Acceptance Criteria

- Users without project access get 404 on comment reply/resolve/move endpoints
- Users with project access can still interact with comments normally
- `GetComment` returns the comment with its `VersionID` populated
