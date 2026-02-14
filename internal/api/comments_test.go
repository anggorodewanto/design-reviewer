package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/ab/design-reviewer/internal/auth"
	"github.com/ab/design-reviewer/internal/db"
)

// mockDB embeds a real DataStore and allows overriding specific methods to inject errors.
type mockDB struct {
	DataStore
	getUnresolvedErr           error
	getCommentsErr             error
	getRepliesErr              error
	createCommentErr           error
	createReplyErr             error
	toggleResolveErr           error
	toggleResolveResult        bool
	listVersionsErr            error
	listProjectsWithVCErr      error
	updateProjectStatusErr     error
	getProjectByNameErr        error
	createProjectErr           error
	createVersionErr           error
	getProjectErr              error
	getVersionErr              error
	getLatestVersionErr        error
	createTokenErr             error
	canAccessProjectErr        error
	canAccessProjectResult     *bool
	getProjectOwnerErr         error
	getProjectOwnerResult      string
	createInviteErr            error
	getInviteByTokenErr        error
	deleteInviteErr            error
	addMemberErr               error
	listMembersErr             error
	removeMemberErr            error
	listProjectsForUserErr     error
	moveCommentErr             error
	getCommentErr              error
}

func (m *mockDB) GetUnresolvedCommentsUpTo(versionID string) ([]db.Comment, error) {
	if m.getUnresolvedErr != nil {
		return nil, m.getUnresolvedErr
	}
	return m.DataStore.GetUnresolvedCommentsUpTo(versionID)
}

func (m *mockDB) GetCommentsForVersion(versionID string) ([]db.Comment, error) {
	if m.getCommentsErr != nil {
		return nil, m.getCommentsErr
	}
	return m.DataStore.GetCommentsForVersion(versionID)
}

func (m *mockDB) GetReplies(commentID string) ([]db.Reply, error) {
	if m.getRepliesErr != nil {
		return nil, m.getRepliesErr
	}
	return m.DataStore.GetReplies(commentID)
}

func (m *mockDB) CreateComment(versionID, page string, xPct, yPct float64, authorName, authorEmail, body string) (*db.Comment, error) {
	if m.createCommentErr != nil {
		return nil, m.createCommentErr
	}
	return m.DataStore.CreateComment(versionID, page, xPct, yPct, authorName, authorEmail, body)
}

func (m *mockDB) CreateReply(commentID, authorName, authorEmail, body string) (*db.Reply, error) {
	if m.createReplyErr != nil {
		return nil, m.createReplyErr
	}
	return m.DataStore.CreateReply(commentID, authorName, authorEmail, body)
}

func (m *mockDB) ToggleResolve(commentID string) (bool, error) {
	if m.toggleResolveErr != nil {
		return false, m.toggleResolveErr
	}
	return m.DataStore.ToggleResolve(commentID)
}

func (m *mockDB) ListVersions(projectID string) ([]db.Version, error) {
	if m.listVersionsErr != nil {
		return nil, m.listVersionsErr
	}
	return m.DataStore.ListVersions(projectID)
}

func (m *mockDB) ListProjectsWithVersionCount() ([]db.ProjectWithVersionCount, error) {
	if m.listProjectsWithVCErr != nil {
		return nil, m.listProjectsWithVCErr
	}
	return m.DataStore.ListProjectsWithVersionCount()
}

func (m *mockDB) UpdateProjectStatus(id, status string) error {
	if m.updateProjectStatusErr != nil {
		return m.updateProjectStatusErr
	}
	return m.DataStore.UpdateProjectStatus(id, status)
}

func (m *mockDB) GetProjectByName(name string) (*db.Project, error) {
	if m.getProjectByNameErr != nil {
		return nil, m.getProjectByNameErr
	}
	return m.DataStore.GetProjectByName(name)
}

func (m *mockDB) CreateProject(name, ownerEmail string) (*db.Project, error) {
	if m.createProjectErr != nil {
		return nil, m.createProjectErr
	}
	return m.DataStore.CreateProject(name, ownerEmail)
}

