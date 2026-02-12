package api

import (
	"net/http"

	"github.com/ab/design-reviewer/internal/auth"
	"github.com/ab/design-reviewer/internal/db"
	"github.com/ab/design-reviewer/internal/storage"
)

// DataStore abstracts database operations for testability.
type DataStore interface {
	CreateProject(name string) (*db.Project, error)
	GetProject(id string) (*db.Project, error)
	GetProjectByName(name string) (*db.Project, error)
	ListProjectsWithVersionCount() ([]db.ProjectWithVersionCount, error)
	UpdateProjectStatus(id, status string) error
	CreateVersion(projectID, storagePath string) (*db.Version, error)
	GetVersion(id string) (*db.Version, error)
	GetLatestVersion(projectID string) (*db.Version, error)
	ListVersions(projectID string) ([]db.Version, error)
	CreateComment(versionID, page string, xPct, yPct float64, authorName, authorEmail, body string) (*db.Comment, error)
	GetCommentsForVersion(versionID string) ([]db.Comment, error)
	GetUnresolvedCommentsUpTo(versionID string) ([]db.Comment, error)
	ToggleResolve(commentID string) (bool, error)
	CreateReply(commentID, authorName, authorEmail, body string) (*db.Reply, error)
	GetReplies(commentID string) ([]db.Reply, error)
	CreateToken(token, userName, userEmail string) error
	GetUserByToken(token string) (name, email string, err error)
}

type Handler struct {
	DB           DataStore
	Storage      *storage.Storage
	TemplatesDir string
	StaticDir    string
	Auth         *auth.Config // nil = auth disabled
	OAuthConfig  OAuthProvider
}

func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	// Auth routes (no middleware)
	if h.Auth != nil {
		mux.HandleFunc("GET /auth/google/login", h.handleGoogleLogin)
		mux.HandleFunc("GET /auth/google/callback", h.handleGoogleCallback)
		mux.HandleFunc("GET /auth/google/cli-login", h.handleCLILogin)
		mux.HandleFunc("POST /api/auth/token", h.handleTokenExchange)
		mux.HandleFunc("GET /auth/logout", h.handleLogout)
		mux.HandleFunc("GET /login", h.handleLoginPage)
	}

	// Static files (no auth)
	mux.Handle("GET /static/", http.StripPrefix("/static/", http.FileServer(http.Dir(h.StaticDir))))

	// Web routes (web middleware)
	webHome := http.HandlerFunc(h.handleHome)
	webViewer := http.HandlerFunc(h.handleViewer)
	if h.Auth != nil {
		mux.Handle("GET /{$}", h.webMiddleware(webHome))
		mux.Handle("GET /projects/{id}", h.webMiddleware(webViewer))
	} else {
		mux.Handle("GET /{$}", webHome)
		mux.Handle("GET /projects/{id}", webViewer)
	}

	// Design files (no auth - served in iframe)
	mux.HandleFunc("GET /designs/{version_id}/{filepath...}", h.handleDesignFile)

	// API routes (API middleware)
	apiUpload := http.HandlerFunc(h.handleUpload)
	apiListProjects := http.HandlerFunc(h.handleListProjects)
	apiListVersions := http.HandlerFunc(h.handleListVersions)
	apiUpdateStatus := http.HandlerFunc(h.handleUpdateStatus)
	apiGetComments := http.HandlerFunc(h.handleGetComments)
	apiCreateComment := http.HandlerFunc(h.handleCreateComment)
	apiCreateReply := http.HandlerFunc(h.handleCreateReply)
	apiToggleResolve := http.HandlerFunc(h.handleToggleResolve)

	if h.Auth != nil {
		mux.Handle("POST /api/upload", h.apiMiddleware(apiUpload))
		mux.Handle("GET /api/projects", h.apiMiddleware(apiListProjects))
		mux.Handle("GET /api/projects/{id}/versions", h.apiMiddleware(apiListVersions))
		mux.Handle("PATCH /api/projects/{id}/status", h.apiMiddleware(apiUpdateStatus))
		mux.Handle("GET /api/versions/{id}/comments", h.apiMiddleware(apiGetComments))
		mux.Handle("POST /api/versions/{id}/comments", h.apiMiddleware(apiCreateComment))
		mux.Handle("POST /api/comments/{id}/replies", h.apiMiddleware(apiCreateReply))
		mux.Handle("PATCH /api/comments/{id}/resolve", h.apiMiddleware(apiToggleResolve))
	} else {
		mux.Handle("POST /api/upload", apiUpload)
		mux.Handle("GET /api/projects", apiListProjects)
		mux.Handle("GET /api/projects/{id}/versions", apiListVersions)
		mux.Handle("PATCH /api/projects/{id}/status", apiUpdateStatus)
		mux.Handle("GET /api/versions/{id}/comments", apiGetComments)
		mux.Handle("POST /api/versions/{id}/comments", apiCreateComment)
		mux.Handle("POST /api/comments/{id}/replies", apiCreateReply)
		mux.Handle("PATCH /api/comments/{id}/resolve", apiToggleResolve)
	}
}
