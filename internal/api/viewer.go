package api

import (
	"database/sql"
	"html/template"
	"net/http"
	"sort"
)

func (h *Handler) handleViewer(w http.ResponseWriter, r *http.Request) {
	projectID := r.PathValue("id")

	project, err := h.DB.GetProject(projectID)
	if err == sql.ErrNoRows {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		http.Error(w, "database error", http.StatusInternalServerError)
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
			http.Error(w, "database error", http.StatusInternalServerError)
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
			http.Error(w, "database error", http.StatusInternalServerError)
			return
		}
		version = &struct {
			ID         string
			VersionNum int
		}{v.ID, v.VersionNum}
	}

	pages, err := h.Storage.ListHTMLFiles(version.ID)
	if err != nil {
		http.Error(w, "storage error", http.StatusInternalServerError)
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
		http.Error(w, "template error", http.StatusInternalServerError)
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
	}{
		ProjectName: project.Name,
		ProjectID:   project.ID,
		Status:      project.Status,
		StatusLabel: statusLabels[project.Status],
		VersionID:   version.ID,
		VersionNum:  version.VersionNum,
		Pages:       pages,
		DefaultPage: defaultPage,
	}
	tmpl.Execute(w, data)
}
