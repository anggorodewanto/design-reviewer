package api

import (
	"archive/zip"
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/ab/design-reviewer/internal/auth"
)

func withUser(r *http.Request, name, email string) *http.Request {
	ctx := auth.SetUserInContext(r.Context(), name, email)
	return r.WithContext(ctx)
}

func TestHandleCreateInvite(t *testing.T) {
	h := setupTestHandler(t)
	h.Auth = &auth.Config{BaseURL: "http://localhost:8080"}
	p, _ := h.DB.CreateProject("proj", "alice@test.com")

	req := httptest.NewRequest("POST", "/api/projects/"+p.ID+"/invites", nil)
	req.SetPathValue("id", p.ID)
	req = withUser(req, "Alice", "alice@test.com")
	w := httptest.NewRecorder()
	h.handleCreateInvite(w, req)

	if w.Code != 200 {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var result map[string]string
	json.NewDecoder(w.Body).Decode(&result)
	if result["invite_url"] == "" {
		t.Error("expected invite_url in response")
	}
	if result["id"] == "" {
		t.Error("expected id in response")
	}
}

func TestHandleCreateInviteDBError(t *testing.T) {
	h := mockHandler(t, func(m *mockDB) { m.createInviteErr = errDB })
	p, _ := h.DB.CreateProject("proj", "a@t.com")
	req := httptest.NewRequest("POST", "/api/projects/"+p.ID+"/invites", nil)
	req.SetPathValue("id", p.ID)
	req = withUser(req, "A", "a@t.com")
	w := httptest.NewRecorder()
	h.handleCreateInvite(w, req)
	if w.Code != 500 {
		t.Errorf("expected 500, got %d", w.Code)
	}
}

func TestHandleDeleteInvite(t *testing.T) {
	h := setupTestHandler(t)
	p, _ := h.DB.CreateProject("proj", "alice@test.com")
	inv, _ := h.DB.CreateInvite(p.ID, "alice@test.com")

	req := httptest.NewRequest("DELETE", "/api/projects/"+p.ID+"/invites/"+inv.ID, nil)
	req.SetPathValue("id", p.ID)
	req.SetPathValue("inviteID", inv.ID)
	w := httptest.NewRecorder()
	h.handleDeleteInvite(w, req)

	if w.Code != 204 {
		t.Fatalf("expected 204, got %d", w.Code)
	}
}

func TestHandleListMembers(t *testing.T) {
	h := setupTestHandler(t)
	p, _ := h.DB.CreateProject("proj", "alice@test.com")
	h.DB.AddMember(p.ID, "bob@test.com")

	req := httptest.NewRequest("GET", "/api/projects/"+p.ID+"/members", nil)
	req.SetPathValue("id", p.ID)
	w := httptest.NewRecorder()
	h.handleListMembers(w, req)

	if w.Code != 200 {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var members []map[string]string
	json.NewDecoder(w.Body).Decode(&members)
	if len(members) != 1 {
		t.Fatalf("expected 1 member, got %d", len(members))
	}
	if members[0]["email"] != "bob@test.com" {
		t.Errorf("email = %q, want bob@test.com", members[0]["email"])
	}
}

func TestHandleListMembersEmpty(t *testing.T) {
	h := setupTestHandler(t)
	p, _ := h.DB.CreateProject("proj", "alice@test.com")

	req := httptest.NewRequest("GET", "/api/projects/"+p.ID+"/members", nil)
	req.SetPathValue("id", p.ID)
	w := httptest.NewRecorder()
	h.handleListMembers(w, req)

	if w.Code != 200 {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var members []map[string]string
	json.NewDecoder(w.Body).Decode(&members)
	if len(members) != 0 {
		t.Errorf("expected 0 members, got %d", len(members))
	}
}

func TestHandleListMembersDBError(t *testing.T) {
	h := mockHandler(t, func(m *mockDB) { m.listMembersErr = errDB })
	req := httptest.NewRequest("GET", "/api/projects/x/members", nil)
	req.SetPathValue("id", "x")
	w := httptest.NewRecorder()
	h.handleListMembers(w, req)
	if w.Code != 500 {
		t.Errorf("expected 500, got %d", w.Code)
	}
}

func TestHandleRemoveMember(t *testing.T) {
	h := setupTestHandler(t)
	p, _ := h.DB.CreateProject("proj", "alice@test.com")
	h.DB.AddMember(p.ID, "bob@test.com")

	req := httptest.NewRequest("DELETE", "/api/projects/"+p.ID+"/members/bob@test.com", nil)
	req.SetPathValue("id", p.ID)
	req.SetPathValue("email", "bob@test.com")
	req = withUser(req, "Alice", "alice@test.com")
	w := httptest.NewRecorder()
	h.handleRemoveMember(w, req)

	if w.Code != 204 {
		t.Fatalf("expected 204, got %d", w.Code)
	}
	members, _ := h.DB.ListMembers(p.ID)
	if len(members) != 0 {
		t.Errorf("expected 0 members after removal, got %d", len(members))
	}
}

func TestHandleRemoveMemberCannotRemoveOwner(t *testing.T) {
	h := setupTestHandler(t)
	p, _ := h.DB.CreateProject("proj", "alice@test.com")

	req := httptest.NewRequest("DELETE", "/api/projects/"+p.ID+"/members/alice@test.com", nil)
	req.SetPathValue("id", p.ID)
	req.SetPathValue("email", "alice@test.com")
	req = withUser(req, "Alice", "alice@test.com")
	w := httptest.NewRecorder()
	h.handleRemoveMember(w, req)

	if w.Code != 400 {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestHandleAcceptInvite(t *testing.T) {
	h := setupTestHandler(t)
	p, _ := h.DB.CreateProject("proj", "alice@test.com")
	inv, _ := h.DB.CreateInvite(p.ID, "alice@test.com")

	req := httptest.NewRequest("GET", "/invite/"+inv.Token, nil)
	req.SetPathValue("token", inv.Token)
	req = withUser(req, "Bob", "bob@test.com")
	w := httptest.NewRecorder()
	h.handleAcceptInvite(w, req)

	if w.Code != 302 {
		t.Fatalf("expected 302, got %d: %s", w.Code, w.Body.String())
	}
	if loc := w.Header().Get("Location"); loc != "/projects/"+p.ID {
		t.Errorf("redirect to %q, want /projects/%s", loc, p.ID)
	}
	// Verify member was added
	ok, _ := h.DB.CanAccessProject(p.ID, "bob@test.com")
	if !ok {
		t.Error("bob should have access after accepting invite")
	}
}

func TestHandleAcceptInviteInvalidToken(t *testing.T) {
	h := setupTestHandler(t)
	h.TemplatesDir = "../../web/templates"

	req := httptest.NewRequest("GET", "/invite/badtoken", nil)
	req.SetPathValue("token", "badtoken")
	req = withUser(req, "Bob", "bob@test.com")
	w := httptest.NewRecorder()
	h.handleAcceptInvite(w, req)

	if w.Code != 200 {
		t.Fatalf("expected 200 (error page), got %d", w.Code)
	}
}

func TestHandleAcceptInviteInvalidTokenNoTemplate(t *testing.T) {
	h := setupTestHandler(t)
	h.TemplatesDir = "/nonexistent"

	req := httptest.NewRequest("GET", "/invite/badtoken", nil)
	req.SetPathValue("token", "badtoken")
	req = withUser(req, "Bob", "bob@test.com")
	w := httptest.NewRecorder()
	h.handleAcceptInvite(w, req)

	if w.Code != 404 {
		t.Fatalf("expected 404 fallback, got %d", w.Code)
	}
}

func TestHandleAcceptInviteDBError(t *testing.T) {
	h := mockHandler(t, func(m *mockDB) {
		m.getInviteByTokenErr = sql.ErrConnDone // not ErrNoRows
	})
	req := httptest.NewRequest("GET", "/invite/tok", nil)
	req.SetPathValue("token", "tok")
	req = withUser(req, "B", "b@t.com")
	w := httptest.NewRecorder()
	h.handleAcceptInvite(w, req)
	if w.Code != 500 {
		t.Errorf("expected 500, got %d", w.Code)
	}
}

// --- Middleware tests ---

func TestProjectAccessMiddlewareAllowed(t *testing.T) {
	h := setupTestHandler(t)
	p, _ := h.DB.CreateProject("proj", "alice@test.com")

	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	})
	handler := h.projectAccess(inner)

	req := httptest.NewRequest("GET", "/projects/"+p.ID, nil)
	req.SetPathValue("id", p.ID)
	req = withUser(req, "Alice", "alice@test.com")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestProjectAccessMiddlewareDenied(t *testing.T) {
	h := setupTestHandler(t)
	p, _ := h.DB.CreateProject("proj", "alice@test.com")

	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	})
	handler := h.projectAccess(inner)

	req := httptest.NewRequest("GET", "/projects/"+p.ID, nil)
	req.SetPathValue("id", p.ID)
	req = withUser(req, "Bob", "bob@test.com")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != 404 {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

func TestProjectAccessMiddlewareNoUser(t *testing.T) {
	h := setupTestHandler(t)
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	})
	handler := h.projectAccess(inner)

	req := httptest.NewRequest("GET", "/projects/x", nil)
	req.SetPathValue("id", "x")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != 404 {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

func TestVersionAccessMiddlewareAllowed(t *testing.T) {
	h := setupTestHandler(t)
	p, _ := h.DB.CreateProject("proj", "alice@test.com")
	v, _ := h.DB.CreateVersion(p.ID, "/tmp")

	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	})
	handler := h.versionAccess(inner)

	req := httptest.NewRequest("GET", "/api/versions/"+v.ID+"/comments", nil)
	req.SetPathValue("id", v.ID)
	req = withUser(req, "Alice", "alice@test.com")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestVersionAccessMiddlewareDenied(t *testing.T) {
	h := setupTestHandler(t)
	p, _ := h.DB.CreateProject("proj", "alice@test.com")
	v, _ := h.DB.CreateVersion(p.ID, "/tmp")

	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	})
	handler := h.versionAccess(inner)

	req := httptest.NewRequest("GET", "/api/versions/"+v.ID+"/comments", nil)
	req.SetPathValue("id", v.ID)
	req = withUser(req, "Bob", "bob@test.com")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != 404 {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

func TestOwnerOnlyMiddlewareAllowed(t *testing.T) {
	h := setupTestHandler(t)
	p, _ := h.DB.CreateProject("proj", "alice@test.com")

	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	})
	handler := h.ownerOnly(inner)

	req := httptest.NewRequest("POST", "/api/projects/"+p.ID+"/invites", nil)
	req.SetPathValue("id", p.ID)
	req = withUser(req, "Alice", "alice@test.com")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestOwnerOnlyMiddlewareDenied(t *testing.T) {
	h := setupTestHandler(t)
	p, _ := h.DB.CreateProject("proj", "alice@test.com")

	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	})
	handler := h.ownerOnly(inner)

	req := httptest.NewRequest("POST", "/api/projects/"+p.ID+"/invites", nil)
	req.SetPathValue("id", p.ID)
	req = withUser(req, "Bob", "bob@test.com")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != 403 {
		t.Errorf("expected 403, got %d", w.Code)
	}
}

