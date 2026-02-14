package api

import (
	"net/http"
	"os"

	"github.com/ab/design-reviewer/internal/auth"
	"github.com/ab/design-reviewer/internal/db"
	"github.com/ab/design-reviewer/internal/storage"
)

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

// DataStore abstracts database operations for testability.
type DataStore interface {
	CreateProject(name, ownerEmail string) (*db.Project, error)
	GetProject(id string) (*db.Project, error)
	GetProjectByName(name string) (*db.Project, error)
	ListProjectsWithVersionCount() ([]db.ProjectWithVersionCount, error)
	ListProjectsWithVersionCountForUser(email string) ([]db.ProjectWithVersionCount, error)
	UpdateProjectStatus(id, status string) error
	CreateVersion(projectID, storagePath string) (*db.Version, error)
	GetVersion(id string) (*db.Version, error)
	GetLatestVersion(projectID string) (*db.Version, error)
	ListVersions(projectID string) ([]db.Version, error)
	CreateComment(versionID, page string, xPct, yPct float64, authorName, authorEmail, body string) (*db.Comment, error)
	GetCommentsForVersion(versionID string) ([]db.Comment, error)
	GetUnresolvedCommentsUpTo(versionID string) ([]db.Comment, error)
	GetComment(id string) (*db.Comment, error)
	ToggleResolve(commentID string) (bool, error)
	MoveComment(id string, x, y float64) error
	CreateReply(commentID, authorName, authorEmail, body string) (*db.Reply, error)
	GetReplies(commentID string) ([]db.Reply, error)
	CreateToken(token, userName, userEmail string) error
	GetUserByToken(token string) (name, email string, err error)
	CanAccessProject(projectID, email string) (bool, error)
	GetProjectOwner(projectID string) (string, error)
	CreateInvite(projectID, createdBy string) (*db.ProjectInvite, error)
	GetInviteByToken(token string) (*db.ProjectInvite, error)
	DeleteInvite(id string) error
	AddMember(projectID, email string) error
	ListMembers(projectID string) ([]db.ProjectMember, error)
	RemoveMember(projectID, email string) error
	CreateSession(id, userName, userEmail string) error
	GetSession(id string) (string, string, error)
	DeleteSession(id string) error
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
	mux.Handle("GET /static/", http.StripPrefix("/static/", http.FileServer(noDirFS{http.Dir(h.StaticDir)})))

	// Web routes (web middleware)
	webHome := http.HandlerFunc(h.handleHome)
	webViewer := http.HandlerFunc(h.handleViewer)
	if h.Auth != nil {
		mux.Handle("GET /{$}", h.webMiddleware(webHome))
		mux.Handle("GET /projects/{id}", h.webMiddleware(h.projectAccess(webViewer)))
		mux.Handle("GET /invite/{token}", h.webMiddleware(http.HandlerFunc(h.handleAcceptInvite)))
	} else {
		mux.Handle("GET /{$}", webHome)
		mux.Handle("GET /projects/{id}", webViewer)
	}

	// Design files
	designHandler := http.HandlerFunc(h.handleDesignFile)
	if h.Auth != nil {
		mux.Handle("GET /designs/{version_id}/{filepath...}", h.webMiddleware(h.versionAccess(designHandler)))
	} else {
		mux.Handle("GET /designs/{version_id}/{filepath...}", designHandler)
	}

	// API routes (API middleware)
	apiUpload := http.HandlerFunc(h.handleUpload)
	apiListProjects := http.HandlerFunc(h.handleListProjects)
	apiListVersions := http.HandlerFunc(h.handleListVersions)
	apiUpdateStatus := http.HandlerFunc(h.handleUpdateStatus)
	apiGetComments := http.HandlerFunc(h.handleGetComments)
	apiCreateComment := http.HandlerFunc(h.handleCreateComment)
	apiCreateReply := http.HandlerFunc(h.handleCreateReply)
	apiToggleResolve := http.HandlerFunc(h.handleToggleResolve)
	apiMoveComment := http.HandlerFunc(h.handleMoveComment)

	// Sharing API handlers
	apiCreateInvite := http.HandlerFunc(h.handleCreateInvite)
	apiDeleteInvite := http.HandlerFunc(h.handleDeleteInvite)
	apiListMembers := http.HandlerFunc(h.handleListMembers)
	apiRemoveMember := http.HandlerFunc(h.handleRemoveMember)

	if h.Auth != nil {
		mux.Handle("POST /api/upload", h.apiMiddleware(apiUpload))
		mux.Handle("GET /api/projects", h.apiMiddleware(apiListProjects))
		mux.Handle("GET /api/projects/{id}/versions", h.apiMiddleware(h.projectAccess(apiListVersions)))
		mux.Handle("PATCH /api/projects/{id}/status", h.apiMiddleware(h.ownerOnly(apiUpdateStatus)))
		mux.Handle("GET /api/versions/{id}/comments", h.apiMiddleware(h.versionAccess(apiGetComments)))
		mux.Handle("POST /api/versions/{id}/comments", h.apiMiddleware(h.versionAccess(apiCreateComment)))
		mux.Handle("POST /api/comments/{id}/replies", h.apiMiddleware(h.commentAccess(apiCreateReply)))
		mux.Handle("PATCH /api/comments/{id}/resolve", h.apiMiddleware(h.commentAccess(apiToggleResolve)))
		mux.Handle("PATCH /api/comments/{id}/move", h.apiMiddleware(h.commentAccess(apiMoveComment)))
		// Sharing routes
		mux.Handle("POST /api/projects/{id}/invites", h.apiMiddleware(h.ownerOnly(apiCreateInvite)))
		mux.Handle("DELETE /api/projects/{id}/invites/{inviteID}", h.apiMiddleware(h.ownerOnly(apiDeleteInvite)))
		mux.Handle("GET /api/projects/{id}/members", h.apiMiddleware(h.projectAccess(apiListMembers)))
		mux.Handle("DELETE /api/projects/{id}/members/{email}", h.apiMiddleware(h.ownerOnly(apiRemoveMember)))
	} else {
		mux.Handle("POST /api/upload", apiUpload)
		mux.Handle("GET /api/projects", apiListProjects)
		mux.Handle("GET /api/projects/{id}/versions", apiListVersions)
		mux.Handle("PATCH /api/projects/{id}/status", apiUpdateStatus)
		mux.Handle("GET /api/versions/{id}/comments", apiGetComments)
		mux.Handle("POST /api/versions/{id}/comments", apiCreateComment)
		mux.Handle("POST /api/comments/{id}/replies", apiCreateReply)
		mux.Handle("PATCH /api/comments/{id}/resolve", apiToggleResolve)
		mux.Handle("PATCH /api/comments/{id}/move", apiMoveComment)
		mux.Handle("POST /api/projects/{id}/invites", apiCreateInvite)
		mux.Handle("DELETE /api/projects/{id}/invites/{inviteID}", apiDeleteInvite)
		mux.Handle("GET /api/projects/{id}/members", apiListMembers)
		mux.Handle("DELETE /api/projects/{id}/members/{email}", apiRemoveMember)
	}
}
