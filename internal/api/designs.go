package api

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

func (h *Handler) handleDesignFile(w http.ResponseWriter, r *http.Request) {
	versionID := r.PathValue("version_id")
	filePath := r.PathValue("filepath")

	fullPath := h.Storage.GetFilePath(versionID, filePath)
	baseDir := filepath.Clean(h.Storage.GetFilePath(versionID, "")) + string(os.PathSeparator)
	if !strings.HasPrefix(fullPath, baseDir) {
		http.Error(w, "invalid path", http.StatusBadRequest)
		return
	}
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