func TestOwnerOnlyMiddlewareNoUser(t *testing.T) {
	h := setupTestHandler(t)
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	})
	handler := h.ownerOnly(inner)

	req := httptest.NewRequest("POST", "/api/projects/x/invites", nil)
	req.SetPathValue("id", "x")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != 404 {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

// --- Upload access control ---

func TestUploadExistingProjectAccessDenied(t *testing.T) {
	h := setupTestHandler(t)
	h.DB.CreateProject("existing", "alice@test.com")

	// Bob tries to push to Alice's project
	z := makeTestZip(t, map[string]string{"index.html": "x"})
	req := createUploadRequest(t, "existing", z)
	req = withUser(req, "Bob", "bob@test.com")
	w := httptest.NewRecorder()
	h.handleUpload(w, req)

	if w.Code != 404 {
		t.Errorf("expected 404, got %d: %s", w.Code, w.Body.String())
	}
}

// helpers for upload tests
func makeTestZip(t *testing.T, files map[string]string) []byte {
	t.Helper()
	return makeZipForTest(t, files)
}

func createUploadRequest(t *testing.T, name string, zipData []byte) *http.Request {
	t.Helper()
	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	mw.WriteField("name", name)
	fw, _ := mw.CreateFormFile("file", "upload.zip")
	fw.Write(zipData)
	mw.Close()
	req := httptest.NewRequest("POST", "/api/upload", &body)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	return req
}

// Reuse the zip helper from viewer_test.go but with a different name to avoid conflicts
func makeZipForTest(t *testing.T, files map[string]string) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for name, content := range files {
		f, _ := zw.Create(name)
		f.Write([]byte(content))
	}
	zw.Close()
	return buf.Bytes()
}

