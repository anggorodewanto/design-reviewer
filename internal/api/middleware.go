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
			http.Redirect(w, r, "/login", http.StatusFound)
			return
		}
		u, err := auth.VerifySession(h.Auth.SessionSecret, cookie.Value)
		if err != nil {
			http.Redirect(w, r, "/login", http.StatusFound)
			return
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
