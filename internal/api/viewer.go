package api

import (
	"database/sql"
	"html/template"
	"net/http"
	"sort"

	"github.com/ab/design-reviewer/internal/auth"
)

func (h *Handler) handleViewer(w http.ResponseWriter, r *http.Request) {
	projectID := r.PathValue("id")

	project, err := h.DB.GetProject(projectID)
	if err == sql.ErrNoRows {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		serverError(w, "database error", err)
		return
	}

	var version *struct {
		ID         string
		VersionNum int
	}

	if vID := r.URL.Query().Get("version"); vID != "" {
		v, err := h.DB.GetVersion(vID)
		if err == sql.ErrNoRows {
			http.NotFound(w, r)
			return
		}
		if err != nil {
			serverError(w, "database error", err)
			return
		}
		version = &struct {
			ID         string
			VersionNum int
		}{v.ID, v.VersionNum}
	} else {
		v, err := h.DB.GetLatestVersion(projectID)
		if err == sql.ErrNoRows {
			http.NotFound(w, r)
			return
		}
		if err != nil {
			serverError(w, "database error", err)
			return
		}
		version = &struct {
			ID         string
			VersionNum int
		}{v.ID, v.VersionNum}
	}

	pages, err := h.Storage.ListHTMLFiles(version.ID)
	if err != nil {
		serverError(w, "storage error", err)
		return
	}
	sort.Strings(pages)

	defaultPage := ""
	if len(pages) > 0 {
		defaultPage = pages[0]
		for _, p := range pages {
			if p == "index.html" {
				defaultPage = "index.html"
				break
			}
		}
	}

	tmpl, err := template.ParseFiles(h.TemplatesDir+"/layout.html", h.TemplatesDir+"/viewer.html")
	if err != nil {
		serverError(w, "template error", err)
		return
	}

	data := struct {
		ProjectName string
		ProjectID   string
		Status      string
		StatusLabel string
		VersionID   string
		VersionNum  int
		Pages       []string
		DefaultPage string
		UserName    string
		IsOwner     bool
	}{
		ProjectName: project.Name,
		ProjectID:   project.ID,
		Status:      project.Status,
		StatusLabel: statusLabels[project.Status],
		VersionID:   version.ID,
		VersionNum:  version.VersionNum,
		Pages:       pages,
		DefaultPage: defaultPage,
		UserName:    func() string { n, _ := auth.GetUserFromContext(r.Context()); return n }(),
		IsOwner: func() bool {
			_, e := auth.GetUserFromContext(r.Context())
			return e != "" && project.OwnerEmail != nil && *project.OwnerEmail == e
		}(),
	}
	tmpl.Execute(w, data)
}