// --- versionAccess additional paths ---

func TestVersionAccessMiddlewareWithVersionIDPathValue(t *testing.T) {
	h := setupTestHandler(t)
	p, _ := h.DB.CreateProject("proj", "alice@test.com")
	v, _ := h.DB.CreateVersion(p.ID, "/tmp")

	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(200) })
	handler := h.versionAccess(inner)

	req := httptest.NewRequest("GET", "/designs/"+v.ID+"/index.html", nil)
	req.SetPathValue("version_id", v.ID)
	req = withUser(req, "Alice", "alice@test.com")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestVersionAccessMiddlewareNoUser(t *testing.T) {
	h := setupTestHandler(t)
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(200) })
	handler := h.versionAccess(inner)
	req := httptest.NewRequest("GET", "/api/versions/x/comments", nil)
	req.SetPathValue("id", "x")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	if w.Code != 404 {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

func TestVersionAccessMiddlewareVersionNotFound(t *testing.T) {
	h := setupTestHandler(t)
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(200) })
	handler := h.versionAccess(inner)
	req := httptest.NewRequest("GET", "/api/versions/nonexistent/comments", nil)
	req.SetPathValue("id", "nonexistent")
	req = withUser(req, "Alice", "alice@test.com")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	if w.Code != 404 {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

// --- ownerOnly error path ---

func TestOwnerOnlyMiddlewareDBError(t *testing.T) {
	h := mockHandler(t, func(m *mockDB) { m.getProjectOwnerErr = errDB })
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(200) })
	handler := h.ownerOnly(inner)
	req := httptest.NewRequest("POST", "/api/projects/x/invites", nil)
	req.SetPathValue("id", "x")
	req = withUser(req, "A", "a@t.com")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	if w.Code != 404 {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

// --- handleDeleteInvite error ---

func TestHandleDeleteInviteDBError(t *testing.T) {
	h := mockHandler(t, func(m *mockDB) { m.deleteInviteErr = errDB })
	req := httptest.NewRequest("DELETE", "/api/projects/x/invites/inv1", nil)
	req.SetPathValue("id", "x")
	req.SetPathValue("inviteID", "inv1")
	w := httptest.NewRecorder()
	h.handleDeleteInvite(w, req)
	if w.Code != 500 {
		t.Errorf("expected 500, got %d", w.Code)
	}
}

// --- handleRemoveMember error paths ---

func TestHandleRemoveMemberGetOwnerDBError(t *testing.T) {
	h := mockHandler(t, func(m *mockDB) { m.getProjectOwnerErr = errDB })
	req := httptest.NewRequest("DELETE", "/api/projects/x/members/b@t.com", nil)
	req.SetPathValue("id", "x")
	req.SetPathValue("email", "b@t.com")
	req = withUser(req, "A", "a@t.com")
	w := httptest.NewRecorder()
	h.handleRemoveMember(w, req)
	if w.Code != 500 {
		t.Errorf("expected 500, got %d", w.Code)
	}
}

func TestHandleRemoveMemberDBError(t *testing.T) {
	h := mockHandler(t, func(m *mockDB) { m.removeMemberErr = errDB })
	p, _ := h.DB.CreateProject("proj", "a@t.com")
	h.DB.AddMember(p.ID, "b@t.com")
	req := httptest.NewRequest("DELETE", "/api/projects/"+p.ID+"/members/b@t.com", nil)
	req.SetPathValue("id", p.ID)
	req.SetPathValue("email", "b@t.com")
	req = withUser(req, "A", "a@t.com")
	w := httptest.NewRecorder()
	h.handleRemoveMember(w, req)
	if w.Code != 500 {
		t.Errorf("expected 500, got %d", w.Code)
	}
}

// --- handleAcceptInvite AddMember error ---

func TestHandleAcceptInviteAddMemberDBError(t *testing.T) {
	h := mockHandler(t, func(m *mockDB) { m.addMemberErr = errDB })
	p, _ := h.DB.CreateProject("proj", "a@t.com")
	inv, _ := h.DB.CreateInvite(p.ID, "a@t.com")
	req := httptest.NewRequest("GET", "/invite/"+inv.Token, nil)
	req.SetPathValue("token", inv.Token)
	req = withUser(req, "B", "b@t.com")
	w := httptest.NewRecorder()
	h.handleAcceptInvite(w, req)
	if w.Code != 500 {
		t.Errorf("expected 500, got %d", w.Code)
	}
}

// --- handleListProjects/handleHome user-scoped error ---

func TestHandleListProjectsUserScopedDBError(t *testing.T) {
	h := mockHandler(t, func(m *mockDB) { m.listProjectsForUserErr = errDB })
	req := httptest.NewRequest("GET", "/api/projects", nil)
	req = withUser(req, "A", "a@t.com")
	w := httptest.NewRecorder()
	h.handleListProjects(w, req)
	if w.Code != 500 {
		t.Errorf("expected 500, got %d", w.Code)
	}
}

func TestHandleHomeUserScopedDBError(t *testing.T) {
	h := mockHandler(t, func(m *mockDB) { m.listProjectsForUserErr = errDB })
	h.TemplatesDir = "../../web/templates"
	req := httptest.NewRequest("GET", "/", nil)
	req = withUser(req, "A", "a@t.com")
	w := httptest.NewRecorder()
	h.handleHome(w, req)
	if w.Code != 500 {
		t.Errorf("expected 500, got %d", w.Code)
	}
}

// --- projectAccess DB error ---

func TestProjectAccessMiddlewareDBError(t *testing.T) {
	h := mockHandler(t, func(m *mockDB) { m.canAccessProjectErr = errDB })
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(200) })
	handler := h.projectAccess(inner)
	req := httptest.NewRequest("GET", "/projects/x", nil)
	req.SetPathValue("id", "x")
	req = withUser(req, "A", "a@t.com")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	if w.Code != 404 {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

// --- Phase 13: XSS fix ---

func TestHandleListMembersHTMLInEmail(t *testing.T) {
	h := setupTestHandler(t)
	p, _ := h.DB.CreateProject("proj", "alice@test.com")
	xss := `<img src=x onerror=alert(1)>`
	h.DB.AddMember(p.ID, xss)

	req := httptest.NewRequest("GET", "/api/projects/"+p.ID+"/members", nil)
	req.SetPathValue("id", p.ID)
	w := httptest.NewRecorder()
	h.handleListMembers(w, req)

	if w.Code != 200 {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var members []map[string]string
	json.NewDecoder(w.Body).Decode(&members)
	if len(members) != 1 {
		t.Fatalf("expected 1 member, got %d", len(members))
	}
	// API returns raw email in JSON — escaping is client-side
	if members[0]["email"] != xss {
		t.Errorf("email = %q, want %q", members[0]["email"], xss)
	}
}

// Unused import guard
var _ = context.Background
