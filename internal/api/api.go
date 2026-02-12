package api

import (
	"net/http"

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
}

type Handler struct {
	DB           DataStore
	Storage      *storage.Storage
	TemplatesDir string
	StaticDir    string
}

func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /{$}", h.handleHome)
	mux.HandleFunc("GET /api/projects", h.handleListProjects)
	mux.Handle("GET /static/", http.StripPrefix("/static/", http.FileServer(http.Dir(h.StaticDir))))
	mux.HandleFunc("POST /api/upload", h.handleUpload)
	mux.HandleFunc("GET /designs/{version_id}/{filepath...}", h.handleDesignFile)
	mux.HandleFunc("GET /projects/{id}", h.handleViewer)
	// Phase 6: Version History
	mux.HandleFunc("GET /api/projects/{id}/versions", h.handleListVersions)
	// Phase 5: Annotations
	mux.HandleFunc("GET /api/versions/{id}/comments", h.handleGetComments)
	mux.HandleFunc("POST /api/versions/{id}/comments", h.handleCreateComment)
	mux.HandleFunc("POST /api/comments/{id}/replies", h.handleCreateReply)
	mux.HandleFunc("PATCH /api/comments/{id}/resolve", h.handleToggleResolve)
}
