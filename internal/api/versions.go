package api

import (
	"encoding/json"
	"net/http"
	"sort"
	"time"
)

func (h *Handler) handleListVersions(w http.ResponseWriter, r *http.Request) {
	projectID := r.PathValue("id")

	versions, err := h.DB.ListVersions(projectID)
	if err != nil {
		serverError(w, "database error", err)
		return
	}

	type versionJSON struct {
		ID         string   `json:"id"`
		VersionNum int      `json:"version_num"`
		CreatedAt  string   `json:"created_at"`
		Pages      []string `json:"pages"`
	}

	out := make([]versionJSON, len(versions))
	for i, v := range versions {
		pages, _ := h.Storage.ListHTMLFiles(v.ID)
		sort.Strings(pages)
		if pages == nil {
			pages = []string{}
		}
		out[i] = versionJSON{
			ID:         v.ID,
			VersionNum: v.VersionNum,
			CreatedAt:  v.CreatedAt.Format(time.RFC3339),
			Pages:      pages,
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(out)
}
