package db

import (
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
	p, err := d.CreateProject("proj-a")
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
	if _, err := d.CreateProject("empty-proj"); err != nil {
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
	pa, _ := d.CreateProject("proj-a")
	pb, _ := d.CreateProject("proj-b")
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
	p1, _ := d.CreateProject("older")
	d.CreateProject("newer")
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
	p, _ := d.CreateProject("proj")
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
	p, _ := d.CreateProject("proj")
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
	p, _ := d.CreateProject("proj")
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
	p, _ := d.CreateProject("proj")
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
	p, _ := d.CreateProject("proj")
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
	p, _ := d.CreateProject("proj")
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
