package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/ab/design-reviewer/internal/auth"
	"golang.org/x/oauth2"
)

// mockOAuth implements OAuthProvider for testing.
type mockOAuth struct {
	authURL  string
	token    *oauth2.Token
	exchErr  error
	userName string
	userEmail string
	infoErr  error
}

func (m *mockOAuth) AuthCodeURL(state string, opts ...oauth2.AuthCodeOption) string {
	return m.authURL + "?state=" + state
}

func (m *mockOAuth) Exchange(r *http.Request, code string) (*oauth2.Token, error) {
	return m.token, m.exchErr
}

func (m *mockOAuth) GetUserInfo(token *oauth2.Token) (name, email string, err error) {
	return m.userName, m.userEmail, m.infoErr
}

func setupAuthHandler(t *testing.T) *Handler {
	t.Helper()
	h := setupTestHandler(t)
	h.Auth = &auth.Config{
		ClientID:      "test-client-id",
		ClientSecret:  "test-secret",
		RedirectURL:   "http://localhost:8080/auth/google/callback",
		SessionSecret: "test-session-secret",
		BaseURL:       "http://localhost:8080",
	}
	h.OAuthConfig = &mockOAuth{
		authURL:   "https://accounts.google.com/o/oauth2/auth",
		token:     &oauth2.Token{AccessToken: "test-access-token"},
		userName:  "Test User",
		userEmail: "test@example.com",
	}
	return h
}

// helper to create a valid session cookie value
func testSessionCookie(t *testing.T, secret, name, email string) *http.Cookie {
	t.Helper()
	val, err := auth.SignSession(secret, auth.User{Name: name, Email: email})
	if err != nil {
		t.Fatal(err)
	}
	return &http.Cookie{Name: "session", Value: val}
}

func TestHandleGoogleLogin(t *testing.T) {
	h := setupAuthHandler(t)
	req := httptest.NewRequest("GET", "/auth/google/login", nil)
	w := httptest.NewRecorder()
	h.handleGoogleLogin(w, req)

	if w.Code != http.StatusFound {
		t.Fatalf("expected 302, got %d", w.Code)
	}
	loc := w.Header().Get("Location")
	if !strings.HasPrefix(loc, "https://accounts.google.com") {
		t.Errorf("expected redirect to Google, got %s", loc)
	}
	// Check state cookie was set
	var stateCookie bool
	for _, c := range w.Result().Cookies() {
		if c.Name == "oauth_state" && c.Value != "" {
			stateCookie = true
		}
	}
	if !stateCookie {
		t.Error("oauth_state cookie not set")
	}
}

func TestHandleGoogleCallbackSuccess(t *testing.T) {
	h := setupAuthHandler(t)
	state := "test-state-123"

	req := httptest.NewRequest("GET", "/auth/google/callback?code=authcode&state="+state, nil)
	req.AddCookie(&http.Cookie{Name: "oauth_state", Value: state})
	w := httptest.NewRecorder()
	h.handleGoogleCallback(w, req)

	if w.Code != http.StatusFound {
		t.Fatalf("expected 302, got %d: %s", w.Code, w.Body.String())
	}
	if loc := w.Header().Get("Location"); loc != "/" {
		t.Errorf("expected redirect to /, got %s", loc)
	}
	// Check session cookie was set
	var sessionSet bool
	for _, c := range w.Result().Cookies() {
		if c.Name == "session" && c.Value != "" {
			sessionSet = true
		}
	}
	if !sessionSet {
		t.Error("session cookie not set after callback")
	}
}

