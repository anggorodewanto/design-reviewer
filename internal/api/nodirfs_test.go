package api

import (
	"net/http"
	"os"
	"path/filepath"
	"testing"
)

func TestNoDirFS_OpenFile(t *testing.T) {
	tmp := t.TempDir()
	os.WriteFile(filepath.Join(tmp, "style.css"), []byte("body{}"), 0644)

	fs := noDirFS{http.Dir(tmp)}
	f, err := fs.Open("style.css")
	if err != nil {
		t.Fatalf("expected file, got error: %v", err)
	}
	f.Close()
}

func TestNoDirFS_OpenDirReturnsNotExist(t *testing.T) {
	tmp := t.TempDir()
	os.MkdirAll(filepath.Join(tmp, "subdir"), 0755)

	fs := noDirFS{http.Dir(tmp)}
	_, err := fs.Open("subdir")
	if !os.IsNotExist(err) {
		t.Fatalf("expected os.ErrNotExist for directory, got: %v", err)
	}
}

func TestNoDirFS_OpenRootDirReturnsNotExist(t *testing.T) {
	tmp := t.TempDir()

	fs := noDirFS{http.Dir(tmp)}
	_, err := fs.Open("/")
	if !os.IsNotExist(err) {
		t.Fatalf("expected os.ErrNotExist for root dir, got: %v", err)
	}
}

func TestNoDirFS_OpenNonExistentReturnsError(t *testing.T) {
	tmp := t.TempDir()

	fs := noDirFS{http.Dir(tmp)}
	_, err := fs.Open("nope.txt")
	if err == nil {
		t.Fatal("expected error for non-existent file")
	}
}
