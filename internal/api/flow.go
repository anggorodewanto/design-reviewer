package api

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/ab/design-reviewer/internal/flow"
)

func (h *Handler) handleGetFlow(w http.ResponseWriter, r *http.Request) {
	versionID := r.PathValue("id")

	baseDir := h.Storage.GetFilePath(versionID, "")

	// List all HTML files (walk recursively for subdirectory pages).
	var pages []string
	filepath.WalkDir(baseDir, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		if strings.HasSuffix(strings.ToLower(d.Name()), ".html") {
			rel, _ := filepath.Rel(baseDir, path)
			pages = append(pages, filepath.ToSlash(rel))
		}
		return nil
	})

	// Parse flow.yaml if present.
	var yamlDef *flow.FlowDef
	if f, err := os.Open(h.Storage.GetFilePath(versionID, "flow.yaml")); err == nil {
		defer f.Close()
		parsed, err := flow.ParseFlowYAML(f)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		yamlDef = parsed
	}

	// Extract data-dr-link from each HTML file.
	htmlEdges := make(map[string][]flow.Edge)
	for _, page := range pages {
		f, err := os.Open(h.Storage.GetFilePath(versionID, page))
		if err != nil {
			continue
		}
		edges, err := flow.ExtractHTMLLinks(page, f)
		f.Close()
		if err != nil {
			continue
		}
		if len(edges) > 0 {
			htmlEdges[page] = edges
		}
	}

	graph := flow.BuildGraph(pages, yamlDef, htmlEdges)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(graph)
}
