package storage

import (
	"archive/zip"
	"bytes"
	"os"
	"path/filepath"
	"sort"
	"testing"
)

func makeZip(t *testing.T, files map[string]string) *bytes.Buffer {
	t.Helper()
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	for name, content := range files {
		f, _ := w.Create(name)
		f.Write([]byte(content))
	}
	w.Close()
	return &buf
}

func TestNew(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "uploads")
	s := New(dir)
	if s.BasePath != dir {
		t.Errorf("BasePath = %q, want %q", s.BasePath, dir)
	}
	if _, err := os.Stat(dir); err != nil {
		t.Errorf("directory not created: %v", err)
	}
}

func TestSaveUploadAndGetFilePath(t *testing.T) {
	s := New(filepath.Join(t.TempDir(), "uploads"))
	z := makeZip(t, map[string]string{"index.html": "<h1>hi</h1>", "style.css": "body{}"})

	if err := s.SaveUpload("v1", z); err != nil {
		t.Fatal(err)
	}

	path := s.GetFilePath("v1", "index.html")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "<h1>hi</h1>" {
		t.Errorf("content = %q", data)
	}
}

func TestSaveUploadNoHTML(t *testing.T) {
	s := New(filepath.Join(t.TempDir(), "uploads"))
	z := makeZip(t, map[string]string{"readme.txt": "no html"})

	err := s.SaveUpload("v1", z)
	if err == nil {
		t.Error("expected error for zip without HTML")
	}
}

func TestSaveUploadEmptyZip(t *testing.T) {
	s := New(filepath.Join(t.TempDir(), "uploads"))
	var buf bytes.Buffer
	zip.NewWriter(&buf).Close()

	err := s.SaveUpload("v1", &buf)
	if err == nil {
		t.Error("expected error for empty zip")
	}
}

func TestSaveUploadInvalidZip(t *testing.T) {
	s := New(filepath.Join(t.TempDir(), "uploads"))
	err := s.SaveUpload("v1", bytes.NewReader([]byte("not a zip")))
	if err == nil {
		t.Error("expected error for invalid zip")
	}
}

func TestListHTMLFiles(t *testing.T) {
	s := New(filepath.Join(t.TempDir(), "uploads"))
	z := makeZip(t, map[string]string{"index.html": "a", "about.html": "b", "style.css": "c"})
	s.SaveUpload("v1", z)

	files, err := s.ListHTMLFiles("v1")
	if err != nil {
		t.Fatal(err)
	}
	sort.Strings(files)
	if len(files) != 2 || files[0] != "about.html" || files[1] != "index.html" {
		t.Errorf("files = %v, want [about.html index.html]", files)
	}
}

func TestListHTMLFilesNoDir(t *testing.T) {
	s := New(filepath.Join(t.TempDir(), "uploads"))
	_, err := s.ListHTMLFiles("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent version")
	}
}

func TestGetFilePath(t *testing.T) {
	s := &Storage{BasePath: "/base"}
	got := s.GetFilePath("v1", "index.html")
	want := filepath.Join("/base", "v1", "index.html")
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestSaveUploadWithSubdirectories(t *testing.T) {
	s := New(filepath.Join(t.TempDir(), "uploads"))
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	// Add a directory entry
	w.Create("images/")
	f, _ := w.Create("index.html")
	f.Write([]byte("<h1>hi</h1>"))
	f2, _ := w.Create("images/logo.png")
	f2.Write([]byte("png-data"))
	w.Close()

	if err := s.SaveUpload("v1", &buf); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(s.GetFilePath("v1", "images/logo.png"))
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "png-data" {
		t.Errorf("content = %q", data)
	}
}

func TestSaveUploadPathTraversalSkipped(t *testing.T) {
	s := New(filepath.Join(t.TempDir(), "uploads"))
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	f, _ := w.Create("index.html")
	f.Write([]byte("ok"))
	// Add a path traversal entry
	f2, _ := w.Create("../../../etc/passwd")
	f2.Write([]byte("evil"))
	w.Close()

	if err := s.SaveUpload("v1", &buf); err != nil {
		t.Fatal(err)
	}
	// The traversal file should not exist outside the version dir
	if _, err := os.Stat(s.GetFilePath("v1", "../../../etc/passwd")); err == nil {
		t.Error("path traversal file should not be created")
	}
}

func TestSaveUploadReadOnlyDir(t *testing.T) {
	tmp := t.TempDir()
	roDir := filepath.Join(tmp, "readonly")
	os.MkdirAll(roDir, 0755)
	s := New(filepath.Join(roDir, "uploads"))
	// Make parent read-only after creating uploads dir
	os.Chmod(roDir, 0444)
	t.Cleanup(func() { os.Chmod(roDir, 0755) })

	z := makeZip(t, map[string]string{"index.html": "x"})
	err := s.SaveUpload("v1", z)
	if err == nil {
		t.Error("expected error writing to read-only directory")
	}
}

func TestSaveUploadHTMLCaseInsensitive(t *testing.T) {
	s := New(filepath.Join(t.TempDir(), "uploads"))
	z := makeZip(t, map[string]string{"PAGE.HTML": "<h1>hi</h1>"})
	if err := s.SaveUpload("v1", z); err != nil {
		t.Fatal(err)
	}
}
