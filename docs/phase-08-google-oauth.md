# Phase 8: Google OAuth Authentication

## Goal
Add Google OAuth SSO for both the web app and CLI auth endpoints. Protect all routes behind authentication.

## Prerequisites
Phases 1-7 complete — all features work without auth. This phase wraps everything in authentication.

## What to Build

### 1. Auth Package (`internal/auth/auth.go`)

Configuration struct:
```go
type Config struct {
    ClientID     string // Google OAuth client ID
    ClientSecret string // Google OAuth client secret
    RedirectURL  string // e.g., "http://localhost:8080/auth/google/callback"
    CLIRedirectURL string // e.g., "http://localhost:8080/auth/google/cli-callback"
}
```

Dependencies to add:
- `golang.org/x/oauth2`
- `golang.org/x/oauth2/google`

Functions:
- `NewGoogleOAuthConfig(cfg Config) *oauth2.Config`
- `GetUserInfo(token *oauth2.Token) (name, email string, err error)` — calls Google's userinfo API
- `GenerateAPIToken() string` — generates a random token for CLI auth
- Session management using signed cookies (use a secret key from env var)

### 2. Web OAuth Flow (`internal/api/auth.go`)

**`GET /auth/google/login`**
- Generates OAuth state parameter (random string, stored in cookie)
- Redirects to Google's OAuth consent screen
- Scopes: `openid`, `email`, `profile`

**`GET /auth/google/callback`**
- Validates state parameter
- Exchanges code for token
- Fetches user info (name, email)
- Creates a session cookie (HTTP-only, secure, signed)
- Redirects to `/`

**Session cookie contents:** user's name and email, signed with server secret. Use a simple approach — encode as JSON, sign with HMAC-SHA256.

### 3. CLI OAuth Flow (`internal/api/auth.go`)

**`GET /auth/google/cli-login?port=9876`**
- Same as web login but stores the CLI's localhost port in the OAuth state
- After Google auth, redirects to `http://localhost:{port}/callback?token={api_token}`

**`POST /api/auth/token`**
- Exchanges an OAuth authorization code for an API token
- Stores the token → user mapping in a `tokens` table (or in-memory map)
- Returns: `{"token": "...", "name": "...", "email": "..."}`

### 4. Token Storage

Add a `tokens` table to the DB schema:
```sql
CREATE TABLE IF NOT EXISTS tokens (
    token TEXT PRIMARY KEY,
    user_name TEXT NOT NULL,
    user_email TEXT NOT NULL,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
```

DB methods:
- `CreateToken(token, userName, userEmail string) error`
- `GetUserByToken(token string) (name, email string, err error)`

### 5. Auth Middleware (`internal/api/middleware.go`)

Two middleware functions:

**Web middleware** — for browser routes (`/`, `/projects/*`):
- Checks for session cookie
- If missing/invalid, redirects to `/auth/google/login`
- If valid, adds user info to request context

**API middleware** — for API routes (`/api/*`):
- Checks for `Authorization: Bearer {token}` header
- If missing/invalid, returns 401 JSON error
- If valid, adds user info to request context
- Also accepts session cookie (so web app JS can call APIs)

**Context helpers:**
```go
func GetUserFromContext(ctx context.Context) (name, email string)
func SetUserInContext(ctx context.Context, name, email string) context.Context
```

### 6. Wire Auth Into Existing Handlers

- Comment creation: get author_name and author_email from context instead of request body
- Reply creation: same
- All API handlers: user info comes from auth context

### 7. Server Configuration

Environment variables:
- `GOOGLE_CLIENT_ID`
- `GOOGLE_CLIENT_SECRET`
- `SESSION_SECRET` — for signing cookies
- `BASE_URL` — e.g., `http://localhost:8080` (used to construct redirect URLs)

### 8. Login/Logout UI

- Add a "Login with Google" button on an unauthenticated landing page
- Add user name display + "Logout" link in the page header (layout.html)
- `GET /auth/logout` — clears session cookie, redirects to login

### 9. Register Routes

Add to `Handler.RegisterRoutes()`:
- `GET /auth/google/login`
- `GET /auth/google/callback`
- `GET /auth/google/cli-login`
- `POST /api/auth/token`
- `GET /auth/logout`
- Apply web middleware to `/` and `/projects/*`
- Apply API middleware to `/api/*`

## Verification
- Unauthenticated visit to `/` redirects to Google login
- After Google auth, redirected back to `/` with session
- User name shown in header
- Logout clears session
- API calls without token return 401
- API calls with valid Bearer token succeed
- Comment/reply author info comes from auth, not request body
