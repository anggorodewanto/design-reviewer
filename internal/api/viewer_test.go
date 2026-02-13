package api

import (
	"archive/zip"
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/ab/design-reviewer/internal/auth"
)

func seedProject(t *testing.T, h *Handler, files map[string]string) (projectID, versionID string) {
	t.Helper()
	p, err := h.DB.CreateProject("test-proj", "")
	if err != nil {
		t.Fatal(err)
	}
	v, err := h.DB.CreateVersion(p.ID, "")
	if err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for name, content := range files {
		f, _ := zw.Create(name)
		f.Write([]byte(content))
	}
	zw.Close()
	if err := h.Storage.SaveUpload(v.ID, &buf); err != nil {
		t.Fatal(err)
	}
	return p.ID, v.ID
}

func TestHandleViewerSuccess(t *testing.T) {
	h := setupTestHandler(t)
	pid, vid := seedProject(t, h, map[string]string{"index.html": "<h1>hi</h1>", "about.html": "<h1>about</h1>"})

	req := httptest.NewRequest("GET", "/projects/"+pid, nil)
	req.SetPathValue("id", pid)
	w := httptest.NewRecorder()
	h.handleViewer(w, req)

	if w.Code != 200 {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	body := w.Body.String()
	if !strings.Contains(body, "test-proj") {
		t.Error("missing project name")
	}
	if !strings.Contains(body, vid) {
		t.Error("missing version ID in iframe src")
	}
	if !strings.Contains(body, "index.html") {
		t.Error("missing default page")
	}
	if !strings.Contains(body, `sandbox="allow-same-origin"`) {
		t.Error("missing sandbox attribute")
	}
}

func TestHandleViewerDefaultPageFallback(t *testing.T) {
	h := setupTestHandler(t)
	pid, _ := seedProject(t, h, map[string]string{"page.html": "<h1>page</h1>"})

	req := httptest.NewRequest("GET", "/projects/"+pid, nil)
	req.SetPathValue("id", pid)
	w := httptest.NewRecorder()
	h.handleViewer(w, req)

	if w.Code != 200 {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), "page.html") {
		t.Error("should fall back to first HTML file when no index.html")
	}
}

func TestHandleViewerWithVersionParam(t *testing.T) {
	h := setupTestHandler(t)
	pid, vid := seedProject(t, h, map[string]string{"index.html": "v1"})

	req := httptest.NewRequest("GET", "/projects/"+pid+"?version="+vid, nil)
	req.SetPathValue("id", pid)
	w := httptest.NewRecorder()
	h.handleViewer(w, req)

	if w.Code != 200 {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), vid) {
		t.Error("should use specified version")
	}
}

