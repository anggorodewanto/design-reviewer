package api

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"html/template"
	"net/http"
	"strings"
	"time"

	"github.com/ab/design-reviewer/internal/auth"
	"github.com/ab/design-reviewer/internal/db"
)

var statusLabels = map[string]string{
	"draft":      "Draft",
	"in_review":  "In Review",
	"approved":   "Approved",
	"handed_off": "Handed Off",
}

type projectView struct {
	ID           string
	Name         string
	Status       string
	StatusLabel  string
	VersionCount int
	TimeAgo      string
	UpdatedAt    time.Time
}

func toProjectViews(projects []db.ProjectWithVersionCount) []projectView {
	views := make([]projectView, len(projects))
	for i, p := range projects {
		views[i] = projectView{
			ID:           p.ID,
			Name:         p.Name,
			Status:       p.Status,
			StatusLabel:  statusLabels[p.Status],
			VersionCount: p.VersionCount,
			TimeAgo:      relativeTime(p.UpdatedAt),
			UpdatedAt:    p.UpdatedAt,
		}
	}
	return views
}

func relativeTime(t time.Time) string {
	d := time.Since(t)
	switch {
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		m := int(d.Minutes())
		if m == 1 {
			return "1 minute ago"
		}
		return fmt.Sprintf("%d minutes ago", m)
	case d < 24*time.Hour:
		h := int(d.Hours())
		if h == 1 {
			return "1 hour ago"
		}
		return fmt.Sprintf("%d hours ago", h)
	default:
		days := int(d.Hours() / 24)
		if days == 1 {
			return "1 day ago"
		}
		return fmt.Sprintf("%d days ago", days)
	}
}

func (h *Handler) handleListProjects(w http.ResponseWriter, r *http.Request) {
	projects, err := h.DB.ListProjectsWithVersionCount()
	if err != nil {
		http.Error(w, "database error", http.StatusInternalServerError)
		return
	}
	if projects == nil {
		projects = []db.ProjectWithVersionCount{}
	}

	type apiProject struct {
		ID           string `json:"id"`
		Name         string `json:"name"`
		Status       string `json:"status"`
		VersionCount int    `json:"version_count"`
		UpdatedAt    string `json:"updated_at"`
	}
	out := make([]apiProject, len(projects))
	for i, p := range projects {
		out[i] = apiProject{
			ID:           p.ID,
			Name:         p.Name,
			Status:       p.Status,
			VersionCount: p.VersionCount,
			UpdatedAt:    p.UpdatedAt.Format(time.RFC3339),
		}
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(out)
}

func (h *Handler) handleUpdateStatus(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var req struct {
		Status string `json:"status"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}
	if err := h.DB.UpdateProjectStatus(id, req.Status); err != nil {
		if err == sql.ErrNoRows {
			http.NotFound(w, r)
			return
		}
		if strings.HasPrefix(err.Error(), "invalid status") {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		http.Error(w, "database error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"id": id, "status": req.Status})
}

func (h *Handler) handleHome(w http.ResponseWriter, r *http.Request) {
	projects, err := h.DB.ListProjectsWithVersionCount()
	if err != nil {
		http.Error(w, "database error", http.StatusInternalServerError)
		return
	}

	tmpl, err := template.ParseFiles(h.TemplatesDir+"/layout.html", h.TemplatesDir+"/home.html")
	if err != nil {
		http.Error(w, "template error", http.StatusInternalServerError)
		return
	}

	data := struct {
		Projects []projectView
		UserName string
	}{
		Projects: toProjectViews(projects),
		UserName: func() string { n, _ := auth.GetUserFromContext(r.Context()); return n }(),
	}
	tmpl.Execute(w, data)
}
