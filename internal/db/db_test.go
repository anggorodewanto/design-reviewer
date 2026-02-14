package db

import (
	"database/sql"
	"path/filepath"
	"testing"
)

func newTestDB(t *testing.T) *DB {
	t.Helper()
	d, err := New(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { d.Close() })
	return d
}

func TestGetProject(t *testing.T) {
	d := newTestDB(t)
	p, _ := d.CreateProject("gp", "")
	got, err := d.GetProject(p.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Name != "gp" {
		t.Errorf("name = %q, want gp", got.Name)
	}
}

func TestGetProjectNotFound(t *testing.T) {
	d := newTestDB(t)
	_, err := d.GetProject("nonexistent")
	if err != sql.ErrNoRows {
		t.Errorf("expected ErrNoRows, got %v", err)
	}
}

func TestGetProjectByName(t *testing.T) {
	d := newTestDB(t)
	p, _ := d.CreateProject("byname", "")
	got, err := d.GetProjectByName("byname")
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != p.ID {
		t.Errorf("id mismatch")
	}
}

func TestGetProjectByNameNotFound(t *testing.T) {
	d := newTestDB(t)
	_, err := d.GetProjectByName("nope")
	if err != sql.ErrNoRows {
		t.Errorf("expected ErrNoRows, got %v", err)
	}
}

func TestListProjects(t *testing.T) {
	d := newTestDB(t)
	d.CreateProject("a", "")
	d.CreateProject("b", "")
	projects, err := d.ListProjects()
	if err != nil {
		t.Fatal(err)
	}
	if len(projects) != 2 {
		t.Errorf("expected 2, got %d", len(projects))
	}
}

func TestListProjectsEmpty(t *testing.T) {
	d := newTestDB(t)
	projects, err := d.ListProjects()
	if err != nil {
		t.Fatal(err)
	}
	if len(projects) != 0 {
		t.Errorf("expected 0, got %d", len(projects))
	}
}

func TestUpdateProjectStatus(t *testing.T) {
	d := newTestDB(t)
	p, _ := d.CreateProject("st", "")
	if err := d.UpdateProjectStatus(p.ID, "in_review"); err != nil {
		t.Fatal(err)
	}
	got, _ := d.GetProject(p.ID)
	if got.Status != "in_review" {
		t.Errorf("status = %q, want in_review", got.Status)
	}
}

func TestUpdateProjectStatusInvalid(t *testing.T) {
	d := newTestDB(t)
	p, _ := d.CreateProject("st2", "")
	if err := d.UpdateProjectStatus(p.ID, "bogus"); err == nil {
		t.Error("expected error for invalid status")
	}
}

func TestUpdateProjectStatusNotFound(t *testing.T) {
	d := newTestDB(t)
	err := d.UpdateProjectStatus("nonexistent", "draft")
	if err != sql.ErrNoRows {
		t.Errorf("expected ErrNoRows, got %v", err)
	}
}

func TestGetVersion(t *testing.T) {
	d := newTestDB(t)
	p, _ := d.CreateProject("vp", "")
	v, _ := d.CreateVersion(p.ID, "/path")
	got, err := d.GetVersion(v.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.VersionNum != 1 || got.ProjectID != p.ID {
		t.Errorf("unexpected version: %+v", got)
	}
}

func TestGetVersionNotFound(t *testing.T) {
	d := newTestDB(t)
	_, err := d.GetVersion("nonexistent")
	if err != sql.ErrNoRows {
		t.Errorf("expected ErrNoRows, got %v", err)
	}
}

func TestGetLatestVersion(t *testing.T) {
	d := newTestDB(t)
	p, _ := d.CreateProject("lv", "")
	d.CreateVersion(p.ID, "/v1")
	d.CreateVersion(p.ID, "/v2")
	got, err := d.GetLatestVersion(p.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.VersionNum != 2 {
		t.Errorf("expected v2, got v%d", got.VersionNum)
	}
}

func TestGetLatestVersionNotFound(t *testing.T) {
	d := newTestDB(t)
	p, _ := d.CreateProject("nover", "")
	_, err := d.GetLatestVersion(p.ID)
	if err != sql.ErrNoRows {
		t.Errorf("expected ErrNoRows, got %v", err)
	}
}

func TestListProjectsWithVersionCountEmpty(t *testing.T) {
	d := newTestDB(t)
	projects, err := d.ListProjectsWithVersionCount()
	if err != nil {
		t.Fatal(err)
	}
	if len(projects) != 0 {
		t.Errorf("expected 0 projects, got %d", len(projects))
	}
}

func TestListProjectsWithVersionCountSingle(t *testing.T) {
	d := newTestDB(t)
	p, err := d.CreateProject("proj-a", "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := d.CreateVersion(p.ID, "/tmp/v1"); err != nil {
		t.Fatal(err)
	}
	if _, err := d.CreateVersion(p.ID, "/tmp/v2"); err != nil {
		t.Fatal(err)
	}

	projects, err := d.ListProjectsWithVersionCount()
	if err != nil {
		t.Fatal(err)
	}
	if len(projects) != 1 {
		t.Fatalf("expected 1 project, got %d", len(projects))
	}
	if projects[0].VersionCount != 2 {
		t.Errorf("expected version_count=2, got %d", projects[0].VersionCount)
	}
	if projects[0].Name != "proj-a" {
		t.Errorf("expected name=proj-a, got %s", projects[0].Name)
	}
}

func TestListProjectsWithVersionCountNoVersions(t *testing.T) {
	d := newTestDB(t)
	if _, err := d.CreateProject("empty-proj", ""); err != nil {
		t.Fatal(err)
	}

	projects, err := d.ListProjectsWithVersionCount()
	if err != nil {
		t.Fatal(err)
	}
	if len(projects) != 1 {
		t.Fatalf("expected 1 project, got %d", len(projects))
	}
	if projects[0].VersionCount != 0 {
		t.Errorf("expected version_count=0, got %d", projects[0].VersionCount)
	}
}

func TestListProjectsWithVersionCountMultiple(t *testing.T) {
	d := newTestDB(t)
	pa, _ := d.CreateProject("proj-a", "")
	pb, _ := d.CreateProject("proj-b", "")
	d.CreateVersion(pa.ID, "/tmp/v1")
	d.CreateVersion(pa.ID, "/tmp/v2")
	d.CreateVersion(pb.ID, "/tmp/v1")

	projects, err := d.ListProjectsWithVersionCount()
	if err != nil {
		t.Fatal(err)
	}
	if len(projects) != 2 {
		t.Fatalf("expected 2 projects, got %d", len(projects))
	}

	// Results ordered by updated_at DESC; both created at ~same time, check counts by name
	counts := map[string]int{}
	for _, p := range projects {
		counts[p.Name] = p.VersionCount
	}
	if counts["proj-a"] != 2 {
		t.Errorf("proj-a: expected 2 versions, got %d", counts["proj-a"])
	}
	if counts["proj-b"] != 1 {
		t.Errorf("proj-b: expected 1 version, got %d", counts["proj-b"])
	}
}

func TestListProjectsWithVersionCountOrderByUpdatedAt(t *testing.T) {
	d := newTestDB(t)
	// Create "older" first, then manually set its updated_at to the past
	p1, _ := d.CreateProject("older", "")
	d.CreateProject("newer", "")
	d.Exec(`UPDATE projects SET updated_at = datetime('now', '-1 hour') WHERE id = ?`, p1.ID)

	projects, err := d.ListProjectsWithVersionCount()
	if err != nil {
		t.Fatal(err)
	}
	if len(projects) != 2 {
		t.Fatalf("expected 2 projects, got %d", len(projects))
	}
	if projects[0].Name != "newer" {
		t.Errorf("expected first project to be 'newer', got %q", projects[0].Name)
	}
}

// --- Phase 5: Comment/Reply DB tests ---

func TestCreateCommentAndGet(t *testing.T) {
	d := newTestDB(t)
	p, _ := d.CreateProject("proj", "")
	v, _ := d.CreateVersion(p.ID, "/tmp/v1")

	c, err := d.CreateComment(v.ID, "index.html", 10.5, 20.3, "Alice", "a@t.com", "hello")
	if err != nil {
		t.Fatal(err)
	}
	if c.ID == "" || c.Body != "hello" || c.Resolved {
		t.Errorf("unexpected comment: %+v", c)
	}

	comments, err := d.GetCommentsForVersion(v.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(comments) != 1 {
		t.Fatalf("expected 1, got %d", len(comments))
	}
	if comments[0].XPercent != 10.5 || comments[0].YPercent != 20.3 {
		t.Errorf("coords mismatch: %v, %v", comments[0].XPercent, comments[0].YPercent)
	}
}

func TestToggleResolve(t *testing.T) {
	d := newTestDB(t)
	p, _ := d.CreateProject("proj", "")
	v, _ := d.CreateVersion(p.ID, "/tmp/v1")
	c, _ := d.CreateComment(v.ID, "index.html", 10, 20, "Alice", "a@t.com", "fix")

	resolved, err := d.ToggleResolve(c.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !resolved {
		t.Error("expected resolved=true")
	}

	resolved, _ = d.ToggleResolve(c.ID)
	if resolved {
		t.Error("expected resolved=false")
	}
}

func TestToggleResolveNotFound(t *testing.T) {
	d := newTestDB(t)
	_, err := d.ToggleResolve("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent comment")
	}
}

func TestCreateReplyAndGet(t *testing.T) {
	d := newTestDB(t)
	p, _ := d.CreateProject("proj", "")
	v, _ := d.CreateVersion(p.ID, "/tmp/v1")
	c, _ := d.CreateComment(v.ID, "index.html", 10, 20, "Alice", "a@t.com", "hello")

	r, err := d.CreateReply(c.ID, "Bob", "b@t.com", "reply")
	if err != nil {
		t.Fatal(err)
	}
	if r.Body != "reply" || r.AuthorName != "Bob" {
		t.Errorf("unexpected reply: %+v", r)
	}

	replies, err := d.GetReplies(c.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(replies) != 1 {
		t.Fatalf("expected 1 reply, got %d", len(replies))
	}
}

func TestGetUnresolvedCommentsUpTo(t *testing.T) {
	d := newTestDB(t)
	p, _ := d.CreateProject("proj", "")
	v1, _ := d.CreateVersion(p.ID, "/tmp/v1")
	v2, _ := d.CreateVersion(p.ID, "/tmp/v2")

	// Unresolved on v1
	d.CreateComment(v1.ID, "index.html", 10, 20, "Alice", "a@t.com", "unresolved")
	// Resolved on v1
	resolved, _ := d.CreateComment(v1.ID, "index.html", 30, 40, "Bob", "b@t.com", "resolved")
	d.ToggleResolve(resolved.ID)
	// Unresolved on v2
	d.CreateComment(v2.ID, "index.html", 50, 60, "Carol", "c@t.com", "new on v2")

	comments, err := d.GetUnresolvedCommentsUpTo(v2.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(comments) != 2 {
		t.Fatalf("expected 2 unresolved, got %d", len(comments))
	}

	// For v1, should only get the unresolved one
	comments1, _ := d.GetUnresolvedCommentsUpTo(v1.ID)
	if len(comments1) != 1 {
		t.Fatalf("expected 1 unresolved for v1, got %d", len(comments1))
	}
}

func TestGetRepliesEmpty(t *testing.T) {
	d := newTestDB(t)
	p, _ := d.CreateProject("proj", "")
	v, _ := d.CreateVersion(p.ID, "/tmp/v1")
	c, _ := d.CreateComment(v.ID, "index.html", 10, 20, "Alice", "a@t.com", "hello")

	replies, err := d.GetReplies(c.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(replies) != 0 {
		t.Errorf("expected 0 replies, got %d", len(replies))
	}
}

func TestGetRepliesOrder(t *testing.T) {
	d := newTestDB(t)
	p, _ := d.CreateProject("proj", "")
	v, _ := d.CreateVersion(p.ID, "/tmp/v1")
	c, _ := d.CreateComment(v.ID, "index.html", 10, 20, "Alice", "a@t.com", "hello")

	d.CreateReply(c.ID, "Bob", "b@t.com", "first")
	d.CreateReply(c.ID, "Carol", "c@t.com", "second")

	replies, _ := d.GetReplies(c.ID)
	if len(replies) != 2 {
		t.Fatalf("expected 2 replies, got %d", len(replies))
	}
	if replies[0].Body != "first" || replies[1].Body != "second" {
		t.Errorf("replies out of order: %q, %q", replies[0].Body, replies[1].Body)
	}
}

// --- Phase 6: Version History ---

func TestListVersionsEmpty(t *testing.T) {
	d := newTestDB(t)
	p, _ := d.CreateProject("empty", "")
	versions, err := d.ListVersions(p.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(versions) != 0 {
		t.Errorf("expected 0 versions, got %d", len(versions))
	}
}

func TestListVersionsOrdered(t *testing.T) {
	d := newTestDB(t)
	p, _ := d.CreateProject("ordered", "")
	d.CreateVersion(p.ID, "/v1")
	d.CreateVersion(p.ID, "/v2")
	d.CreateVersion(p.ID, "/v3")

	versions, err := d.ListVersions(p.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(versions) != 3 {
		t.Fatalf("expected 3 versions, got %d", len(versions))
	}
	if versions[0].VersionNum != 3 {
		t.Errorf("first should be v3, got v%d", versions[0].VersionNum)
	}
	if versions[2].VersionNum != 1 {
		t.Errorf("last should be v1, got v%d", versions[2].VersionNum)
	}
}

func TestListVersionsIsolatedByProject(t *testing.T) {
	d := newTestDB(t)
	p1, _ := d.CreateProject("proj1", "")
	p2, _ := d.CreateProject("proj2", "")
	d.CreateVersion(p1.ID, "/a")
	d.CreateVersion(p1.ID, "/b")
	d.CreateVersion(p2.ID, "/c")

	v1, _ := d.ListVersions(p1.ID)
	v2, _ := d.ListVersions(p2.ID)
	if len(v1) != 2 {
		t.Errorf("proj1: expected 2 versions, got %d", len(v1))
	}
	if len(v2) != 1 {
		t.Errorf("proj2: expected 1 version, got %d", len(v2))
	}
}

// --- Tokens ---

func TestCreateTokenAndGetUserByToken(t *testing.T) {
	d := newTestDB(t)
	err := d.CreateToken("tok123", "Alice", "alice@test.com")
	if err != nil {
		t.Fatal(err)
	}
	name, email, err := d.GetUserByToken("tok123")
	if err != nil {
		t.Fatal(err)
	}
	if name != "Alice" || email != "alice@test.com" {
		t.Errorf("got name=%q email=%q, want Alice alice@test.com", name, email)
	}
}

func TestGetUserByTokenNotFound(t *testing.T) {
	d := newTestDB(t)
	_, _, err := d.GetUserByToken("nonexistent")
	if err != sql.ErrNoRows {
		t.Errorf("expected sql.ErrNoRows, got %v", err)
	}
}

func TestCreateTokenDuplicate(t *testing.T) {
	d := newTestDB(t)
	d.CreateToken("dup", "A", "a@t.com")
	err := d.CreateToken("dup", "B", "b@t.com")
	if err == nil {
		t.Error("expected error for duplicate token")
	}
}

// --- Phase 17: Token Expiry ---

func TestExpiredTokenRejected(t *testing.T) {
	d := newTestDB(t)
	d.CreateToken("exp-tok", "Alice", "alice@test.com")
	d.Exec(`UPDATE tokens SET expires_at = datetime('now', '-1 second') WHERE token = ?`, "exp-tok")
	_, _, err := d.GetUserByToken("exp-tok")
	if err != sql.ErrNoRows {
		t.Errorf("expected ErrNoRows for expired token, got %v", err)
	}
}

func TestTokenHasExpiresAt(t *testing.T) {
	d := newTestDB(t)
	d.CreateToken("check-tok", "Bob", "bob@test.com")
	var expiresAt string
	err := d.QueryRow(`SELECT expires_at FROM tokens WHERE token = ?`, "check-tok").Scan(&expiresAt)
	if err != nil {
		t.Fatal(err)
	}
	if expiresAt == "" {
		t.Error("expires_at should be set")
	}
}

// --- Closed DB error tests ---

func closedDB(t *testing.T) *DB {
	t.Helper()
	d := newTestDB(t)
	d.Close()
	return d
}

func TestNewInvalidPath(t *testing.T) {
	_, err := New("/nonexistent/dir/test.db")
	if err == nil {
		t.Error("expected error for invalid path")
	}
}

func TestCreateProjectClosedDB(t *testing.T) {
	d := closedDB(t)
	_, err := d.CreateProject("x", "")
	if err == nil {
		t.Error("expected error")
	}
}

func TestGetProjectClosedDB(t *testing.T) {
	d := closedDB(t)
	_, err := d.GetProject("x")
	if err == nil {
		t.Error("expected error")
	}
}

func TestGetProjectByNameClosedDB(t *testing.T) {
	d := closedDB(t)
	_, err := d.GetProjectByName("x")
	if err == nil {
		t.Error("expected error")
	}
}

func TestListProjectsClosedDB(t *testing.T) {
	d := closedDB(t)
	_, err := d.ListProjects()
	if err == nil {
		t.Error("expected error")
	}
}

func TestListProjectsWithVersionCountClosedDB(t *testing.T) {
	d := closedDB(t)
	_, err := d.ListProjectsWithVersionCount()
	if err == nil {
		t.Error("expected error")
	}
}

func TestUpdateProjectStatusClosedDB(t *testing.T) {
	d := closedDB(t)
	err := d.UpdateProjectStatus("x", "draft")
	if err == nil {
		t.Error("expected error")
	}
}

func TestCreateVersionClosedDB(t *testing.T) {
	d := closedDB(t)
	_, err := d.CreateVersion("x", "/path")
	if err == nil {
		t.Error("expected error")
	}
}

func TestGetVersionClosedDB(t *testing.T) {
	d := closedDB(t)
	_, err := d.GetVersion("x")
	if err == nil {
		t.Error("expected error")
	}
}

func TestGetLatestVersionClosedDB(t *testing.T) {
	d := closedDB(t)
	_, err := d.GetLatestVersion("x")
	if err == nil {
		t.Error("expected error")
	}
}

func TestListVersionsClosedDB(t *testing.T) {
	d := closedDB(t)
	_, err := d.ListVersions("x")
	if err == nil {
		t.Error("expected error")
	}
}

func TestCreateCommentClosedDB(t *testing.T) {
	d := closedDB(t)
	_, err := d.CreateComment("v", "p", 0, 0, "n", "e", "b")
	if err == nil {
		t.Error("expected error")
	}
}

func TestGetCommentsForVersionClosedDB(t *testing.T) {
	d := closedDB(t)
	_, err := d.GetCommentsForVersion("x")
	if err == nil {
		t.Error("expected error")
	}
}

func TestGetUnresolvedCommentsUpToClosedDB(t *testing.T) {
	d := closedDB(t)
	_, err := d.GetUnresolvedCommentsUpTo("x")
	if err == nil {
		t.Error("expected error")
	}
}

func TestToggleResolveClosedDB(t *testing.T) {
	d := closedDB(t)
	_, err := d.ToggleResolve("x")
	if err == nil {
		t.Error("expected error")
	}
}

func TestCreateReplyClosedDB(t *testing.T) {
	d := closedDB(t)
	_, err := d.CreateReply("c", "n", "e", "b")
	if err == nil {
		t.Error("expected error")
	}
}

func TestGetRepliesClosedDB(t *testing.T) {
	d := closedDB(t)
	_, err := d.GetReplies("x")
	if err == nil {
		t.Error("expected error")
	}
}

func TestCreateTokenClosedDB(t *testing.T) {
	d := closedDB(t)
	err := d.CreateToken("t", "n", "e")
	if err == nil {
		t.Error("expected error")
	}
}

func TestGetUserByTokenClosedDB(t *testing.T) {
	d := closedDB(t)
	_, _, err := d.GetUserByToken("t")
	if err == nil {
		t.Error("expected error")
	}
}

func TestCreateProjectDuplicateName(t *testing.T) {
	d := newTestDB(t)
	d.CreateProject("dup", "")
	_, err := d.CreateProject("dup", "")
	if err == nil {
		t.Error("expected error for duplicate name")
	}
}

// --- Phase 12: Sharing ---

func TestCreateProjectWithOwner(t *testing.T) {
	d := newTestDB(t)
	p, err := d.CreateProject("owned", "alice@test.com")
	if err != nil {
		t.Fatal(err)
	}
	if p.OwnerEmail == nil || *p.OwnerEmail != "alice@test.com" {
		t.Errorf("owner = %v, want alice@test.com", p.OwnerEmail)
	}
}

func TestCreateProjectNullOwner(t *testing.T) {
	d := newTestDB(t)
	p, _ := d.CreateProject("seed", "")
	if p.OwnerEmail != nil {
		t.Errorf("expected nil owner, got %v", p.OwnerEmail)
	}
	got, _ := d.GetProject(p.ID)
	if got.OwnerEmail != nil {
		t.Errorf("GetProject: expected nil owner, got %v", got.OwnerEmail)
	}
}

func TestCanAccessProjectOwner(t *testing.T) {
	d := newTestDB(t)
	p, _ := d.CreateProject("p", "alice@test.com")
	ok, err := d.CanAccessProject(p.ID, "alice@test.com")
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Error("owner should have access")
	}
}

func TestCanAccessProjectNonMember(t *testing.T) {
	d := newTestDB(t)
	p, _ := d.CreateProject("p", "alice@test.com")
	ok, _ := d.CanAccessProject(p.ID, "bob@test.com")
	if ok {
		t.Error("non-member should not have access")
	}
}

func TestCanAccessProjectMember(t *testing.T) {
	d := newTestDB(t)
	p, _ := d.CreateProject("p", "alice@test.com")
	d.AddMember(p.ID, "bob@test.com")
	ok, _ := d.CanAccessProject(p.ID, "bob@test.com")
	if !ok {
		t.Error("member should have access")
	}
}

func TestCanAccessProjectNullOwner(t *testing.T) {
	d := newTestDB(t)
	p, _ := d.CreateProject("seed", "")
	ok, _ := d.CanAccessProject(p.ID, "anyone@test.com")
	if !ok {
		t.Error("NULL owner project should be accessible to all")
	}
}

func TestCanAccessProjectNotFound(t *testing.T) {
	d := newTestDB(t)
	ok, _ := d.CanAccessProject("nonexistent", "a@t.com")
	if ok {
		t.Error("nonexistent project should not be accessible")
	}
}

func TestListProjectsWithVersionCountForUser(t *testing.T) {
	d := newTestDB(t)
	d.CreateProject("seed", "")
	d.CreateProject("alice-proj", "alice@test.com")
	bob, _ := d.CreateProject("bob-proj", "bob@test.com")
	d.AddMember(bob.ID, "alice@test.com")

	// Alice sees: seed + her own + bob's (as member)
	projects, _ := d.ListProjectsWithVersionCountForUser("alice@test.com")
	if len(projects) != 3 {
		t.Errorf("alice should see 3 projects, got %d", len(projects))
	}

	// Charlie sees only seed
	projects, _ = d.ListProjectsWithVersionCountForUser("charlie@test.com")
	if len(projects) != 1 {
		t.Errorf("charlie should see 1 project, got %d", len(projects))
	}
}

func TestCreateInviteAndGetByToken(t *testing.T) {
	d := newTestDB(t)
	p, _ := d.CreateProject("p", "alice@test.com")
	inv, err := d.CreateInvite(p.ID, "alice@test.com")
	if err != nil {
		t.Fatal(err)
	}
	if len(inv.Token) != 64 {
		t.Errorf("token len = %d, want 64", len(inv.Token))
	}

	got, err := d.GetInviteByToken(inv.Token)
	if err != nil {
		t.Fatal(err)
	}
	if got.ProjectID != p.ID {
		t.Errorf("project mismatch")
	}
}

func TestGetInviteByTokenNotFound(t *testing.T) {
	d := newTestDB(t)
	_, err := d.GetInviteByToken("nonexistent")
	if err == nil {
		t.Error("expected error")
	}
}

func TestGetInviteByTokenExpired(t *testing.T) {
	d := newTestDB(t)
	p, _ := d.CreateProject("p", "a@t.com")
	inv, _ := d.CreateInvite(p.ID, "a@t.com")
	// Set expires_at to the past
	d.Exec(`UPDATE project_invites SET expires_at = datetime('now', '-1 hour') WHERE id = ?`, inv.ID)
	_, err := d.GetInviteByToken(inv.Token)
	if err == nil {
		t.Error("expired invite should not be returned")
	}
}

func TestDeleteInvite(t *testing.T) {
	d := newTestDB(t)
	p, _ := d.CreateProject("p", "a@t.com")
	inv, _ := d.CreateInvite(p.ID, "a@t.com")
	d.DeleteInvite(inv.ID)
	_, err := d.GetInviteByToken(inv.Token)
	if err == nil {
		t.Error("deleted invite should not be found")
	}
}

func TestAddMemberDuplicate(t *testing.T) {
	d := newTestDB(t)
	p, _ := d.CreateProject("p", "a@t.com")
	d.AddMember(p.ID, "b@t.com")
	err := d.AddMember(p.ID, "b@t.com")
	if err != nil {
		t.Errorf("duplicate AddMember should not error (INSERT OR IGNORE), got %v", err)
	}
	members, _ := d.ListMembers(p.ID)
	if len(members) != 1 {
		t.Errorf("expected 1 member, got %d", len(members))
	}
}

func TestListMembersEmpty(t *testing.T) {
	d := newTestDB(t)
	p, _ := d.CreateProject("p", "a@t.com")
	members, err := d.ListMembers(p.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(members) != 0 {
		t.Errorf("expected 0, got %d", len(members))
	}
}

func TestRemoveMember(t *testing.T) {
	d := newTestDB(t)
	p, _ := d.CreateProject("p", "a@t.com")
	d.AddMember(p.ID, "b@t.com")
	d.RemoveMember(p.ID, "b@t.com")
	members, _ := d.ListMembers(p.ID)
	if len(members) != 0 {
		t.Errorf("expected 0 after removal, got %d", len(members))
	}
}

func TestGetProjectOwner(t *testing.T) {
	d := newTestDB(t)
	p, _ := d.CreateProject("p", "alice@test.com")
	owner, err := d.GetProjectOwner(p.ID)
	if err != nil {
		t.Fatal(err)
	}
	if owner != "alice@test.com" {
		t.Errorf("owner = %q, want alice@test.com", owner)
	}
}

func TestGetProjectOwnerNull(t *testing.T) {
	d := newTestDB(t)
	p, _ := d.CreateProject("seed", "")
	owner, err := d.GetProjectOwner(p.ID)
	if err != nil {
		t.Fatal(err)
	}
	if owner != "" {
		t.Errorf("expected empty owner for NULL, got %q", owner)
	}
}

func TestGetProjectOwnerNotFound(t *testing.T) {
	d := newTestDB(t)
	_, err := d.GetProjectOwner("nonexistent")
	if err == nil {
		t.Error("expected error")
	}
}

func TestCanAccessProjectClosedDB(t *testing.T) {
	d := closedDB(t)
	_, err := d.CanAccessProject("x", "e")
	if err == nil {
		t.Error("expected error")
	}
}

func TestListProjectsWithVersionCountForUserClosedDB(t *testing.T) {
	d := closedDB(t)
	_, err := d.ListProjectsWithVersionCountForUser("e")
	if err == nil {
		t.Error("expected error")
	}
}

func TestCreateInviteClosedDB(t *testing.T) {
	d := closedDB(t)
	_, err := d.CreateInvite("x", "e")
	if err == nil {
		t.Error("expected error")
	}
}

func TestAddMemberClosedDB(t *testing.T) {
	d := closedDB(t)
	err := d.AddMember("x", "e")
	if err == nil {
		t.Error("expected error")
	}
}

func TestListMembersClosedDB(t *testing.T) {
	d := closedDB(t)
	_, err := d.ListMembers("x")
	if err == nil {
		t.Error("expected error")
	}
}

// --- Phase 20: MoveComment ---

func TestMoveComment(t *testing.T) {
	d := newTestDB(t)
	p, _ := d.CreateProject("mv", "")
	v, _ := d.CreateVersion(p.ID, "/tmp/v")
	c, _ := d.CreateComment(v.ID, "index.html", 10, 20, "A", "a@t.com", "hi")

	if err := d.MoveComment(c.ID, 55.5, 77.3); err != nil {
		t.Fatal(err)
	}

	comments, _ := d.GetCommentsForVersion(v.ID)
	if len(comments) != 1 {
		t.Fatalf("expected 1 comment, got %d", len(comments))
	}
	if comments[0].XPercent != 55.5 || comments[0].YPercent != 77.3 {
		t.Errorf("coords = (%v, %v), want (55.5, 77.3)", comments[0].XPercent, comments[0].YPercent)
	}
}

func TestMoveCommentNonexistent(t *testing.T) {
	d := newTestDB(t)
	// Should not error — UPDATE affects 0 rows
	if err := d.MoveComment("nonexistent", 10, 20); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestMoveCommentClosedDB(t *testing.T) {
	d := closedDB(t)
	if err := d.MoveComment("x", 10, 20); err == nil {
		t.Error("expected error")
	}
}

// --- Phase 21: GetComment ---

func TestGetComment(t *testing.T) {
	d := newTestDB(t)
	p, _ := d.CreateProject("gc", "")
	v, _ := d.CreateVersion(p.ID, "/tmp/v")
	c, _ := d.CreateComment(v.ID, "index.html", 10.5, 20.3, "Alice", "a@t.com", "hello")

	got, err := d.GetComment(c.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.VersionID != v.ID {
		t.Errorf("VersionID = %q, want %q", got.VersionID, v.ID)
	}
	if got.Body != "hello" || got.Page != "index.html" {
		t.Errorf("unexpected comment: %+v", got)
	}
}

func TestGetCommentNotFound(t *testing.T) {
	d := newTestDB(t)
	_, err := d.GetComment("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent comment")
	}
}

func TestGetCommentClosedDB(t *testing.T) {
	d := closedDB(t)
	_, err := d.GetComment("x")
	if err == nil {
		t.Error("expected error")
	}
}
