package storage

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

type Storage struct {
	BasePath string
}

func New(basePath string) *Storage {
	os.MkdirAll(basePath, 0o755)
	return &Storage{BasePath: basePath}
}

func (s *Storage) SaveUpload(versionID string, zipData io.Reader) error {
	data, err := io.ReadAll(zipData)
	if err != nil {
		return err
	}
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return err
	}
	if len(zr.File) == 0 {
		return fmt.Errorf("zip is empty")
	}
	hasHTML := false
	for _, f := range zr.File {
		if strings.HasSuffix(strings.ToLower(f.Name), ".html") && !f.FileInfo().IsDir() {
			hasHTML = true
			break
		}
	}
	if !hasHTML {
		return fmt.Errorf("zip must contain at least one .html file")
	}
	dir := filepath.Join(s.BasePath, versionID)
	for _, f := range zr.File {
		target := filepath.Join(dir, f.Name)
		if !strings.HasPrefix(target, filepath.Clean(dir)+string(os.PathSeparator)) && target != filepath.Clean(dir) {
			continue // skip path traversal entries
		}
		if f.FileInfo().IsDir() {
			os.MkdirAll(target, 0o755)
			continue
		}
		os.MkdirAll(filepath.Dir(target), 0o755)
		rc, err := f.Open()
		if err != nil {
			return err
		}
		out, err := os.Create(target)
		if err != nil {
			rc.Close()
			return err
		}
		_, err = io.Copy(out, rc)
		rc.Close()
		out.Close()
		if err != nil {
			return err
		}
	}
	return nil
}

func (s *Storage) GetFilePath(versionID, filePath string) string {
	return filepath.Join(s.BasePath, versionID, filePath)
}

func (s *Storage) ListHTMLFiles(versionID string) ([]string, error) {
	dir := filepath.Join(s.BasePath, versionID)
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var files []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(strings.ToLower(e.Name()), ".html") {
			files = append(files, e.Name())
		}
	}
	return files, nil
}