func (m *mockDB) CreateVersion(projectID, storagePath string) (*db.Version, error) {
	if m.createVersionErr != nil {
		return nil, m.createVersionErr
	}
	return m.DataStore.CreateVersion(projectID, storagePath)
}

func (m *mockDB) GetProject(id string) (*db.Project, error) {
	if m.getProjectErr != nil {
		return nil, m.getProjectErr
	}
	return m.DataStore.GetProject(id)
}

func (m *mockDB) GetVersion(id string) (*db.Version, error) {
	if m.getVersionErr != nil {
		return nil, m.getVersionErr
	}
	return m.DataStore.GetVersion(id)
}

func (m *mockDB) GetLatestVersion(projectID string) (*db.Version, error) {
	if m.getLatestVersionErr != nil {
		return nil, m.getLatestVersionErr
	}
	return m.DataStore.GetLatestVersion(projectID)
}

func (m *mockDB) CreateToken(token, userName, userEmail string) error {
	if m.createTokenErr != nil {
		return m.createTokenErr
	}
	return m.DataStore.CreateToken(token, userName, userEmail)
}

func (m *mockDB) CanAccessProject(projectID, email string) (bool, error) {
	if m.canAccessProjectErr != nil {
		return false, m.canAccessProjectErr
	}
	if m.canAccessProjectResult != nil {
		return *m.canAccessProjectResult, nil
	}
	return m.DataStore.CanAccessProject(projectID, email)
}

func (m *mockDB) GetProjectOwner(projectID string) (string, error) {
	if m.getProjectOwnerErr != nil {
		return "", m.getProjectOwnerErr
	}
	if m.getProjectOwnerResult != "" {
		return m.getProjectOwnerResult, nil
	}
	return m.DataStore.GetProjectOwner(projectID)
}

func (m *mockDB) CreateInvite(projectID, createdBy string) (*db.ProjectInvite, error) {
	if m.createInviteErr != nil {
		return nil, m.createInviteErr
	}
	return m.DataStore.CreateInvite(projectID, createdBy)
}

func (m *mockDB) GetInviteByToken(token string) (*db.ProjectInvite, error) {
	if m.getInviteByTokenErr != nil {
		return nil, m.getInviteByTokenErr
	}
	return m.DataStore.GetInviteByToken(token)
}

func (m *mockDB) DeleteInvite(id string) error {
	if m.deleteInviteErr != nil {
		return m.deleteInviteErr
	}
	return m.DataStore.DeleteInvite(id)
}

func (m *mockDB) AddMember(projectID, email string) error {
	if m.addMemberErr != nil {
		return m.addMemberErr
	}
	return m.DataStore.AddMember(projectID, email)
}

func (m *mockDB) ListMembers(projectID string) ([]db.ProjectMember, error) {
	if m.listMembersErr != nil {
		return nil, m.listMembersErr
	}
	return m.DataStore.ListMembers(projectID)
}

func (m *mockDB) RemoveMember(projectID, email string) error {
	if m.removeMemberErr != nil {
		return m.removeMemberErr
	}
	return m.DataStore.RemoveMember(projectID, email)
}

func (m *mockDB) ListProjectsWithVersionCountForUser(email string) ([]db.ProjectWithVersionCount, error) {
	if m.listProjectsForUserErr != nil {
		return nil, m.listProjectsForUserErr
	}
	return m.DataStore.ListProjectsWithVersionCountForUser(email)
}

func (m *mockDB) MoveComment(id string, x, y float64) error {
	if m.moveCommentErr != nil {
		return m.moveCommentErr
	}
	return m.DataStore.MoveComment(id, x, y)
}

func (m *mockDB) GetComment(id string) (*db.Comment, error) {
	if m.getCommentErr != nil {
		return nil, m.getCommentErr
	}
	return m.DataStore.GetComment(id)
}

