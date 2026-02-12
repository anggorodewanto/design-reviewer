package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/ab/design-reviewer/internal/db"
)

// mockDB embeds a real DataStore and allows overriding specific methods to inject errors.
type mockDB struct {
	DataStore
	getUnresolvedErr    error
	getCommentsErr      error
	getRepliesErr       error
	createCommentErr    error
	createReplyErr      error
	toggleResolveErr    error
	toggleResolveResult bool
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
	p, _ := h.DB.CreateProject("carry-proj")
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
	p, _ := h.DB.CreateProject("resolved-proj")
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