func TestHandleGoogleCallbackInvalidState(t *testing.T) {
	h := setupAuthHandler(t)
	req := httptest.NewRequest("GET", "/auth/google/callback?code=authcode&state=wrong", nil)
	req.AddCookie(&http.Cookie{Name: "oauth_state", Value: "correct"})
	w := httptest.NewRecorder()
	h.handleGoogleCallback(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestHandleGoogleCallbackNoStateCookie(t *testing.T) {
	h := setupAuthHandler(t)
	req := httptest.NewRequest("GET", "/auth/google/callback?code=authcode&state=test", nil)
	w := httptest.NewRecorder()
	h.handleGoogleCallback(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestHandleGoogleCallbackCLIFlow(t *testing.T) {
	h := setupAuthHandler(t)
	state := "randomstate:9876"

	req := httptest.NewRequest("GET", "/auth/google/callback?code=authcode&state="+state, nil)
	req.AddCookie(&http.Cookie{Name: "oauth_state", Value: state})
	w := httptest.NewRecorder()
	h.handleGoogleCallback(w, req)

	if w.Code != http.StatusFound {
		t.Fatalf("expected 302, got %d: %s", w.Code, w.Body.String())
	}
	loc := w.Header().Get("Location")
	if !strings.HasPrefix(loc, "http://localhost:9876/callback?token=") {
		t.Errorf("expected redirect to CLI localhost, got %s", loc)
	}
}

func TestHandleGoogleCallbackExchangeError(t *testing.T) {
	h := setupAuthHandler(t)
	h.OAuthConfig = &mockOAuth{exchErr: errDB}
	state := "test-state"

	req := httptest.NewRequest("GET", "/auth/google/callback?code=bad&state="+state, nil)
	req.AddCookie(&http.Cookie{Name: "oauth_state", Value: state})
	w := httptest.NewRecorder()
	h.handleGoogleCallback(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", w.Code)
	}
}

func TestHandleGoogleCallbackUserInfoError(t *testing.T) {
	h := setupAuthHandler(t)
	h.OAuthConfig = &mockOAuth{
		token:   &oauth2.Token{AccessToken: "test"},
		infoErr: errDB,
	}
	state := "test-state"

	req := httptest.NewRequest("GET", "/auth/google/callback?code=code&state="+state, nil)
	req.AddCookie(&http.Cookie{Name: "oauth_state", Value: state})
	w := httptest.NewRecorder()
	h.handleGoogleCallback(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", w.Code)
	}
}

func TestHandleCLILogin(t *testing.T) {
	h := setupAuthHandler(t)
	req := httptest.NewRequest("GET", "/auth/google/cli-login?port=9876", nil)
	w := httptest.NewRecorder()
	h.handleCLILogin(w, req)

	if w.Code != http.StatusFound {
		t.Fatalf("expected 302, got %d", w.Code)
	}
	// State should contain port
	for _, c := range w.Result().Cookies() {
		if c.Name == "oauth_state" {
			if !strings.HasSuffix(c.Value, ":9876") {
				t.Errorf("state should end with :9876, got %s", c.Value)
			}
		}
	}
}

func TestHandleCLILoginMissingPort(t *testing.T) {
	h := setupAuthHandler(t)
	req := httptest.NewRequest("GET", "/auth/google/cli-login", nil)
	w := httptest.NewRecorder()
	h.handleCLILogin(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestHandleTokenExchange(t *testing.T) {
	h := setupAuthHandler(t)
	body := `{"code":"auth-code"}`
	req := httptest.NewRequest("POST", "/api/auth/token", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.handleTokenExchange(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var result map[string]string
	json.NewDecoder(w.Body).Decode(&result)
	if result["token"] == "" {
		t.Error("missing token in response")
	}
	if result["name"] != "Test User" {
		t.Errorf("name = %q, want Test User", result["name"])
	}
	if result["email"] != "test@example.com" {
		t.Errorf("email = %q, want test@example.com", result["email"])
	}
}

func TestHandleTokenExchangeMissingCode(t *testing.T) {
	h := setupAuthHandler(t)
	req := httptest.NewRequest("POST", "/api/auth/token", strings.NewReader(`{}`))
	w := httptest.NewRecorder()
	h.handleTokenExchange(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestHandleTokenExchangeInvalidJSON(t *testing.T) {
	h := setupAuthHandler(t)
	req := httptest.NewRequest("POST", "/api/auth/token", strings.NewReader("not json"))
	w := httptest.NewRecorder()
	h.handleTokenExchange(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestHandleTokenExchangeExchangeError(t *testing.T) {
	h := setupAuthHandler(t)
	h.OAuthConfig = &mockOAuth{exchErr: errDB}
	req := httptest.NewRequest("POST", "/api/auth/token", strings.NewReader(`{"code":"bad"}`))
	w := httptest.NewRecorder()
	h.handleTokenExchange(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", w.Code)
	}
}

func TestHandleTokenExchangeUserInfoError(t *testing.T) {
	h := setupAuthHandler(t)
	h.OAuthConfig = &mockOAuth{
		token:   &oauth2.Token{AccessToken: "test"},
		infoErr: errDB,
	}
	req := httptest.NewRequest("POST", "/api/auth/token", strings.NewReader(`{"code":"code"}`))
	w := httptest.NewRecorder()
	h.handleTokenExchange(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", w.Code)
	}
}

func TestHandleLogout(t *testing.T) {
	h := setupAuthHandler(t)
	req := httptest.NewRequest("GET", "/auth/logout", nil)
	w := httptest.NewRecorder()
	h.handleLogout(w, req)

	if w.Code != http.StatusFound {
		t.Fatalf("expected 302, got %d", w.Code)
	}
	if loc := w.Header().Get("Location"); loc != "/login" {
		t.Errorf("expected redirect to /login, got %s", loc)
	}
	// Check session cookie is cleared
	for _, c := range w.Result().Cookies() {
		if c.Name == "session" && c.MaxAge != -1 {
			t.Error("session cookie should have MaxAge=-1")
		}
	}
}

func TestHandleLoginPage(t *testing.T) {
	h := setupAuthHandler(t)
	req := httptest.NewRequest("GET", "/login", nil)
	w := httptest.NewRecorder()
	h.handleLoginPage(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	body := w.Body.String()
	if !strings.Contains(body, "Login with Google") {
		t.Error("login page missing 'Login with Google' button")
	}
}

// --- Middleware Tests ---

func TestWebMiddlewareRedirectsWithoutSession(t *testing.T) {
	h := setupAuthHandler(t)
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	})
	handler := h.webMiddleware(inner)

	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusFound {
		t.Fatalf("expected 302, got %d", w.Code)
	}
	if loc := w.Header().Get("Location"); loc != "/login" {
		t.Errorf("expected redirect to /login, got %s", loc)
	}
}

func TestWebMiddlewareRedirectsWithInvalidSession(t *testing.T) {
	h := setupAuthHandler(t)
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	})
	handler := h.webMiddleware(inner)

	req := httptest.NewRequest("GET", "/", nil)
	req.AddCookie(&http.Cookie{Name: "session", Value: "invalid"})
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusFound {
		t.Fatalf("expected 302, got %d", w.Code)
	}
}

func TestWebMiddlewarePassesWithValidSession(t *testing.T) {
	h := setupAuthHandler(t)
	var gotName, gotEmail string
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotName, gotEmail = auth.GetUserFromContext(r.Context())
		w.WriteHeader(200)
	})
	handler := h.webMiddleware(inner)

	req := httptest.NewRequest("GET", "/", nil)
	req.AddCookie(testSessionCookie(t, h.Auth.SessionSecret, "Alice", "alice@test.com"))
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if gotName != "Alice" || gotEmail != "alice@test.com" {
		t.Errorf("got name=%q email=%q", gotName, gotEmail)
	}
}

func TestAPIMiddlewareReturns401WithoutAuth(t *testing.T) {
	h := setupAuthHandler(t)
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	})
	handler := h.apiMiddleware(inner)

	req := httptest.NewRequest("GET", "/api/projects", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
	var result map[string]string
	json.NewDecoder(w.Body).Decode(&result)
	if result["error"] != "unauthorized" {
		t.Errorf("expected error=unauthorized, got %v", result["error"])
	}
}

func TestAPIMiddlewareAcceptsBearerToken(t *testing.T) {
	h := setupAuthHandler(t)
	// Create a token in DB
	h.DB.CreateToken("test-api-token", "Bob", "bob@test.com")

	var gotName, gotEmail string
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotName, gotEmail = auth.GetUserFromContext(r.Context())
		w.WriteHeader(200)
	})
	handler := h.apiMiddleware(inner)

	req := httptest.NewRequest("GET", "/api/projects", nil)
	req.Header.Set("Authorization", "Bearer test-api-token")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if gotName != "Bob" || gotEmail != "bob@test.com" {
		t.Errorf("got name=%q email=%q", gotName, gotEmail)
	}
}

func TestAPIMiddlewareAcceptsSessionCookie(t *testing.T) {
	h := setupAuthHandler(t)
	var gotName string
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotName, _ = auth.GetUserFromContext(r.Context())
		w.WriteHeader(200)
	})
	handler := h.apiMiddleware(inner)

	req := httptest.NewRequest("GET", "/api/projects", nil)
	req.AddCookie(testSessionCookie(t, h.Auth.SessionSecret, "Carol", "carol@test.com"))
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if gotName != "Carol" {
		t.Errorf("got name=%q, want Carol", gotName)
	}
}

func TestAPIMiddlewareRejectsInvalidToken(t *testing.T) {
	h := setupAuthHandler(t)
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	})
	handler := h.apiMiddleware(inner)

	req := httptest.NewRequest("GET", "/api/projects", nil)
	req.Header.Set("Authorization", "Bearer invalid-token")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", w.Code)
	}
}