func TestHandleViewerProjectNotFound(t *testing.T) {
	h := setupTestHandler(t)
	req := httptest.NewRequest("GET", "/projects/nonexistent", nil)
	req.SetPathValue("id", "nonexistent")
	w := httptest.NewRecorder()
	h.handleViewer(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

func TestHandleViewerNoVersions(t *testing.T) {
	h := setupTestHandler(t)
	p, _ := h.DB.CreateProject("empty-proj", "")

	req := httptest.NewRequest("GET", "/projects/"+p.ID, nil)
	req.SetPathValue("id", p.ID)
	w := httptest.NewRecorder()
	h.handleViewer(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404 for project with no versions, got %d", w.Code)
	}
}

func TestHandleViewerInvalidVersionParam(t *testing.T) {
	h := setupTestHandler(t)
	pid, _ := seedProject(t, h, map[string]string{"index.html": "x"})

	req := httptest.NewRequest("GET", "/projects/"+pid+"?version=bad-id", nil)
	req.SetPathValue("id", pid)
	w := httptest.NewRecorder()
	h.handleViewer(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404 for invalid version, got %d", w.Code)
	}
}

func TestHandleViewerTemplateMissing(t *testing.T) {
	h := setupTestHandler(t)
	pid, _ := seedProject(t, h, map[string]string{"index.html": "x"})
	h.TemplatesDir = "/nonexistent"

	req := httptest.NewRequest("GET", "/projects/"+pid, nil)
	req.SetPathValue("id", pid)
	w := httptest.NewRecorder()
	h.handleViewer(w, req)

	if w.Code != 500 {
		t.Errorf("expected 500, got %d", w.Code)
	}
}

func TestHandleViewerPageTabs(t *testing.T) {
	h := setupTestHandler(t)
	pid, _ := seedProject(t, h, map[string]string{
		"index.html":   "<h1>home</h1>",
		"about.html":   "<h1>about</h1>",
		"contact.html": "<h1>contact</h1>",
	})

	req := httptest.NewRequest("GET", "/projects/"+pid, nil)
	req.SetPathValue("id", pid)
	w := httptest.NewRecorder()
	h.handleViewer(w, req)

	body := w.Body.String()
	if !strings.Contains(body, "page-tabs") {
		t.Error("missing page tabs container")
	}
	if !strings.Contains(body, "about.html") || !strings.Contains(body, "contact.html") {
		t.Error("missing page tab entries")
	}
}

// --- DB error path tests for viewer ---

func TestHandleViewerGetProjectDBError(t *testing.T) {
	h := mockHandler(t, func(m *mockDB) { m.getProjectErr = errDB })
	req := httptest.NewRequest("GET", "/projects/some-id", nil)
	req.SetPathValue("id", "some-id")
	w := httptest.NewRecorder()
	h.handleViewer(w, req)
	if w.Code != 500 {
		t.Errorf("expected 500, got %d", w.Code)
	}
}

func TestHandleViewerGetVersionDBError(t *testing.T) {
	h := setupTestHandler(t)
	pid, _ := seedProject(t, h, map[string]string{"index.html": "x"})
	m := &mockDB{DataStore: h.DB, getVersionErr: errDB}
	h.DB = m

	req := httptest.NewRequest("GET", "/projects/"+pid+"?version=bad", nil)
	req.SetPathValue("id", pid)
	w := httptest.NewRecorder()
	h.handleViewer(w, req)
	if w.Code != 500 {
		t.Errorf("expected 500, got %d", w.Code)
	}
}

func TestHandleViewerGetLatestVersionDBError(t *testing.T) {
	h := setupTestHandler(t)
	pid, _ := seedProject(t, h, map[string]string{"index.html": "x"})
	m := &mockDB{DataStore: h.DB, getLatestVersionErr: errDB}
	h.DB = m

	req := httptest.NewRequest("GET", "/projects/"+pid, nil)
	req.SetPathValue("id", pid)
	w := httptest.NewRecorder()
	h.handleViewer(w, req)
	if w.Code != 500 {
		t.Errorf("expected 500, got %d", w.Code)
	}
}

func TestHandleViewerIsOwner(t *testing.T) {
	h := setupTestHandler(t)
	p, _ := h.DB.CreateProject("owner-proj", "alice@test.com")
	v, _ := h.DB.CreateVersion(p.ID, "")
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	f, _ := zw.Create("index.html")
	f.Write([]byte("<h1>hi</h1>"))
	zw.Close()
	h.Storage.SaveUpload(v.ID, &buf)

	req := httptest.NewRequest("GET", "/projects/"+p.ID, nil)
	req.SetPathValue("id", p.ID)
	ctx := auth.SetUserInContext(req.Context(), "Alice", "alice@test.com")
	req = req.WithContext(ctx)
	w := httptest.NewRecorder()
	h.handleViewer(w, req)

	if w.Code != 200 {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), "share-btn") {
		t.Error("owner should see share button")
	}
}

func TestHandleViewerNotOwner(t *testing.T) {
	h := setupTestHandler(t)
	p, _ := h.DB.CreateProject("other-proj", "alice@test.com")
	v, _ := h.DB.CreateVersion(p.ID, "")
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	f, _ := zw.Create("index.html")
	f.Write([]byte("<h1>hi</h1>"))
	zw.Close()
	h.Storage.SaveUpload(v.ID, &buf)

	req := httptest.NewRequest("GET", "/projects/"+p.ID, nil)
	req.SetPathValue("id", p.ID)
	ctx := auth.SetUserInContext(req.Context(), "Bob", "bob@test.com")
	req = req.WithContext(ctx)
	w := httptest.NewRecorder()
	h.handleViewer(w, req)

	if w.Code != 200 {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if strings.Contains(w.Body.String(), "share-btn") {
		t.Error("non-owner should not see share button")
	}
}

func TestHandleViewerStorageError(t *testing.T) {
	h := setupTestHandler(t)
	p, _ := h.DB.CreateProject("proj", "")
	// Create version but don't save any files — ListHTMLFiles will fail
	h.DB.CreateVersion(p.ID, "")

	req := httptest.NewRequest("GET", "/projects/"+p.ID, nil)
	req.SetPathValue("id", p.ID)
	w := httptest.NewRecorder()
	h.handleViewer(w, req)

	if w.Code != 500 {
		t.Errorf("expected 500, got %d", w.Code)
	}
}
