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

const maxDecompressedSize = 500 << 20 // 500 MB
const maxFileCount = 1000

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
	if len(zr.File) > maxFileCount {
		return fmt.Errorf("zip contains too many files (max %d)", maxFileCount)
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
	var totalWritten int64
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
		n, err := io.Copy(out, io.LimitReader(rc, maxDecompressedSize-totalWritten+1))
		rc.Close()
		out.Close()
		totalWritten += n
		if err != nil {
			return err
		}
		if totalWritten > maxDecompressedSize {
			return fmt.Errorf("decompressed size exceeds limit (%d bytes)", maxDecompressedSize)
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