// --- Auth context in comment/reply handlers ---

func TestCreateCommentUsesAuthContext(t *testing.T) {
	h := setupAuthHandler(t)
	_, vid := seedProject(t, h, map[string]string{"index.html": "x"})

	body := `{"page":"index.html","x_percent":10,"y_percent":20,"author_name":"Ignored","author_email":"ignored@test.com","body":"test comment"}`
	req := httptest.NewRequest("POST", "/api/versions/"+vid+"/comments", strings.NewReader(body))
	req.SetPathValue("id", vid)
	// Set auth context
	ctx := auth.SetUserInContext(req.Context(), "AuthUser", "auth@test.com")
	req = req.WithContext(ctx)
	w := httptest.NewRecorder()
	h.handleCreateComment(w, req)

	if w.Code != 201 {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}
	var result commentJSON
	json.NewDecoder(w.Body).Decode(&result)
	if result.AuthorName != "AuthUser" {
		t.Errorf("author_name = %q, want AuthUser (from context)", result.AuthorName)
	}
	if result.AuthorEmail != "auth@test.com" {
		t.Errorf("author_email = %q, want auth@test.com", result.AuthorEmail)
	}
}

func TestCreateReplyUsesAuthContext(t *testing.T) {
	h := setupAuthHandler(t)
	_, vid := seedProject(t, h, map[string]string{"index.html": "x"})

	// Create a comment first
	c, _ := h.DB.CreateComment(vid, "index.html", 10, 20, "A", "a@t.com", "hello")

	body := `{"author_name":"Ignored","author_email":"ignored@test.com","body":"reply"}`
	req := httptest.NewRequest("POST", "/api/comments/"+c.ID+"/replies", strings.NewReader(body))
	req.SetPathValue("id", c.ID)
	ctx := auth.SetUserInContext(req.Context(), "ReplyUser", "reply@test.com")
	req = req.WithContext(ctx)
	w := httptest.NewRecorder()
	h.handleCreateReply(w, req)

	if w.Code != 201 {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}
	var result replyJSON
	json.NewDecoder(w.Body).Decode(&result)
	if result.AuthorName != "ReplyUser" {
		t.Errorf("author_name = %q, want ReplyUser", result.AuthorName)
	}
}

