package api

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/ab/design-reviewer/internal/auth"
)

// webMiddleware checks for a valid session cookie; redirects to login if missing.
func (h *Handler) webMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie("session")
		if err != nil || cookie.Value == "" {
			http.SetCookie(w, &http.Cookie{
				Name:     "redirect_to",
				Value:    r.URL.RequestURI(),
				Path:     "/",
				HttpOnly: true,
				SameSite: http.SameSiteLaxMode,
				MaxAge:   300,
			})
			http.Redirect(w, r, "/login", http.StatusFound)
			return
		}
		u, err := auth.VerifySession(h.Auth.SessionSecret, cookie.Value)
		if err != nil {
			http.Redirect(w, r, "/login", http.StatusFound)
			return
		}
		if u.SessionID != "" {
			if _, _, err := h.DB.GetSession(u.SessionID); err != nil {
				http.Redirect(w, r, "/login", http.StatusFound)
				return
			}
		}
		ctx := auth.SetUserInContext(r.Context(), u.Name, u.Email)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// apiMiddleware checks for Bearer token or session cookie; returns 401 if missing.
func (h *Handler) apiMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Try Bearer token first
		if authHeader := r.Header.Get("Authorization"); strings.HasPrefix(authHeader, "Bearer ") {
			token := strings.TrimPrefix(authHeader, "Bearer ")
			name, email, err := h.DB.GetUserByToken(token)
			if err == nil {
				ctx := auth.SetUserInContext(r.Context(), name, email)
				next.ServeHTTP(w, r.WithContext(ctx))
				return
			}
		}
		// Try session cookie
		if cookie, err := r.Cookie("session"); err == nil && cookie.Value != "" {
			if u, err := auth.VerifySession(h.Auth.SessionSecret, cookie.Value); err == nil {
				if u.SessionID != "" {
					if _, _, err := h.DB.GetSession(u.SessionID); err != nil {
						w.Header().Set("Content-Type", "application/json")
						w.WriteHeader(http.StatusUnauthorized)
						json.NewEncoder(w).Encode(map[string]string{"error": "unauthorized"})
						return
					}
				}
				ctx := auth.SetUserInContext(r.Context(), u.Name, u.Email)
				next.ServeHTTP(w, r.WithContext(ctx))
				return
			}
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]string{"error": "unauthorized"})
	})
}

// projectAccess checks that the authenticated user can access the project identified by {id}.
func (h *Handler) projectAccess(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, email := auth.GetUserFromContext(r.Context())
		if email == "" {
			http.NotFound(w, r)
			return
		}
		projectID := r.PathValue("id")
		ok, err := h.DB.CanAccessProject(projectID, email)
		if err != nil || !ok {
			http.NotFound(w, r)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// versionAccess checks access via version_id → project lookup.
func (h *Handler) versionAccess(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, email := auth.GetUserFromContext(r.Context())
		if email == "" {
			http.NotFound(w, r)
			return
		}
		versionID := r.PathValue("id")
		if versionID == "" {
			versionID = r.PathValue("version_id")
		}
		v, err := h.DB.GetVersion(versionID)
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

// commentAccess checks access via comment_id → version → project lookup.
func (h *Handler) commentAccess(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, email := auth.GetUserFromContext(r.Context())
		if email == "" {
			http.NotFound(w, r)
			return
		}
		c, err := h.DB.GetComment(r.PathValue("id"))
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

// ownerOnly checks that the authenticated user is the project owner.
func (h *Handler) ownerOnly(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, email := auth.GetUserFromContext(r.Context())
		if email == "" {
			http.NotFound(w, r)
			return
		}
		projectID := r.PathValue("id")
		owner, err := h.DB.GetProjectOwner(projectID)
		if err != nil {
			http.NotFound(w, r)
			return
		}
		if owner != email {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusForbidden)
			json.NewEncoder(w).Encode(map[string]string{"error": "owner only"})
			return
		}
		next.ServeHTTP(w, r)
	})
}
