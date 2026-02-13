package api

import (
	"encoding/json"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ab/design-reviewer/internal/db"
	"github.com/ab/design-reviewer/internal/storage"
)

func setupTestHandler(t *testing.T) *Handler {
	t.Helper()
	tmp := t.TempDir()
	database, err := db.New(filepath.Join(tmp, "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { database.Close() })
	return &Handler{
		DB:           database,
		Storage:      storage.New(filepath.Join(tmp, "uploads")),
		TemplatesDir: "../../web/templates",
		StaticDir:    "../../web/static",
	}
}

func TestHandleListProjectsEmpty(t *testing.T) {
	h := setupTestHandler(t)
	req := httptest.NewRequest("GET", "/api/projects", nil)
	w := httptest.NewRecorder()
	h.handleListProjects(w, req)

	if w.Code != 200 {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var result []map[string]any
	json.NewDecoder(w.Body).Decode(&result)
	if len(result) != 0 {
		t.Errorf("expected empty array, got %d items", len(result))
	}
}

func TestHandleListProjectsWithData(t *testing.T) {
	h := setupTestHandler(t)
	p, _ := h.DB.CreateProject("test-proj", "")
	h.DB.CreateVersion(p.ID, "/tmp/v1")

	req := httptest.NewRequest("GET", "/api/projects", nil)
	w := httptest.NewRecorder()
	h.handleListProjects(w, req)

	if w.Code != 200 {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if ct := w.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("expected application/json, got %s", ct)
	}

	var result []map[string]any
	json.NewDecoder(w.Body).Decode(&result)
	if len(result) != 1 {
		t.Fatalf("expected 1 project, got %d", len(result))
	}
	if result[0]["name"] != "test-proj" {
		t.Errorf("expected name=test-proj, got %v", result[0]["name"])
	}
	if result[0]["version_count"].(float64) != 1 {
		t.Errorf("expected version_count=1, got %v", result[0]["version_count"])
	}
	if result[0]["id"] == nil || result[0]["status"] == nil || result[0]["updated_at"] == nil {
		t.Error("missing expected fields in response")
	}
}

func TestHandleHomeEmpty(t *testing.T) {
	h := setupTestHandler(t)
	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()
	h.handleHome(w, req)

	if w.Code != 200 {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	body := w.Body.String()
	if !strings.Contains(body, "Design Reviewer") {
		t.Error("missing page title")
	}
	if !strings.Contains(body, "No projects yet") {
		t.Error("missing empty state message")
	}
}

func TestHandleHomeWithProjects(t *testing.T) {
	h := setupTestHandler(t)
	p, _ := h.DB.CreateProject("my-design", "")
	h.DB.CreateVersion(p.ID, "/tmp/v1")

	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()
	h.handleHome(w, req)

	if w.Code != 200 {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	body := w.Body.String()
	if !strings.Contains(body, "my-design") {
		t.Error("missing project name in HTML")
	}
	if !strings.Contains(body, "/projects/"+p.ID) {
		t.Error("missing project link")
	}
	if !strings.Contains(body, "badge-draft") {
		t.Error("missing status badge class")
	}
}

func TestHandleHomeTemplateMissing(t *testing.T) {
	h := setupTestHandler(t)
	h.TemplatesDir = "/nonexistent"
	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()
	h.handleHome(w, req)

	if w.Code != 500 {
		t.Errorf("expected 500, got %d", w.Code)
	}
}

func TestRelativeTime(t *testing.T) {
	tests := []struct {
		name string
		dur  time.Duration
		want string
	}{
		{"just now", 10 * time.Second, "just now"},
		{"1 minute", time.Minute, "1 minute ago"},
		{"5 minutes", 5 * time.Minute, "5 minutes ago"},
		{"1 hour", time.Hour, "1 hour ago"},
		{"3 hours", 3 * time.Hour, "3 hours ago"},
		{"1 day", 25 * time.Hour, "1 day ago"},
		{"5 days", 121 * time.Hour, "5 days ago"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := relativeTime(time.Now().Add(-tt.dur))
			if got != tt.want {
				t.Errorf("relativeTime(%v ago) = %q, want %q", tt.dur, got, tt.want)
			}
		})
	}
}

// --- Phase 7: Status Workflow ---

func TestHandleUpdateStatusSuccess(t *testing.T) {
	h := setupTestHandler(t)
	p, _ := h.DB.CreateProject("proj", "")

	req := httptest.NewRequest("PATCH", "/api/projects/"+p.ID+"/status", strings.NewReader(`{"status":"in_review"}`))
	req.SetPathValue("id", p.ID)
	w := httptest.NewRecorder()
	h.handleUpdateStatus(w, req)

	if w.Code != 200 {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var resp map[string]string
	json.NewDecoder(w.Body).Decode(&resp)
	if resp["id"] != p.ID {
		t.Errorf("expected id=%s, got %s", p.ID, resp["id"])
	}
	if resp["status"] != "in_review" {
		t.Errorf("expected status=in_review, got %s", resp["status"])
	}

	// Verify DB was updated
	updated, _ := h.DB.GetProject(p.ID)
	if updated.Status != "in_review" {
		t.Errorf("DB status not updated, got %s", updated.Status)
	}
}

func TestHandleUpdateStatusAllStatuses(t *testing.T) {
	h := setupTestHandler(t)
	p, _ := h.DB.CreateProject("proj", "")

	for _, s := range []string{"in_review", "approved", "handed_off", "draft"} {
		req := httptest.NewRequest("PATCH", "/api/projects/"+p.ID+"/status", strings.NewReader(`{"status":"`+s+`"}`))
		req.SetPathValue("id", p.ID)
		w := httptest.NewRecorder()
		h.handleUpdateStatus(w, req)
		if w.Code != 200 {
			t.Errorf("status %q: expected 200, got %d", s, w.Code)
		}
	}
}

func TestHandleUpdateStatusInvalid(t *testing.T) {
	h := setupTestHandler(t)
	p, _ := h.DB.CreateProject("proj", "")

	req := httptest.NewRequest("PATCH", "/api/projects/"+p.ID+"/status", strings.NewReader(`{"status":"bogus"}`))
	req.SetPathValue("id", p.ID)
	w := httptest.NewRecorder()
	h.handleUpdateStatus(w, req)

	if w.Code != 400 {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestHandleUpdateStatusNotFound(t *testing.T) {
	h := setupTestHandler(t)

	req := httptest.NewRequest("PATCH", "/api/projects/nonexistent/status", strings.NewReader(`{"status":"draft"}`))
	req.SetPathValue("id", "nonexistent")
	w := httptest.NewRecorder()
	h.handleUpdateStatus(w, req)

	if w.Code != 404 {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

func TestHandleUpdateStatusBadJSON(t *testing.T) {
	h := setupTestHandler(t)

	req := httptest.NewRequest("PATCH", "/api/projects/x/status", strings.NewReader(`not json`))
	req.SetPathValue("id", "x")
	w := httptest.NewRecorder()
	h.handleUpdateStatus(w, req)

	if w.Code != 400 {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

// --- DB error path tests ---

func TestHandleListProjectsDBError(t *testing.T) {
	h := mockHandler(t, func(m *mockDB) { m.listProjectsWithVCErr = errDB })
	req := httptest.NewRequest("GET", "/api/projects", nil)
	w := httptest.NewRecorder()
	h.handleListProjects(w, req)
	if w.Code != 500 {
		t.Errorf("expected 500, got %d", w.Code)
	}
}

func TestHandleHomeDBError(t *testing.T) {
	h := mockHandler(t, func(m *mockDB) { m.listProjectsWithVCErr = errDB })
	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()
	h.handleHome(w, req)
	if w.Code != 500 {
		t.Errorf("expected 500, got %d", w.Code)
	}
}

func TestHandleUpdateStatusDBError(t *testing.T) {
	h := mockHandler(t, func(m *mockDB) { m.updateProjectStatusErr = errDB })
	req := httptest.NewRequest("PATCH", "/api/projects/x/status", strings.NewReader(`{"status":"draft"}`))
	req.SetPathValue("id", "x")
	w := httptest.NewRecorder()
	h.handleUpdateStatus(w, req)
	if w.Code != 500 {
		t.Errorf("expected 500, got %d", w.Code)
	}
}
