package api

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"

	"github.com/ab/design-reviewer/internal/auth"
)

func (h *Handler) handleUpload(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 50<<20) // 50 MB

	file, _, err := r.FormFile("file")
	if err != nil {
		var maxErr *http.MaxBytesError
		if errors.As(err, &maxErr) {
			http.Error(w, "upload exceeds 50MB limit", http.StatusRequestEntityTooLarge)
			return
		}
		http.Error(w, "missing file field", http.StatusBadRequest)
		return
	}
	defer file.Close()

	name := r.FormValue("name")
	if name == "" {
		http.Error(w, "missing name field", http.StatusBadRequest)
		return
	}

	// Read zip data into memory for storage
	var buf bytes.Buffer
	if _, err := io.Copy(&buf, file); err != nil {
		serverError(w, "failed to read file", err)
		return
	}

	_, email := auth.GetUserFromContext(r.Context())

	// Get or create project
	project, err := h.DB.GetProjectByName(name)
	if err == sql.ErrNoRows {
		project, err = h.DB.CreateProject(name, email)
	} else if err == nil && email != "" {
		// Check access for existing project
		ok, aErr := h.DB.CanAccessProject(project.ID, email)
		if aErr != nil || !ok {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
	}
	if err != nil {
		serverError(w, "database error", err)
		return
	}

	// Create version
	version, err := h.DB.CreateVersion(project.ID, "")
	if err != nil {
		serverError(w, "failed to create version", err)
		return
	}

	// Save zip to storage
	if err := h.Storage.SaveUpload(version.ID, &buf); err != nil {
		http.Error(w, fmt.Sprintf("failed to save upload: %v", err), http.StatusBadRequest)
		return
	}

	// Update project's updated_at
	h.DB.UpdateProjectStatus(project.ID, project.Status)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"project_id":  project.ID,
		"version_id":  version.ID,
		"version_num": version.VersionNum,
		"url":         fmt.Sprintf("/projects/%s", project.ID),
	})
}
