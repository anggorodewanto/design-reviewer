package api

import (
	"net/http"
	"os"
	"strings"
)

func (h *Handler) handleDesignFile(w http.ResponseWriter, r *http.Request) {
	versionID := r.PathValue("version_id")
	filePath := r.PathValue("filepath")

	if strings.Contains(filePath, "..") {
		http.Error(w, "invalid path", http.StatusBadRequest)
		return
	}

	fullPath := h.Storage.GetFilePath(versionID, filePath)
	f, err := os.Open(fullPath)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	defer f.Close()

	stat, err := f.Stat()
	if err != nil || stat.IsDir() {
		http.NotFound(w, r)
		return
	}

	http.ServeContent(w, r, filePath, stat.ModTime(), f)
}