var errDB = errors.New("db failure")

func TestHandleGetCommentsEmpty(t *testing.T) {
	h := setupTestHandler(t)
	_, vid := seedProject(t, h, map[string]string{"index.html": "x"})

	req := httptest.NewRequest("GET", "/api/versions/"+vid+"/comments", nil)
	req.SetPathValue("id", vid)
	w := httptest.NewRecorder()
	h.handleGetComments(w, req)

	if w.Code != 200 {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var result []commentJSON
	json.NewDecoder(w.Body).Decode(&result)
	if len(result) != 0 {
		t.Errorf("expected empty, got %d", len(result))
	}
}

func TestHandleCreateComment(t *testing.T) {
	h := setupTestHandler(t)
	_, vid := seedProject(t, h, map[string]string{"index.html": "x"})

	body := `{"page":"index.html","x_percent":10.5,"y_percent":20.3,"author_name":"Alice","author_email":"alice@test.com","body":"looks good"}`
	req := httptest.NewRequest("POST", "/api/versions/"+vid+"/comments", strings.NewReader(body))
	req.SetPathValue("id", vid)
	w := httptest.NewRecorder()
	h.handleCreateComment(w, req)

	if w.Code != 201 {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}
	var c commentJSON
	json.NewDecoder(w.Body).Decode(&c)
	if c.Body != "looks good" {
		t.Errorf("body = %q, want %q", c.Body, "looks good")
	}
	if c.Page != "index.html" {
		t.Errorf("page = %q, want %q", c.Page, "index.html")
	}
	if c.XPercent != 10.5 || c.YPercent != 20.3 {
		t.Errorf("coords = (%v, %v), want (10.5, 20.3)", c.XPercent, c.YPercent)
	}
	if c.Resolved {
		t.Error("new comment should not be resolved")
	}
	if len(c.Replies) != 0 {
		t.Error("new comment should have no replies")
	}
}

func TestHandleCreateCommentMissingBody(t *testing.T) {
	h := setupTestHandler(t)
	_, vid := seedProject(t, h, map[string]string{"index.html": "x"})

	body := `{"page":"index.html","x_percent":10,"y_percent":20}`
	req := httptest.NewRequest("POST", "/api/versions/"+vid+"/comments", strings.NewReader(body))
	req.SetPathValue("id", vid)
	w := httptest.NewRecorder()
	h.handleCreateComment(w, req)

	if w.Code != 400 {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestHandleCreateCommentInvalidJSON(t *testing.T) {
	h := setupTestHandler(t)
	_, vid := seedProject(t, h, map[string]string{"index.html": "x"})

	req := httptest.NewRequest("POST", "/api/versions/"+vid+"/comments", strings.NewReader("not json"))
	req.SetPathValue("id", vid)
	w := httptest.NewRecorder()
	h.handleCreateComment(w, req)

	if w.Code != 400 {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestHandleGetCommentsWithReplies(t *testing.T) {
	h := setupTestHandler(t)
	_, vid := seedProject(t, h, map[string]string{"index.html": "x"})

	c, _ := h.DB.CreateComment(vid, "index.html", 10, 20, "Alice", "a@t.com", "hello")
	h.DB.CreateReply(c.ID, "Bob", "b@t.com", "reply1")

	req := httptest.NewRequest("GET", "/api/versions/"+vid+"/comments", nil)
	req.SetPathValue("id", vid)
	w := httptest.NewRecorder()
	h.handleGetComments(w, req)

	var result []commentJSON
	json.NewDecoder(w.Body).Decode(&result)
	if len(result) != 1 {
		t.Fatalf("expected 1 comment, got %d", len(result))
	}
	if len(result[0].Replies) != 1 {
		t.Fatalf("expected 1 reply, got %d", len(result[0].Replies))
	}
	if result[0].Replies[0].Body != "reply1" {
		t.Errorf("reply body = %q, want %q", result[0].Replies[0].Body, "reply1")
	}
}

func TestHandleCreateReply(t *testing.T) {
	h := setupTestHandler(t)
	_, vid := seedProject(t, h, map[string]string{"index.html": "x"})
	c, _ := h.DB.CreateComment(vid, "index.html", 10, 20, "Alice", "a@t.com", "hello")

	body := `{"author_name":"Bob","author_email":"b@t.com","body":"nice"}`
	req := httptest.NewRequest("POST", "/api/comments/"+c.ID+"/replies", strings.NewReader(body))
	req.SetPathValue("id", c.ID)
	w := httptest.NewRecorder()
	h.handleCreateReply(w, req)

	if w.Code != 201 {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}
	var r replyJSON
	json.NewDecoder(w.Body).Decode(&r)
	if r.Body != "nice" {
		t.Errorf("body = %q, want %q", r.Body, "nice")
	}
	if r.AuthorName != "Bob" {
		t.Errorf("author = %q, want %q", r.AuthorName, "Bob")
	}
}

func TestHandleCreateReplyMissingBody(t *testing.T) {
	h := setupTestHandler(t)
	_, vid := seedProject(t, h, map[string]string{"index.html": "x"})
	c, _ := h.DB.CreateComment(vid, "index.html", 10, 20, "Alice", "a@t.com", "hello")

	req := httptest.NewRequest("POST", "/api/comments/"+c.ID+"/replies", strings.NewReader(`{"author_name":"Bob"}`))
	req.SetPathValue("id", c.ID)
	w := httptest.NewRecorder()
	h.handleCreateReply(w, req)

	if w.Code != 400 {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestHandleToggleResolve(t *testing.T) {
	h := setupTestHandler(t)
	_, vid := seedProject(t, h, map[string]string{"index.html": "x"})
	c, _ := h.DB.CreateComment(vid, "index.html", 10, 20, "Alice", "a@t.com", "hello")

	// Resolve
	req := httptest.NewRequest("PATCH", "/api/comments/"+c.ID+"/resolve", nil)
	req.SetPathValue("id", c.ID)
	w := httptest.NewRecorder()
	h.handleToggleResolve(w, req)

	if w.Code != 200 {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var res map[string]bool
	json.NewDecoder(w.Body).Decode(&res)
	if !res["resolved"] {
		t.Error("expected resolved=true after first toggle")
	}

	// Unresolve
	req = httptest.NewRequest("PATCH", "/api/comments/"+c.ID+"/resolve", nil)
	req.SetPathValue("id", c.ID)
	w = httptest.NewRecorder()
	h.handleToggleResolve(w, req)

	json.NewDecoder(w.Body).Decode(&res)
	if res["resolved"] {
		t.Error("expected resolved=false after second toggle")
	}
}

func TestHandleToggleResolveNotFound(t *testing.T) {
	h := setupTestHandler(t)

	req := httptest.NewRequest("PATCH", "/api/comments/nonexistent/resolve", nil)
	req.SetPathValue("id", "nonexistent")
	w := httptest.NewRecorder()
	h.handleToggleResolve(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

func TestHandleGetCommentsCarryOver(t *testing.T) {
	h := setupTestHandler(t)
	p, _ := h.DB.CreateProject("carry-proj", "")
	v1, _ := h.DB.CreateVersion(p.ID, "/tmp/v1")
	v2, _ := h.DB.CreateVersion(p.ID, "/tmp/v2")

	// Create unresolved comment on v1
	h.DB.CreateComment(v1.ID, "index.html", 10, 20, "Alice", "a@t.com", "unresolved on v1")
	// Create resolved comment on v1
	resolved, _ := h.DB.CreateComment(v1.ID, "index.html", 30, 40, "Bob", "b@t.com", "resolved on v1")
	h.DB.ToggleResolve(resolved.ID)

	// GET comments for v2 should include unresolved from v1 but NOT resolved from v1
	req := httptest.NewRequest("GET", "/api/versions/"+v2.ID+"/comments", nil)
	req.SetPathValue("id", v2.ID)
	w := httptest.NewRecorder()
	h.handleGetComments(w, req)

	var result []commentJSON
	json.NewDecoder(w.Body).Decode(&result)
	if len(result) != 1 {
		t.Fatalf("expected 1 comment (unresolved carry-over), got %d", len(result))
	}
	if result[0].Body != "unresolved on v1" {
		t.Errorf("expected carried-over comment, got %q", result[0].Body)
	}
}

func TestHandleGetCommentsResolvedOnCurrentVersion(t *testing.T) {
	h := setupTestHandler(t)
	p, _ := h.DB.CreateProject("resolved-proj", "")
	v1, _ := h.DB.CreateVersion(p.ID, "/tmp/v1")

	// Create and resolve a comment on v1
	c, _ := h.DB.CreateComment(v1.ID, "index.html", 10, 20, "Alice", "a@t.com", "resolved here")
	h.DB.ToggleResolve(c.ID)

	// GET comments for v1 should include the resolved comment
	req := httptest.NewRequest("GET", "/api/versions/"+v1.ID+"/comments", nil)
	req.SetPathValue("id", v1.ID)
	w := httptest.NewRecorder()
	h.handleGetComments(w, req)

	var result []commentJSON
	json.NewDecoder(w.Body).Decode(&result)
	if len(result) != 1 {
		t.Fatalf("expected 1 comment (resolved on this version), got %d", len(result))
	}
	if !result[0].Resolved {
		t.Error("expected comment to be resolved")
	}
}

func TestHandleCreateCommentMissingPage(t *testing.T) {
	h := setupTestHandler(t)
	_, vid := seedProject(t, h, map[string]string{"index.html": "x"})

	body := `{"body":"hello","x_percent":10,"y_percent":20}`
	req := httptest.NewRequest("POST", "/api/versions/"+vid+"/comments", strings.NewReader(body))
	req.SetPathValue("id", vid)
	w := httptest.NewRecorder()
	h.handleCreateComment(w, req)

	if w.Code != 400 {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestHandleCreateReplyInvalidJSON(t *testing.T) {
	h := setupTestHandler(t)
	_, vid := seedProject(t, h, map[string]string{"index.html": "x"})
	c, _ := h.DB.CreateComment(vid, "index.html", 10, 20, "Alice", "a@t.com", "hello")

	req := httptest.NewRequest("POST", "/api/comments/"+c.ID+"/replies", strings.NewReader("bad json"))
	req.SetPathValue("id", c.ID)
	w := httptest.NewRecorder()
	h.handleCreateReply(w, req)

	if w.Code != 400 {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestHandleGetCommentsMultipleWithFilter(t *testing.T) {
	h := setupTestHandler(t)
	_, vid := seedProject(t, h, map[string]string{"index.html": "x", "about.html": "y"})

	// Create comments on different pages
	h.DB.CreateComment(vid, "index.html", 10, 20, "Alice", "a@t.com", "on index")
	h.DB.CreateComment(vid, "about.html", 30, 40, "Bob", "b@t.com", "on about")
	c3, _ := h.DB.CreateComment(vid, "index.html", 50, 60, "Carol", "c@t.com", "resolved one")
	h.DB.ToggleResolve(c3.ID)

	req := httptest.NewRequest("GET", "/api/versions/"+vid+"/comments", nil)
	req.SetPathValue("id", vid)
	w := httptest.NewRecorder()
	h.handleGetComments(w, req)

	var result []commentJSON
	json.NewDecoder(w.Body).Decode(&result)
	if len(result) != 3 {
		t.Fatalf("expected 3 comments, got %d", len(result))
	}

	// Verify resolved comment is included
	var foundResolved bool
	for _, c := range result {
		if c.Resolved {
			foundResolved = true
		}
	}
	if !foundResolved {
		t.Error("expected to find resolved comment in results")
	}
}

func TestHandleGetCommentsResponseFormat(t *testing.T) {
	h := setupTestHandler(t)
	_, vid := seedProject(t, h, map[string]string{"index.html": "x"})
	h.DB.CreateComment(vid, "index.html", 45.2, 30.1, "Jane", "jane@co.com", "needs padding")

	req := httptest.NewRequest("GET", "/api/versions/"+vid+"/comments", nil)
	req.SetPathValue("id", vid)
	w := httptest.NewRecorder()
	h.handleGetComments(w, req)

	if ct := w.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", ct)
	}

	var result []commentJSON
	json.NewDecoder(w.Body).Decode(&result)
	c := result[0]
	if c.AuthorName != "Jane" || c.AuthorEmail != "jane@co.com" {
		t.Errorf("author mismatch: %q / %q", c.AuthorName, c.AuthorEmail)
	}
	if c.CreatedAt == "" {
		t.Error("missing created_at")
	}
	if c.Replies == nil {
		t.Error("replies should be non-nil (empty array)")
	}
}

// --- Mock-based error path tests ---

func mockHandler(t *testing.T, overrides func(*mockDB)) *Handler {
	t.Helper()
	h := setupTestHandler(t)
	m := &mockDB{DataStore: h.DB}
	overrides(m)
	h.DB = m
	return h
}

func TestGetCommentsErrUnresolved(t *testing.T) {
	h := mockHandler(t, func(m *mockDB) { m.getUnresolvedErr = errDB })
	req := httptest.NewRequest("GET", "/api/versions/x/comments", nil)
	req.SetPathValue("id", "x")
	w := httptest.NewRecorder()
	h.handleGetComments(w, req)
	if w.Code != 500 {
		t.Errorf("expected 500, got %d", w.Code)
	}
}

func TestGetCommentsErrForVersion(t *testing.T) {
	h := mockHandler(t, func(m *mockDB) { m.getCommentsErr = errDB })
	_, vid := seedProject(t, h, map[string]string{"index.html": "x"})
	// Need to set the real DB back for seedProject, then swap mock
	// Actually seedProject already ran with real DB embedded in mock. The mock only intercepts GetCommentsForVersion.
	req := httptest.NewRequest("GET", "/api/versions/"+vid+"/comments", nil)
	req.SetPathValue("id", vid)
	w := httptest.NewRecorder()
	h.handleGetComments(w, req)
	if w.Code != 500 {
		t.Errorf("expected 500, got %d", w.Code)
	}
}

func TestGetCommentsErrReplies(t *testing.T) {
	h := setupTestHandler(t)
	_, vid := seedProject(t, h, map[string]string{"index.html": "x"})
	h.DB.CreateComment(vid, "index.html", 10, 20, "A", "a@t.com", "hi")
	m := &mockDB{DataStore: h.DB, getRepliesErr: errDB}
	h.DB = m

	req := httptest.NewRequest("GET", "/api/versions/"+vid+"/comments", nil)
	req.SetPathValue("id", vid)
	w := httptest.NewRecorder()
	h.handleGetComments(w, req)
	if w.Code != 500 {
		t.Errorf("expected 500, got %d", w.Code)
	}
}

func TestCreateCommentErrDB(t *testing.T) {
	h := mockHandler(t, func(m *mockDB) { m.createCommentErr = errDB })
	body := `{"page":"index.html","x_percent":10,"y_percent":20,"body":"hi"}`
	req := httptest.NewRequest("POST", "/api/versions/x/comments", strings.NewReader(body))
	req.SetPathValue("id", "x")
	w := httptest.NewRecorder()
	h.handleCreateComment(w, req)
	if w.Code != 500 {
		t.Errorf("expected 500, got %d", w.Code)
	}
}

func TestCreateReplyErrDB(t *testing.T) {
	h := mockHandler(t, func(m *mockDB) { m.createReplyErr = errDB })
	req := httptest.NewRequest("POST", "/api/comments/x/replies", strings.NewReader(`{"body":"hi"}`))
	req.SetPathValue("id", "x")
	w := httptest.NewRecorder()
	h.handleCreateReply(w, req)
	if w.Code != 500 {
		t.Errorf("expected 500, got %d", w.Code)
	}
}

func TestToggleResolveErrDB(t *testing.T) {
	h := mockHandler(t, func(m *mockDB) { m.toggleResolveErr = errDB })
	req := httptest.NewRequest("PATCH", "/api/comments/x/resolve", nil)
	req.SetPathValue("id", "x")
	w := httptest.NewRecorder()
	h.handleToggleResolve(w, req)
	if w.Code != 500 {
		t.Errorf("expected 500, got %d", w.Code)
	}
}

// --- Phase 20: Move Comment ---

func TestHandleMoveComment(t *testing.T) {
	h := setupTestHandler(t)
	_, vid := seedProject(t, h, map[string]string{"index.html": "x"})
	c, _ := h.DB.CreateComment(vid, "index.html", 10, 20, "A", "a@t.com", "hi")

	body := `{"x_percent":55.5,"y_percent":77.3}`
	req := httptest.NewRequest("PATCH", "/api/comments/"+c.ID+"/move", strings.NewReader(body))
	req.SetPathValue("id", c.ID)
	w := httptest.NewRecorder()
	h.handleMoveComment(w, req)

	if w.Code != 200 {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var res map[string]bool
	json.NewDecoder(w.Body).Decode(&res)
	if !res["ok"] {
		t.Error("expected ok=true")
	}

	// Verify coordinates updated
	comments, _ := h.DB.GetCommentsForVersion(vid)
	for _, cm := range comments {
		if cm.ID == c.ID {
			if cm.XPercent != 55.5 || cm.YPercent != 77.3 {
				t.Errorf("coords = (%v, %v), want (55.5, 77.3)", cm.XPercent, cm.YPercent)
			}
		}
	}
}

func TestHandleMoveCommentInvalidJSON(t *testing.T) {
	h := setupTestHandler(t)
	req := httptest.NewRequest("PATCH", "/api/comments/x/move", strings.NewReader("bad"))
	req.SetPathValue("id", "x")
	w := httptest.NewRecorder()
	h.handleMoveComment(w, req)
	if w.Code != 400 {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestHandleMoveCommentOutOfRange(t *testing.T) {
	h := setupTestHandler(t)
	tests := []struct {
		name string
		body string
	}{
		{"x too high", `{"x_percent":101,"y_percent":50}`},
		{"y too high", `{"x_percent":50,"y_percent":101}`},
		{"x negative", `{"x_percent":-1,"y_percent":50}`},
		{"y negative", `{"x_percent":50,"y_percent":-1}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("PATCH", "/api/comments/x/move", strings.NewReader(tt.body))
			req.SetPathValue("id", "x")
			w := httptest.NewRecorder()
			h.handleMoveComment(w, req)
			if w.Code != 400 {
				t.Errorf("expected 400, got %d", w.Code)
			}
		})
	}
}

func TestHandleMoveCommentBoundary(t *testing.T) {
	h := setupTestHandler(t)
	_, vid := seedProject(t, h, map[string]string{"index.html": "x"})
	c, _ := h.DB.CreateComment(vid, "index.html", 10, 20, "A", "a@t.com", "hi")

	// Exactly 0 and 100 should be valid
	body := `{"x_percent":0,"y_percent":100}`
	req := httptest.NewRequest("PATCH", "/api/comments/"+c.ID+"/move", strings.NewReader(body))
	req.SetPathValue("id", c.ID)
	w := httptest.NewRecorder()
	h.handleMoveComment(w, req)
	if w.Code != 200 {
		t.Errorf("expected 200 for boundary values, got %d", w.Code)
	}
}

func TestMoveCommentErrDB(t *testing.T) {
	h := mockHandler(t, func(m *mockDB) { m.moveCommentErr = errDB })
	body := `{"x_percent":50,"y_percent":50}`
	req := httptest.NewRequest("PATCH", "/api/comments/x/move", strings.NewReader(body))
	req.SetPathValue("id", "x")
	w := httptest.NewRecorder()
	h.handleMoveComment(w, req)
	if w.Code != 500 {
		t.Errorf("expected 500, got %d", w.Code)
	}
}

// --- Phase 21: commentAccess middleware ---

func TestCommentAccessNoEmail(t *testing.T) {
	h := setupTestHandler(t)
	called := false
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { called = true })
	req := httptest.NewRequest("POST", "/api/comments/x/replies", nil)
	req.SetPathValue("id", "x")
	w := httptest.NewRecorder()
	h.commentAccess(inner).ServeHTTP(w, req)
	if w.Code != 404 || called {
		t.Errorf("expected 404 and inner not called, got %d called=%v", w.Code, called)
	}
}

func TestCommentAccessInvalidComment(t *testing.T) {
	h := setupTestHandler(t)
	called := false
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { called = true })
	req := httptest.NewRequest("POST", "/api/comments/nonexistent/replies", nil)
	req.SetPathValue("id", "nonexistent")
	ctx := auth.SetUserInContext(req.Context(), "Alice", "alice@test.com")
	w := httptest.NewRecorder()
	h.commentAccess(inner).ServeHTTP(w, req.WithContext(ctx))
	if w.Code != 404 || called {
		t.Errorf("expected 404, got %d called=%v", w.Code, called)
	}
}

func TestCommentAccessNoProjectAccess(t *testing.T) {
	h := setupTestHandler(t)
	p, _ := h.DB.CreateProject("priv", "owner@test.com")
	v, _ := h.DB.CreateVersion(p.ID, "/tmp/v")
	c, _ := h.DB.CreateComment(v.ID, "index.html", 10, 20, "A", "a@t.com", "hi")

	called := false
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { called = true })
	req := httptest.NewRequest("POST", "/api/comments/"+c.ID+"/replies", nil)
	req.SetPathValue("id", c.ID)
	ctx := auth.SetUserInContext(req.Context(), "Stranger", "stranger@test.com")
	w := httptest.NewRecorder()
	h.commentAccess(inner).ServeHTTP(w, req.WithContext(ctx))
	if w.Code != 404 || called {
		t.Errorf("expected 404, got %d called=%v", w.Code, called)
	}
}

func TestCommentAccessGranted(t *testing.T) {
	h := setupTestHandler(t)
	p, _ := h.DB.CreateProject("pub", "")
	v, _ := h.DB.CreateVersion(p.ID, "/tmp/v")
	c, _ := h.DB.CreateComment(v.ID, "index.html", 10, 20, "A", "a@t.com", "hi")

	called := false
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { called = true; w.WriteHeader(200) })
	req := httptest.NewRequest("POST", "/api/comments/"+c.ID+"/replies", nil)
	req.SetPathValue("id", c.ID)
	ctx := auth.SetUserInContext(req.Context(), "Alice", "alice@test.com")
	w := httptest.NewRecorder()
	h.commentAccess(inner).ServeHTTP(w, req.WithContext(ctx))
	if w.Code != 200 || !called {
		t.Errorf("expected 200 and inner called, got %d called=%v", w.Code, called)
	}
}

func TestCommentAccessGetCommentDBError(t *testing.T) {
	h := mockHandler(t, func(m *mockDB) { m.getCommentErr = errDB })
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { t.Error("should not be called") })
	req := httptest.NewRequest("POST", "/api/comments/x/replies", nil)
	req.SetPathValue("id", "x")
	ctx := auth.SetUserInContext(req.Context(), "A", "a@t.com")
	w := httptest.NewRecorder()
	h.commentAccess(inner).ServeHTTP(w, req.WithContext(ctx))
	if w.Code != 404 {
		t.Errorf("expected 404, got %d", w.Code)
	}
}
