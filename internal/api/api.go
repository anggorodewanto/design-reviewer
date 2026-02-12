package api

import (
	"net/http"

	"github.com/ab/design-reviewer/internal/db"
	"github.com/ab/design-reviewer/internal/storage"
)

type Handler struct {
	DB      *db.DB
	Storage *storage.Storage
}

func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/upload", h.handleUpload)
	mux.HandleFunc("GET /designs/{version_id}/{filepath...}", h.handleDesignFile)
}
