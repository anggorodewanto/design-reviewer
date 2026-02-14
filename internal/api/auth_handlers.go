package api

import (
	"encoding/json"
	"fmt"
	"html/template"
	"net/http"
	"net/url"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/ab/design-reviewer/internal/auth"
	"golang.org/x/oauth2"
)

// OAuthProvider abstracts OAuth operations for testability.
type OAuthProvider interface {
	AuthCodeURL(state string, opts ...oauth2.AuthCodeOption) string
	Exchange(r *http.Request, code string) (*oauth2.Token, error)
	GetUserInfo(token *oauth2.Token) (name, email string, err error)
}

// GoogleOAuth implements OAuthProvider using real Google OAuth.
type GoogleOAuth struct {
	Config *oauth2.Config
}

func (g *GoogleOAuth) AuthCodeURL(state string, opts ...oauth2.AuthCodeOption) string {
	return g.Config.AuthCodeURL(state, opts...)
}

func (g *GoogleOAuth) Exchange(r *http.Request, code string) (*oauth2.Token, error) {
	return g.Config.Exchange(r.Context(), code)
}

func (g *GoogleOAuth) GetUserInfo(token *oauth2.Token) (name, email string, err error) {
	return auth.GetUserInfo(token)
}

func (h *Handler) handleLoginPage(w http.ResponseWriter, r *http.Request) {
	tmpl, err := template.ParseFiles(
		filepath.Join(h.TemplatesDir, "layout.html"),
		filepath.Join(h.TemplatesDir, "login.html"),
	)
	if err != nil {
		http.Error(w, "template error", http.StatusInternalServerError)
		return
	}
	tmpl.Execute(w, struct{ UserName string }{})
}

func (h *Handler) handleGoogleLogin(w http.ResponseWriter, r *http.Request) {
	state := auth.GenerateState()
	http.SetCookie(w, &http.Cookie{
		Name:     "oauth_state",
		Value:    state,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
	url := h.OAuthConfig.AuthCodeURL(state)
	http.Redirect(w, r, url, http.StatusFound)
}

func (h *Handler) handleGoogleCallback(w http.ResponseWriter, r *http.Request) {
	// Validate state
	stateCookie, err := r.Cookie("oauth_state")
	if err != nil || stateCookie.Value != r.URL.Query().Get("state") {
		http.Error(w, "invalid state", http.StatusBadRequest)
		return
	}
	// Clear state cookie
	http.SetCookie(w, &http.Cookie{Name: "oauth_state", Value: "", Path: "/", MaxAge: -1})

	code := r.URL.Query().Get("code")
	token, err := h.OAuthConfig.Exchange(r, code)
	if err != nil {
		http.Error(w, "oauth exchange failed", http.StatusInternalServerError)
		return
	}

	name, email, err := h.OAuthConfig.GetUserInfo(token)
	if err != nil {
		http.Error(w, "failed to get user info", http.StatusInternalServerError)
		return
	}

	// Check if this is a CLI flow (state contains ":port")
	state := stateCookie.Value
	if idx := strings.LastIndex(state, ":"); idx > 0 {
		port := state[idx+1:]
		apiToken := auth.GenerateAPIToken()
		if err := h.DB.CreateToken(apiToken, name, email); err != nil {
			http.Error(w, "failed to create token", http.StatusInternalServerError)
			return
		}
		redirectURL := fmt.Sprintf("http://localhost:%s/callback?token=%s&name=%s", port, apiToken, url.QueryEscape(name))
		http.Redirect(w, r, redirectURL, http.StatusFound)
		return
	}

	secure := strings.HasPrefix(h.Auth.BaseURL, "https://")
	sessionID := auth.GenerateSessionID()
	if err := h.DB.CreateSession(sessionID, name, email); err != nil {
		http.Error(w, "session error", http.StatusInternalServerError)
		return
	}
	if err := auth.SetSessionCookie(w, h.Auth.SessionSecret, auth.User{Name: name, Email: email, SessionID: sessionID}, secure); err != nil {
		http.Error(w, "session error", http.StatusInternalServerError)
		return
	}
	redirectTo := "/"
	if c, err := r.Cookie("redirect_to"); err == nil && c.Value != "" && strings.HasPrefix(c.Value, "/") {
		redirectTo = c.Value
		http.SetCookie(w, &http.Cookie{Name: "redirect_to", Value: "", Path: "/", MaxAge: -1})
	}
	http.Redirect(w, r, redirectTo, http.StatusFound)
}

func (h *Handler) handleCLILogin(w http.ResponseWriter, r *http.Request) {
	port := r.URL.Query().Get("port")
	if port == "" {
		http.Error(w, "missing port parameter", http.StatusBadRequest)
		return
	}
	portNum, err := strconv.Atoi(port)
	if err != nil || portNum < 1 || portNum > 65535 {
		http.Error(w, "invalid port", http.StatusBadRequest)
		return
	}
	state := auth.GenerateState() + ":" + port
	http.SetCookie(w, &http.Cookie{
		Name:     "oauth_state",
		Value:    state,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
	url := h.OAuthConfig.AuthCodeURL(state)
	http.Redirect(w, r, url, http.StatusFound)
}

func (h *Handler) handleTokenExchange(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	var req struct {
		Code string `json:"code"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		if isMaxBytesError(err) {
			http.Error(w, "request body too large", http.StatusRequestEntityTooLarge)
			return
		}
		http.Error(w, "missing code", http.StatusBadRequest)
		return
	}
	if req.Code == "" {
		http.Error(w, "missing code", http.StatusBadRequest)
		return
	}

	token, err := h.OAuthConfig.Exchange(r, req.Code)
	if err != nil {
		http.Error(w, "oauth exchange failed", http.StatusInternalServerError)
		return
	}

	name, email, err := h.OAuthConfig.GetUserInfo(token)
	if err != nil {
		http.Error(w, "failed to get user info", http.StatusInternalServerError)
		return
	}

	apiToken := auth.GenerateAPIToken()
	if err := h.DB.CreateToken(apiToken, name, email); err != nil {
		http.Error(w, "failed to create token", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"token": apiToken,
		"name":  name,
		"email": email,
	})
}

func (h *Handler) handleLogout(w http.ResponseWriter, r *http.Request) {
	if cookie, err := r.Cookie("session"); err == nil && cookie.Value != "" {
		if u, err := auth.VerifySession(h.Auth.SessionSecret, cookie.Value); err == nil && u.SessionID != "" {
			h.DB.DeleteSession(u.SessionID)
		}
	}
	auth.ClearSessionCookie(w)
	http.Redirect(w, r, "/login", http.StatusFound)
}
