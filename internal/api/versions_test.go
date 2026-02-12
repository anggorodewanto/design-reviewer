package api

import (
	"encoding/json"
	"net/http/httptest"
	"testing"
)

func TestHandleListVersionsEmpty(t *testing.T) {
	h := setupTestHandler(t)
	p, _ := h.DB.CreateProject("empty-ver")

	req := httptest.NewRequest("GET", "/api/projects/"+p.ID+"/versions", nil)
	req.SetPathValue("id", p.ID)
	w := httptest.NewRecorder()
	h.handleListVersions(w, req)

	if w.Code != 200 {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var versions []map[string]any
	json.NewDecoder(w.Body).Decode(&versions)
	if len(versions) != 0 {
		t.Errorf("expected 0 versions, got %d", len(versions))
	}
}

func TestHandleListVersionsOrdered(t *testing.T) {
	h := setupTestHandler(t)
	p, _ := h.DB.CreateProject("ver-order")
	h.DB.CreateVersion(p.ID, "")
	h.DB.CreateVersion(p.ID, "")
	h.DB.CreateVersion(p.ID, "")

	req := httptest.NewRequest("GET", "/api/projects/"+p.ID+"/versions", nil)
	req.SetPathValue("id", p.ID)
	w := httptest.NewRecorder()
	h.handleListVersions(w, req)

	if w.Code != 200 {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var versions []map[string]any
	json.NewDecoder(w.Body).Decode(&versions)
	if len(versions) != 3 {
		t.Fatalf("expected 3 versions, got %d", len(versions))
	}
	// Should be newest first (DESC)
	if versions[0]["version_num"].(float64) != 3 {
		t.Errorf("first version should be 3, got %v", versions[0]["version_num"])
	}
	if versions[2]["version_num"].(float64) != 1 {
		t.Errorf("last version should be 1, got %v", versions[2]["version_num"])
	}
}

func TestHandleListVersionsResponseFormat(t *testing.T) {
	h := setupTestHandler(t)
	p, _ := h.DB.CreateProject("ver-fmt")
	h.DB.CreateVersion(p.ID, "")

	req := httptest.NewRequest("GET", "/api/projects/"+p.ID+"/versions", nil)
	req.SetPathValue("id", p.ID)
	w := httptest.NewRecorder()
	h.handleListVersions(w, req)

	if ct := w.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("expected Content-Type application/json, got %s", ct)
	}
	var versions []map[string]any
	json.NewDecoder(w.Body).Decode(&versions)
	v := versions[0]
	for _, field := range []string{"id", "version_num", "created_at", "pages"} {
		if v[field] == nil {
			t.Errorf("missing field %q", field)
		}
	}
}

func TestHandleListVersionsDBError(t *testing.T) {
	h := mockHandler(t, func(m *mockDB) { m.listVersionsErr = errDB })

	req := httptest.NewRequest("GET", "/api/projects/some-id/versions", nil)
	req.SetPathValue("id", "some-id")
	w := httptest.NewRecorder()
	h.handleListVersions(w, req)

	if w.Code != 500 {
		t.Errorf("expected 500, got %d", w.Code)
	}
}