// --- Additional error path tests ---

func TestHandleLoginPageTemplateMissing(t *testing.T) {
	h := setupAuthHandler(t)
	h.TemplatesDir = "/nonexistent"
	req := httptest.NewRequest("GET", "/login", nil)
	w := httptest.NewRecorder()
	h.handleLoginPage(w, req)
	if w.Code != 500 {
		t.Errorf("expected 500, got %d", w.Code)
	}
}

func TestHandleGoogleCallbackCLIFlowCreateTokenError(t *testing.T) {
	h := setupAuthHandler(t)
	m := &mockDB{DataStore: h.DB, createTokenErr: errDB}
	h.DB = m
	state := "randomstate:9876"

	req := httptest.NewRequest("GET", "/auth/google/callback?code=authcode&state="+state, nil)
	req.AddCookie(&http.Cookie{Name: "oauth_state", Value: state})
	w := httptest.NewRecorder()
	h.handleGoogleCallback(w, req)
	if w.Code != 500 {
		t.Errorf("expected 500, got %d", w.Code)
	}
}

func TestHandleTokenExchangeCreateTokenError(t *testing.T) {
	h := setupAuthHandler(t)
	m := &mockDB{DataStore: h.DB, createTokenErr: errDB}
	h.DB = m

	req := httptest.NewRequest("POST", "/api/auth/token", strings.NewReader(`{"code":"code"}`))
	w := httptest.NewRecorder()
	h.handleTokenExchange(w, req)
	if w.Code != 500 {
		t.Errorf("expected 500, got %d", w.Code)
	}
}

func TestRegisterRoutesWithAuth(t *testing.T) {
	h := setupAuthHandler(t)
	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	routes := []struct {
		method string
		path   string
	}{
		{"GET", "/"},
		{"GET", "/login"},
		{"GET", "/auth/google/login"},
		{"GET", "/auth/google/callback"},
		{"GET", "/auth/google/cli-login"},
		{"POST", "/api/auth/token"},
		{"GET", "/auth/logout"},
		{"GET", "/api/projects"},
		{"POST", "/api/upload"},
		{"GET", "/projects/test-id"},
	}
	for _, r := range routes {
		req := httptest.NewRequest(r.method, r.path, nil)
		_, pattern := mux.Handler(req)
		if pattern == "" {
			t.Errorf("no route matched %s %s", r.method, r.path)
		}
	}
}

func TestWebMiddlewareEmptyCookieValue(t *testing.T) {
	h := setupAuthHandler(t)
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	})
	handler := h.webMiddleware(inner)

	req := httptest.NewRequest("GET", "/", nil)
	req.AddCookie(&http.Cookie{Name: "session", Value: ""})
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	if w.Code != http.StatusFound {
		t.Errorf("expected 302, got %d", w.Code)
	}
}

func TestAPIMiddlewareInvalidBearerThenValidSession(t *testing.T) {
	h := setupAuthHandler(t)
	var gotName string
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotName, _ = auth.GetUserFromContext(r.Context())
		w.WriteHeader(200)
	})
	handler := h.apiMiddleware(inner)

	req := httptest.NewRequest("GET", "/api/projects", nil)
	req.Header.Set("Authorization", "Bearer invalid-token")
	req.AddCookie(testSessionCookie(t, h.Auth.SessionSecret, "Fallback", "fb@t.com"))
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if gotName != "Fallback" {
		t.Errorf("expected Fallback, got %q", gotName)
	}
}
