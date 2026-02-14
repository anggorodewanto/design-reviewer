package auth

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

// Config holds Google OAuth configuration.
type Config struct {
	ClientID       string
	ClientSecret   string
	RedirectURL    string
	CLIRedirectURL string
	SessionSecret  string
	BaseURL        string
}

type contextKey string

const userKey contextKey = "user"

// User represents an authenticated user.
type User struct {
	Name      string `json:"name"`
	Email     string `json:"email"`
	ExpiresAt int64  `json:"exp,omitempty"`
	SessionID string `json:"sid,omitempty"`
}

// NewGoogleOAuthConfig creates an oauth2.Config for Google.
func NewGoogleOAuthConfig(cfg Config) *oauth2.Config {
	return &oauth2.Config{
		ClientID:     cfg.ClientID,
		ClientSecret: cfg.ClientSecret,
		RedirectURL:  cfg.RedirectURL,
		Scopes:       []string{"openid", "email", "profile"},
		Endpoint:     google.Endpoint,
	}
}

// GetUserInfo fetches user name and email from Google's userinfo API.
func GetUserInfo(token *oauth2.Token) (name, email string, err error) {
	client := oauth2.NewClient(context.Background(), oauth2.StaticTokenSource(token))
	resp, err := client.Get("https://www.googleapis.com/oauth2/v2/userinfo")
	if err != nil {
		return "", "", err
	}
	defer resp.Body.Close()
	var info struct {
		Name  string `json:"name"`
		Email string `json:"email"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&info); err != nil {
		return "", "", err
	}
	return info.Name, info.Email, nil
}

// GenerateAPIToken generates a random hex token for CLI auth.
func GenerateAPIToken() string {
	b := make([]byte, 32)
	io.ReadFull(rand.Reader, b)
	return hex.EncodeToString(b)
}

// GenerateState generates a random state string for OAuth CSRF protection.
func GenerateState() string {
	b := make([]byte, 16)
	io.ReadFull(rand.Reader, b)
	return hex.EncodeToString(b)
}

// GenerateSessionID generates a random session ID for server-side session tracking.
func GenerateSessionID() string {
	b := make([]byte, 32)
	io.ReadFull(rand.Reader, b)
	return hex.EncodeToString(b)
}

// SignSession creates a signed session cookie value from a User.
func SignSession(secret string, u User) (string, error) {
	u.ExpiresAt = time.Now().Add(24 * time.Hour).Unix()
	data, err := json.Marshal(u)
	if err != nil {
		return "", err
	}
	sig := hmacSign(secret, data)
	// format: base64(json) + "." + base64(sig)
	return base64.RawURLEncoding.EncodeToString(data) + "." + base64.RawURLEncoding.EncodeToString(sig), nil
}

// VerifySession verifies and decodes a signed session cookie value.
func VerifySession(secret, cookie string) (User, error) {
	parts := strings.SplitN(cookie, ".", 2)
	if len(parts) != 2 {
		return User{}, errors.New("invalid session format")
	}
	data, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return User{}, fmt.Errorf("decode data: %w", err)
	}
	sig, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return User{}, fmt.Errorf("decode sig: %w", err)
	}
	expected := hmacSign(secret, data)
	if !hmac.Equal(sig, expected) {
		return User{}, errors.New("invalid signature")
	}
	var u User
	if err := json.Unmarshal(data, &u); err != nil {
		return User{}, err
	}
	if u.ExpiresAt == 0 || time.Now().Unix() > u.ExpiresAt {
		return User{}, errors.New("session expired")
	}
	return u, nil
}

func hmacSign(secret string, data []byte) []byte {
	h := hmac.New(sha256.New, []byte(secret))
	h.Write(data)
	return h.Sum(nil)
}

// HmacSignExported is an exported wrapper for testing.
func HmacSignExported(secret string, data []byte) []byte {
	return hmacSign(secret, data)
}

// SetUserInContext adds user info to the context.
func SetUserInContext(ctx context.Context, name, email string) context.Context {
	return context.WithValue(ctx, userKey, User{Name: name, Email: email})
}

// GetUserFromContext retrieves user info from the context.
func GetUserFromContext(ctx context.Context) (name, email string) {
	u, ok := ctx.Value(userKey).(User)
	if !ok {
		return "", ""
	}
	return u.Name, u.Email
}

// SetSessionCookie sets the signed session cookie on the response.
func SetSessionCookie(w http.ResponseWriter, secret string, u User, secure bool) error {
	val, err := SignSession(secret, u)
	if err != nil {
		return err
	}
	http.SetCookie(w, &http.Cookie{
		Name:     "session",
		Value:    val,
		Path:     "/",
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteLaxMode,
	})
	return nil
}

// ClearSessionCookie removes the session cookie.
func ClearSessionCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     "session",
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
	})
}
