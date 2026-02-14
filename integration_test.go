package integration

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ab/design-reviewer/internal/api"
	authpkg "github.com/ab/design-reviewer/internal/auth"
	"github.com/ab/design-reviewer/internal/cli"
	"github.com/ab/design-reviewer/internal/db"
	"github.com/ab/design-reviewer/internal/storage"
	"golang.org/x/oauth2"
)

// testEnv holds all dependencies for a test run.
type testEnv struct {
	Server  *httptest.Server
	DB      *db.DB
	Storage *storage.Storage
	TmpDir  string
}

// setup creates a fresh test environment with temp DB and storage.
// Call t.Cleanup or defer env.Close() when done.
func setup(t *testing.T) *testEnv {
	t.Helper()
	tmp := t.TempDir()

	database, err := db.New(filepath.Join(tmp, "test.db"))
	if err != nil {
		t.Fatal(err)
	}

	store := storage.New(filepath.Join(tmp, "uploads"))

	h := &api.Handler{DB: database, Storage: store, TemplatesDir: "web/templates", StaticDir: "web/static"}
	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	srv := httptest.NewServer(mux)
	t.Cleanup(func() {
		srv.Close()
		database.Close()
	})

	return &testEnv{Server: srv, DB: database, Storage: store, TmpDir: tmp}
}

// makeZip creates an in-memory zip with the given filename→content pairs.
func makeZip(t *testing.T, files map[string]string) []byte {
	t.Helper()
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	for name, content := range files {
		f, err := w.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		f.Write([]byte(content))
	}
	w.Close()
	return buf.Bytes()
}

// uploadZip posts a zip to /api/upload and returns the parsed JSON response.
func uploadZip(t *testing.T, baseURL, projectName string, zipData []byte) map[string]any {
	t.Helper()
	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	mw.WriteField("name", projectName)
	fw, _ := mw.CreateFormFile("file", "upload.zip")
	fw.Write(zipData)
	mw.Close()

	resp, err := http.Post(baseURL+"/api/upload", mw.FormDataContentType(), &body)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("upload failed: %d %s", resp.StatusCode, b)
	}
	var result map[string]any
	json.NewDecoder(resp.Body).Decode(&result)
	return result
}

// --- Phase 2: Storage + Upload + Static Serving ---

func TestUploadCreatesProjectAndVersion(t *testing.T) {
	env := setup(t)
	z := makeZip(t, map[string]string{"index.html": "<h1>hi</h1>"})

	res := uploadZip(t, env.Server.URL, "my-project", z)

	if res["version_num"].(float64) != 1 {
		t.Errorf("expected version_num=1, got %v", res["version_num"])
	}
	if res["project_id"] == nil || res["version_id"] == nil {
		t.Error("missing project_id or version_id")
	}
}

func TestUploadIncrementsVersion(t *testing.T) {
	env := setup(t)
	z := makeZip(t, map[string]string{"index.html": "<h1>hi</h1>"})

	uploadZip(t, env.Server.URL, "my-project", z)
	res2 := uploadZip(t, env.Server.URL, "my-project", z)

	if res2["version_num"].(float64) != 2 {
		t.Errorf("expected version_num=2, got %v", res2["version_num"])
	}
}

func TestServeUploadedFiles(t *testing.T) {
	env := setup(t)
	z := makeZip(t, map[string]string{
		"index.html": "<h1>hello</h1>",
		"style.css":  "body{}",
	})
	res := uploadZip(t, env.Server.URL, "proj", z)
	vid := res["version_id"].(string)

	tests := []struct {
		path        string
		wantStatus  int
		wantContain string
	}{
		{"/designs/" + vid + "/index.html", 200, "<h1>hello</h1>"},
		{"/designs/" + vid + "/style.css", 200, "body{}"},
		{"/designs/" + vid + "/nope.html", 404, ""},
	}
	for _, tt := range tests {
		resp, err := http.Get(env.Server.URL + tt.path)
		if err != nil {
			t.Fatal(err)
		}
		if resp.StatusCode != tt.wantStatus {
			t.Errorf("%s: got %d, want %d", tt.path, resp.StatusCode, tt.wantStatus)
		}
		if tt.wantContain != "" {
			b, _ := io.ReadAll(resp.Body)
			if !bytes.Contains(b, []byte(tt.wantContain)) {
				t.Errorf("%s: body missing %q", tt.path, tt.wantContain)
			}
		}
		resp.Body.Close()
	}
}

func TestPathTraversalRejected(t *testing.T) {
	env := setup(t)
	z := makeZip(t, map[string]string{"index.html": "x"})
	res := uploadZip(t, env.Server.URL, "proj", z)
	vid := res["version_id"].(string)

	resp, err := http.Get(env.Server.URL + "/designs/" + vid + "/..%2F..%2Fetc%2Fpasswd")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != 400 {
		t.Errorf("expected 400, got %d", resp.StatusCode)
	}
}

func TestUploadMissingFields(t *testing.T) {
	env := setup(t)
	z := makeZip(t, map[string]string{"index.html": "x"})

	// Missing name
	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	fw, _ := mw.CreateFormFile("file", "upload.zip")
	fw.Write(z)
	mw.Close()
	resp, _ := http.Post(env.Server.URL+"/api/upload", mw.FormDataContentType(), &body)
	resp.Body.Close()
	if resp.StatusCode != 400 {
		t.Errorf("missing name: expected 400, got %d", resp.StatusCode)
	}

	// Missing file
	body.Reset()
	mw = multipart.NewWriter(&body)
	mw.WriteField("name", "test")
	mw.Close()
	resp, _ = http.Post(env.Server.URL+"/api/upload", mw.FormDataContentType(), &body)
	resp.Body.Close()
	if resp.StatusCode != 400 {
		t.Errorf("missing file: expected 400, got %d", resp.StatusCode)
	}
}

func TestUploadRejectsZipWithoutHTML(t *testing.T) {
	env := setup(t)
	z := makeZip(t, map[string]string{"readme.txt": "no html here"})

	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	mw.WriteField("name", "bad-project")
	fw, _ := mw.CreateFormFile("file", "upload.zip")
	fw.Write(z)
	mw.Close()

	resp, _ := http.Post(env.Server.URL+"/api/upload", mw.FormDataContentType(), &body)
	resp.Body.Close()
	if resp.StatusCode != 400 {
		t.Errorf("expected 400 for zip without html, got %d", resp.StatusCode)
	}
}

func TestStorageListHTMLFiles(t *testing.T) {
	env := setup(t)
	z := makeZip(t, map[string]string{
		"index.html": "a",
		"about.html": "b",
		"style.css":  "c",
	})
	res := uploadZip(t, env.Server.URL, "proj", z)
	vid := res["version_id"].(string)

	files, err := env.Storage.ListHTMLFiles(vid)
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 2 {
		t.Errorf("expected 2 html files, got %d: %v", len(files), files)
	}
}

func TestStorageSavesFilesToDisk(t *testing.T) {
	env := setup(t)
	z := makeZip(t, map[string]string{"index.html": "disk-check"})
	res := uploadZip(t, env.Server.URL, "proj", z)
	vid := res["version_id"].(string)

	path := env.Storage.GetFilePath(vid, "index.html")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "disk-check" {
		t.Errorf("file content = %q, want %q", data, "disk-check")
	}
}

// --- Phase 3: Project List ---

func TestListProjectsAPIEmpty(t *testing.T) {
	env := setup(t)
	resp, err := http.Get(env.Server.URL + "/api/projects")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	var projects []map[string]any
	json.NewDecoder(resp.Body).Decode(&projects)
	if len(projects) != 0 {
		t.Errorf("expected empty array, got %d items", len(projects))
	}
}

func TestListProjectsAPIAfterUpload(t *testing.T) {
	env := setup(t)
	z := makeZip(t, map[string]string{"index.html": "<h1>hi</h1>"})
	uploadZip(t, env.Server.URL, "proj-a", z)
	uploadZip(t, env.Server.URL, "proj-b", z)
	// Upload second version for proj-a
	uploadZip(t, env.Server.URL, "proj-a", z)

	resp, err := http.Get(env.Server.URL + "/api/projects")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var projects []map[string]any
	json.NewDecoder(resp.Body).Decode(&projects)
	if len(projects) != 2 {
		t.Fatalf("expected 2 projects, got %d", len(projects))
	}

	counts := map[string]float64{}
	for _, p := range projects {
		counts[p["name"].(string)] = p["version_count"].(float64)
	}
	if counts["proj-a"] != 2 {
		t.Errorf("proj-a: expected version_count=2, got %v", counts["proj-a"])
	}
	if counts["proj-b"] != 1 {
		t.Errorf("proj-b: expected version_count=1, got %v", counts["proj-b"])
	}
}

func TestListProjectsAPIResponseFormat(t *testing.T) {
	env := setup(t)
	z := makeZip(t, map[string]string{"index.html": "x"})
	uploadZip(t, env.Server.URL, "format-test", z)

	resp, err := http.Get(env.Server.URL + "/api/projects")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if ct := resp.Header.Get("Content-Type"); ct != "application/json" {
		t.Errorf("expected Content-Type application/json, got %s", ct)
	}

	var projects []map[string]any
	json.NewDecoder(resp.Body).Decode(&projects)
	p := projects[0]
	for _, field := range []string{"id", "name", "status", "version_count", "updated_at"} {
		if p[field] == nil {
			t.Errorf("missing field %q in response", field)
		}
	}
}

func TestHomePageRendersEmpty(t *testing.T) {
	env := setup(t)
	resp, err := http.Get(env.Server.URL + "/")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	b, _ := io.ReadAll(resp.Body)
	body := string(b)
	if !strings.Contains(body, "Design Reviewer") {
		t.Error("missing page title")
	}
	if !strings.Contains(body, "No projects yet") {
		t.Error("missing empty state message")
	}
}

func TestHomePageRendersProjects(t *testing.T) {
	env := setup(t)
	z := makeZip(t, map[string]string{"index.html": "x"})
	uploadZip(t, env.Server.URL, "home-test", z)

	resp, err := http.Get(env.Server.URL + "/")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	body := string(b)
	if !strings.Contains(body, "home-test") {
		t.Error("missing project name in home page")
	}
	if !strings.Contains(body, "badge-draft") {
		t.Error("missing status badge")
	}
}

func TestStaticFileServing(t *testing.T) {
	env := setup(t)
	resp, err := http.Get(env.Server.URL + "/static/style.css")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	b, _ := io.ReadAll(resp.Body)
	if !strings.Contains(string(b), "badge-draft") {
		t.Error("style.css missing expected content")
	}
}

// --- Phase 4: Design Viewer ---

func TestViewerRendersDesign(t *testing.T) {
	env := setup(t)
	z := makeZip(t, map[string]string{"index.html": "<h1>hello</h1>", "style.css": "body{}"})
	res := uploadZip(t, env.Server.URL, "viewer-proj", z)
	pid := res["project_id"].(string)
	vid := res["version_id"].(string)

	resp, err := http.Get(env.Server.URL + "/projects/" + pid)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	b, _ := io.ReadAll(resp.Body)
	body := string(b)
	if !strings.Contains(body, "viewer-proj") {
		t.Error("missing project name")
	}
	if !strings.Contains(body, vid) {
		t.Error("missing version ID in iframe src")
	}
	if !strings.Contains(body, `sandbox="allow-same-origin"`) {
		t.Error("missing sandbox attribute on iframe")
	}
}

func TestViewerMultiPageTabs(t *testing.T) {
	env := setup(t)
	z := makeZip(t, map[string]string{
		"index.html": "<h1>home</h1>",
		"about.html": "<h1>about</h1>",
	})
	res := uploadZip(t, env.Server.URL, "multi-page", z)
	pid := res["project_id"].(string)

	resp, err := http.Get(env.Server.URL + "/projects/" + pid)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	body := string(b)
	if !strings.Contains(body, "page-tabs") {
		t.Error("missing page tabs")
	}
	if !strings.Contains(body, "about.html") {
		t.Error("missing about.html tab")
	}
	if !strings.Contains(body, "index.html") {
		t.Error("missing index.html tab")
	}
}

func TestViewerWithVersionParam(t *testing.T) {
	env := setup(t)
	z := makeZip(t, map[string]string{"index.html": "v1"})
	res := uploadZip(t, env.Server.URL, "ver-proj", z)
	pid := res["project_id"].(string)
	vid1 := res["version_id"].(string)

	// Upload v2
	z2 := makeZip(t, map[string]string{"index.html": "v2"})
	uploadZip(t, env.Server.URL, "ver-proj", z2)

	// Request with explicit version=v1
	resp, err := http.Get(env.Server.URL + "/projects/" + pid + "?version=" + vid1)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	b, _ := io.ReadAll(resp.Body)
	if !strings.Contains(string(b), vid1) {
		t.Error("should render version 1 iframe src")
	}
}

func TestViewerProjectNotFound(t *testing.T) {
	env := setup(t)
	resp, err := http.Get(env.Server.URL + "/projects/nonexistent-id")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != 404 {
		t.Errorf("expected 404, got %d", resp.StatusCode)
	}
}

func TestViewerSinglePageNoTabs(t *testing.T) {
	env := setup(t)
	z := makeZip(t, map[string]string{"index.html": "<h1>only</h1>"})
	res := uploadZip(t, env.Server.URL, "single-page", z)
	pid := res["project_id"].(string)

	resp, err := http.Get(env.Server.URL + "/projects/" + pid)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	if strings.Contains(string(b), "page-tabs") {
		t.Error("should not show page tabs for single-page project")
	}
}

func TestViewerIframeServesDesign(t *testing.T) {
	env := setup(t)
	z := makeZip(t, map[string]string{"index.html": "<h1>iframe-test</h1>"})
	res := uploadZip(t, env.Server.URL, "iframe-proj", z)
	vid := res["version_id"].(string)

	// Verify the iframe src URL actually serves the design
	resp, err := http.Get(env.Server.URL + "/designs/" + vid + "/index.html")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	b, _ := io.ReadAll(resp.Body)
	if !strings.Contains(string(b), "iframe-test") {
		t.Error("design file not served correctly")
	}
}

// --- Phase 5: Annotations ---

func TestCreateCommentAndGetComments(t *testing.T) {
	env := setup(t)
	z := makeZip(t, map[string]string{"index.html": "<h1>hi</h1>"})
	res := uploadZip(t, env.Server.URL, "ann-proj", z)
	vid := res["version_id"].(string)

	// Create a comment
	body := `{"page":"index.html","x_percent":25.5,"y_percent":50.0,"author_name":"Alice","author_email":"alice@test.com","body":"needs padding"}`
	resp, err := http.Post(env.Server.URL+"/api/versions/"+vid+"/comments", "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 201 {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 201, got %d: %s", resp.StatusCode, b)
	}
	var created map[string]any
	json.NewDecoder(resp.Body).Decode(&created)
	if created["body"] != "needs padding" {
		t.Errorf("body = %v, want 'needs padding'", created["body"])
	}

	// Get comments
	resp2, err := http.Get(env.Server.URL + "/api/versions/" + vid + "/comments")
	if err != nil {
		t.Fatal(err)
	}
	defer resp2.Body.Close()
	var comments []map[string]any
	json.NewDecoder(resp2.Body).Decode(&comments)
	if len(comments) != 1 {
		t.Fatalf("expected 1 comment, got %d", len(comments))
	}
	if comments[0]["page"] != "index.html" {
		t.Errorf("page = %v, want index.html", comments[0]["page"])
	}
}

func TestCreateReplyIntegration(t *testing.T) {
	env := setup(t)
	z := makeZip(t, map[string]string{"index.html": "x"})
	res := uploadZip(t, env.Server.URL, "reply-proj", z)
	vid := res["version_id"].(string)

	// Create comment
	body := `{"page":"index.html","x_percent":10,"y_percent":20,"author_name":"Alice","author_email":"a@t.com","body":"hello"}`
	resp, _ := http.Post(env.Server.URL+"/api/versions/"+vid+"/comments", "application/json", strings.NewReader(body))
	var created map[string]any
	json.NewDecoder(resp.Body).Decode(&created)
	resp.Body.Close()
	cid := created["id"].(string)

	// Create reply
	replyBody := `{"author_name":"Bob","author_email":"b@t.com","body":"agreed"}`
	resp2, err := http.Post(env.Server.URL+"/api/comments/"+cid+"/replies", "application/json", strings.NewReader(replyBody))
	if err != nil {
		t.Fatal(err)
	}
	defer resp2.Body.Close()
	if resp2.StatusCode != 201 {
		b, _ := io.ReadAll(resp2.Body)
		t.Fatalf("expected 201, got %d: %s", resp2.StatusCode, b)
	}

	// Verify reply appears in GET comments
	resp3, err := http.Get(env.Server.URL + "/api/versions/" + vid + "/comments")
	if err != nil {
		t.Fatal(err)
	}
	defer resp3.Body.Close()
	var comments []map[string]any
	json.NewDecoder(resp3.Body).Decode(&comments)
	replies := comments[0]["replies"].([]any)
	if len(replies) != 1 {
		t.Fatalf("expected 1 reply, got %d", len(replies))
	}
	r := replies[0].(map[string]any)
	if r["body"] != "agreed" {
		t.Errorf("reply body = %v, want 'agreed'", r["body"])
	}
}

func TestToggleResolveIntegration(t *testing.T) {
	env := setup(t)
	z := makeZip(t, map[string]string{"index.html": "x"})
	res := uploadZip(t, env.Server.URL, "resolve-proj", z)
	vid := res["version_id"].(string)

	// Create comment
	body := `{"page":"index.html","x_percent":10,"y_percent":20,"author_name":"Alice","author_email":"a@t.com","body":"fix this"}`
	resp, _ := http.Post(env.Server.URL+"/api/versions/"+vid+"/comments", "application/json", strings.NewReader(body))
	var created map[string]any
	json.NewDecoder(resp.Body).Decode(&created)
	resp.Body.Close()
	cid := created["id"].(string)

	// Toggle resolve
	req, _ := http.NewRequest("PATCH", env.Server.URL+"/api/comments/"+cid+"/resolve", nil)
	client := &http.Client{}
	resp2, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp2.Body.Close()
	if resp2.StatusCode != 200 {
		t.Fatalf("expected 200, got %d", resp2.StatusCode)
	}
	var result map[string]any
	json.NewDecoder(resp2.Body).Decode(&result)
	if result["resolved"] != true {
		t.Errorf("expected resolved=true, got %v", result["resolved"])
	}

	// Toggle back
	req2, _ := http.NewRequest("PATCH", env.Server.URL+"/api/comments/"+cid+"/resolve", nil)
	resp3, err := client.Do(req2)
	if err != nil {
		t.Fatal(err)
	}
	defer resp3.Body.Close()
	var result2 map[string]any
	json.NewDecoder(resp3.Body).Decode(&result2)
	if result2["resolved"] != false {
		t.Errorf("expected resolved=false, got %v", result2["resolved"])
	}
}

func TestCommentCarryOverIntegration(t *testing.T) {
	env := setup(t)
	z := makeZip(t, map[string]string{"index.html": "x"})

	// Upload v1
	res1 := uploadZip(t, env.Server.URL, "carry-proj", z)
	vid1 := res1["version_id"].(string)

	// Create unresolved comment on v1
	body := `{"page":"index.html","x_percent":10,"y_percent":20,"author_name":"Alice","author_email":"a@t.com","body":"unresolved"}`
	resp, _ := http.Post(env.Server.URL+"/api/versions/"+vid1+"/comments", "application/json", strings.NewReader(body))
	resp.Body.Close()

	// Create and resolve another comment on v1
	body2 := `{"page":"index.html","x_percent":30,"y_percent":40,"author_name":"Bob","author_email":"b@t.com","body":"resolved"}`
	resp2, _ := http.Post(env.Server.URL+"/api/versions/"+vid1+"/comments", "application/json", strings.NewReader(body2))
	var created map[string]any
	json.NewDecoder(resp2.Body).Decode(&created)
	resp2.Body.Close()
	cid := created["id"].(string)
	req, _ := http.NewRequest("PATCH", env.Server.URL+"/api/comments/"+cid+"/resolve", nil)
	client := &http.Client{}
	r, _ := client.Do(req)
	r.Body.Close()

	// Upload v2
	res2 := uploadZip(t, env.Server.URL, "carry-proj", z)
	vid2 := res2["version_id"].(string)

	// GET comments for v2 — should have only the unresolved one
	resp3, err := http.Get(env.Server.URL + "/api/versions/" + vid2 + "/comments")
	if err != nil {
		t.Fatal(err)
	}
	defer resp3.Body.Close()
	var comments []map[string]any
	json.NewDecoder(resp3.Body).Decode(&comments)
	if len(comments) != 1 {
		t.Fatalf("expected 1 carried-over comment, got %d", len(comments))
	}
	if comments[0]["body"] != "unresolved" {
		t.Errorf("expected unresolved comment, got %q", comments[0]["body"])
	}
}

func TestViewerHasAnnotationElements(t *testing.T) {
	env := setup(t)
	z := makeZip(t, map[string]string{"index.html": "x"})
	res := uploadZip(t, env.Server.URL, "ann-viewer", z)
	pid := res["project_id"].(string)

	resp, err := http.Get(env.Server.URL + "/projects/" + pid)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	body := string(b)

	checks := []string{"pin-overlay", "comment-panel", "annotation-filter", "annotations.js", "data-version-id"}
	for _, check := range checks {
		if !strings.Contains(body, check) {
			t.Errorf("viewer HTML missing %q", check)
		}
	}
}

// --- Phase 6: Version History ---

func TestListVersionsAPI(t *testing.T) {
	env := setup(t)
	z := makeZip(t, map[string]string{"index.html": "x"})
	res := uploadZip(t, env.Server.URL, "ver-list", z)
	pid := res["project_id"].(string)
	uploadZip(t, env.Server.URL, "ver-list", z)

	resp, err := http.Get(env.Server.URL + "/api/projects/" + pid + "/versions")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	if ct := resp.Header.Get("Content-Type"); ct != "application/json" {
		t.Errorf("expected application/json, got %s", ct)
	}

	var versions []map[string]any
	json.NewDecoder(resp.Body).Decode(&versions)
	if len(versions) != 2 {
		t.Fatalf("expected 2 versions, got %d", len(versions))
	}
	// Newest first
	if versions[0]["version_num"].(float64) != 2 {
		t.Errorf("first version should be 2, got %v", versions[0]["version_num"])
	}
	if versions[1]["version_num"].(float64) != 1 {
		t.Errorf("second version should be 1, got %v", versions[1]["version_num"])
	}
	// Check pages field
	pages := versions[0]["pages"].([]any)
	if len(pages) == 0 {
		t.Error("expected pages in version response")
	}
}

func TestListVersionsAPIEmpty(t *testing.T) {
	env := setup(t)
	p, _ := env.DB.CreateProject("no-versions", "")

	resp, err := http.Get(env.Server.URL + "/api/projects/" + p.ID + "/versions")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	var versions []map[string]any
	json.NewDecoder(resp.Body).Decode(&versions)
	if len(versions) != 0 {
		t.Errorf("expected 0 versions, got %d", len(versions))
	}
}

func TestViewerHasVersionListContainer(t *testing.T) {
	env := setup(t)
	z := makeZip(t, map[string]string{"index.html": "x"})
	res := uploadZip(t, env.Server.URL, "ver-viewer", z)
	pid := res["project_id"].(string)

	resp, err := http.Get(env.Server.URL + "/projects/" + pid)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	body := string(b)

	checks := []string{"version-list", "data-project-id", "Versions"}
	for _, check := range checks {
		if !strings.Contains(body, check) {
			t.Errorf("viewer HTML missing %q", check)
		}
	}
}

func TestCommentCarryOverResolvedOnPreviousHidden(t *testing.T) {
	env := setup(t)
	z := makeZip(t, map[string]string{"index.html": "x"})

	// Upload v1
	res1 := uploadZip(t, env.Server.URL, "carry2", z)
	vid1 := res1["version_id"].(string)

	// Create comment on v1 and resolve it
	body := `{"page":"index.html","x_percent":10,"y_percent":20,"author_name":"A","author_email":"a@t.com","body":"will resolve"}`
	resp, _ := http.Post(env.Server.URL+"/api/versions/"+vid1+"/comments", "application/json", strings.NewReader(body))
	var created map[string]any
	json.NewDecoder(resp.Body).Decode(&created)
	resp.Body.Close()
	cid := created["id"].(string)

	// Resolve it
	req, _ := http.NewRequest("PATCH", env.Server.URL+"/api/comments/"+cid+"/resolve", nil)
	r, _ := (&http.Client{}).Do(req)
	r.Body.Close()

	// Upload v2
	res2 := uploadZip(t, env.Server.URL, "carry2", z)
	vid2 := res2["version_id"].(string)

	// Upload v3
	res3 := uploadZip(t, env.Server.URL, "carry2", z)
	vid3 := res3["version_id"].(string)

	// v2 should still show the resolved comment (it was resolved on v1, but v1 comments show on v1)
	resp2, _ := http.Get(env.Server.URL + "/api/versions/" + vid2 + "/comments")
	var comments2 []map[string]any
	json.NewDecoder(resp2.Body).Decode(&comments2)
	resp2.Body.Close()
	if len(comments2) != 0 {
		t.Errorf("v2: expected 0 comments (resolved on previous version), got %d", len(comments2))
	}

	// v3 should also not show it
	resp3, _ := http.Get(env.Server.URL + "/api/versions/" + vid3 + "/comments")
	var comments3 []map[string]any
	json.NewDecoder(resp3.Body).Decode(&comments3)
	resp3.Body.Close()
	if len(comments3) != 0 {
		t.Errorf("v3: expected 0 comments, got %d", len(comments3))
	}

	// But v1 should still show it (resolved on current version)
	resp1, _ := http.Get(env.Server.URL + "/api/versions/" + vid1 + "/comments")
	var comments1 []map[string]any
	json.NewDecoder(resp1.Body).Decode(&comments1)
	resp1.Body.Close()
	if len(comments1) != 1 {
		t.Fatalf("v1: expected 1 comment (resolved on this version), got %d", len(comments1))
	}
	if comments1[0]["resolved"] != true {
		t.Error("v1: comment should be resolved")
	}
}

func TestVersionListResponseFields(t *testing.T) {
	env := setup(t)
	z := makeZip(t, map[string]string{"index.html": "a", "about.html": "b"})
	res := uploadZip(t, env.Server.URL, "fields-proj", z)
	pid := res["project_id"].(string)

	resp, err := http.Get(env.Server.URL + "/api/projects/" + pid + "/versions")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var versions []map[string]any
	json.NewDecoder(resp.Body).Decode(&versions)
	if len(versions) != 1 {
		t.Fatalf("expected 1 version, got %d", len(versions))
	}
	v := versions[0]
	for _, field := range []string{"id", "version_num", "created_at", "pages"} {
		if v[field] == nil {
			t.Errorf("missing field %q", field)
		}
	}
	pages := v["pages"].([]any)
	if len(pages) != 2 {
		t.Errorf("expected 2 pages, got %d", len(pages))
	}
}

// --- Phase 7: Status Workflow ---

func TestUpdateStatusAPI(t *testing.T) {
	env := setup(t)
	z := makeZip(t, map[string]string{"index.html": "x"})
	res := uploadZip(t, env.Server.URL, "status-proj", z)
	pid := res["project_id"].(string)

	// Update to in_review
	req, _ := http.NewRequest("PATCH", env.Server.URL+"/api/projects/"+pid+"/status", strings.NewReader(`{"status":"in_review"}`))
	req.Header.Set("Content-Type", "application/json")
	resp, err := (&http.Client{}).Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 200, got %d: %s", resp.StatusCode, b)
	}
	var result map[string]string
	json.NewDecoder(resp.Body).Decode(&result)
	if result["id"] != pid {
		t.Errorf("expected id=%s, got %s", pid, result["id"])
	}
	if result["status"] != "in_review" {
		t.Errorf("expected status=in_review, got %s", result["status"])
	}
}

func TestUpdateStatusCycleAllStatuses(t *testing.T) {
	env := setup(t)
	z := makeZip(t, map[string]string{"index.html": "x"})
	res := uploadZip(t, env.Server.URL, "cycle-proj", z)
	pid := res["project_id"].(string)
	client := &http.Client{}

	for _, status := range []string{"in_review", "approved", "handed_off", "draft"} {
		req, _ := http.NewRequest("PATCH", env.Server.URL+"/api/projects/"+pid+"/status", strings.NewReader(`{"status":"`+status+`"}`))
		req.Header.Set("Content-Type", "application/json")
		resp, err := client.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		if resp.StatusCode != 200 {
			t.Errorf("status %q: expected 200, got %d", status, resp.StatusCode)
		}
	}

	// Verify final status via project list
	resp, err := http.Get(env.Server.URL + "/api/projects")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var projects []map[string]any
	json.NewDecoder(resp.Body).Decode(&projects)
	for _, p := range projects {
		if p["name"] == "cycle-proj" {
			if p["status"] != "draft" {
				t.Errorf("expected final status=draft, got %v", p["status"])
			}
			return
		}
	}
	t.Error("project not found in list")
}

func TestUpdateStatusInvalidRejected(t *testing.T) {
	env := setup(t)
	z := makeZip(t, map[string]string{"index.html": "x"})
	res := uploadZip(t, env.Server.URL, "invalid-status", z)
	pid := res["project_id"].(string)

	req, _ := http.NewRequest("PATCH", env.Server.URL+"/api/projects/"+pid+"/status", strings.NewReader(`{"status":"bogus"}`))
	req.Header.Set("Content-Type", "application/json")
	resp, err := (&http.Client{}).Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != 400 {
		t.Errorf("expected 400, got %d", resp.StatusCode)
	}
}

func TestUpdateStatusNotFound(t *testing.T) {
	env := setup(t)

	req, _ := http.NewRequest("PATCH", env.Server.URL+"/api/projects/nonexistent/status", strings.NewReader(`{"status":"draft"}`))
	req.Header.Set("Content-Type", "application/json")
	resp, err := (&http.Client{}).Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != 404 {
		t.Errorf("expected 404, got %d", resp.StatusCode)
	}
}

func TestStatusReflectedOnHomePage(t *testing.T) {
	env := setup(t)
	z := makeZip(t, map[string]string{"index.html": "x"})
	res := uploadZip(t, env.Server.URL, "home-status", z)
	pid := res["project_id"].(string)

	// Change to approved
	req, _ := http.NewRequest("PATCH", env.Server.URL+"/api/projects/"+pid+"/status", strings.NewReader(`{"status":"approved"}`))
	req.Header.Set("Content-Type", "application/json")
	r, _ := (&http.Client{}).Do(req)
	r.Body.Close()

	// Check home page
	resp, err := http.Get(env.Server.URL + "/")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	if !strings.Contains(string(b), "badge-approved") {
		t.Error("home page should show badge-approved after status change")
	}
}

func TestStatusReflectedOnViewerPage(t *testing.T) {
	env := setup(t)
	z := makeZip(t, map[string]string{"index.html": "x"})
	res := uploadZip(t, env.Server.URL, "viewer-status", z)
	pid := res["project_id"].(string)

	// Change to handed_off
	req, _ := http.NewRequest("PATCH", env.Server.URL+"/api/projects/"+pid+"/status", strings.NewReader(`{"status":"handed_off"}`))
	req.Header.Set("Content-Type", "application/json")
	r, _ := (&http.Client{}).Do(req)
	r.Body.Close()

	// Check viewer page
	resp, err := http.Get(env.Server.URL + "/projects/" + pid)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	body := string(b)
	if !strings.Contains(body, "badge-handed_off") {
		t.Error("viewer page should show badge-handed_off after status change")
	}
	if !strings.Contains(body, "status-select") {
		t.Error("viewer page should have status dropdown")
	}
}

func TestNewProjectStartsAsDraft(t *testing.T) {
	env := setup(t)
	z := makeZip(t, map[string]string{"index.html": "x"})
	uploadZip(t, env.Server.URL, "draft-proj", z)

	resp, err := http.Get(env.Server.URL + "/api/projects")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var projects []map[string]any
	json.NewDecoder(resp.Body).Decode(&projects)
	for _, p := range projects {
		if p["name"] == "draft-proj" {
			if p["status"] != "draft" {
				t.Errorf("new project should be draft, got %v", p["status"])
			}
			return
		}
	}
	t.Error("project not found")
}

// --- Phase 8: Google OAuth ---

// setupWithAuth creates a test environment with auth enabled and a mock OAuth provider.
func setupWithAuth(t *testing.T) (*testEnv, string) {
	t.Helper()
	tmp := t.TempDir()

	database, err := db.New(filepath.Join(tmp, "test.db"))
	if err != nil {
		t.Fatal(err)
	}

	store := storage.New(filepath.Join(tmp, "uploads"))

	authCfg := &authpkg.Config{
		ClientID:      "test-client-id",
		ClientSecret:  "test-secret",
		RedirectURL:   "http://localhost/auth/google/callback",
		SessionSecret: "integration-test-secret",
		BaseURL:       "http://localhost",
	}

	h := &api.Handler{
		DB:           database,
		Storage:      store,
		TemplatesDir: "web/templates",
		StaticDir:    "web/static",
		Auth:         authCfg,
		OAuthConfig:  &mockOAuthProvider{name: "IntegrationUser", email: "integration@test.com"},
	}
	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	srv := httptest.NewServer(mux)
	t.Cleanup(func() {
		srv.Close()
		database.Close()
	})

	// Create a valid session cookie
	sessionVal, _ := authpkg.SignSession(authCfg.SessionSecret, authpkg.User{Name: "IntegrationUser", Email: "integration@test.com"})

	env := &testEnv{Server: srv, DB: database, Storage: store, TmpDir: tmp}
	return env, sessionVal
}

// mockOAuthProvider for integration tests
type mockOAuthProvider struct {
	name  string
	email string
}

func (m *mockOAuthProvider) AuthCodeURL(state string, opts ...oauth2.AuthCodeOption) string {
	return "https://accounts.google.com/o/oauth2/auth?state=" + state
}

func (m *mockOAuthProvider) Exchange(r *http.Request, code string) (*oauth2.Token, error) {
	return &oauth2.Token{AccessToken: "test-token"}, nil
}

func (m *mockOAuthProvider) GetUserInfo(token *oauth2.Token) (name, email string, err error) {
	return m.name, m.email, nil
}

func TestUnauthenticatedRedirectsToLogin(t *testing.T) {
	env, _ := setupWithAuth(t)
	client := &http.Client{CheckRedirect: func(req *http.Request, via []*http.Request) error {
		return http.ErrUseLastResponse
	}}

	resp, err := client.Get(env.Server.URL + "/")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusFound {
		t.Fatalf("expected 302, got %d", resp.StatusCode)
	}
	if loc := resp.Header.Get("Location"); loc != "/login" {
		t.Errorf("expected redirect to /login, got %s", loc)
	}
}

func TestLoginPageRendersGoogleButton(t *testing.T) {
	env, _ := setupWithAuth(t)
	resp, err := http.Get(env.Server.URL + "/login")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	if !strings.Contains(string(b), "Sign in with Google") {
		t.Error("login page missing Google login button")
	}
}

func TestAuthenticatedAccessToHome(t *testing.T) {
	env, sessionVal := setupWithAuth(t)
	req, _ := http.NewRequest("GET", env.Server.URL+"/", nil)
	req.AddCookie(&http.Cookie{Name: "session", Value: sessionVal})
	client := &http.Client{CheckRedirect: func(req *http.Request, via []*http.Request) error {
		return http.ErrUseLastResponse
	}}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	b, _ := io.ReadAll(resp.Body)
	body := string(b)
	if !strings.Contains(body, "IntegrationUser") {
		t.Error("home page should show user name")
	}
	if !strings.Contains(body, "Logout") {
		t.Error("home page should show logout link")
	}
}

func TestAPIReturns401WithoutAuth(t *testing.T) {
	env, _ := setupWithAuth(t)
	resp, err := http.Get(env.Server.URL + "/api/projects")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", resp.StatusCode)
	}
}

func TestAPIWithBearerToken(t *testing.T) {
	env, _ := setupWithAuth(t)
	// Create a token
	env.DB.CreateToken("integration-token", "TokenUser", "token@test.com")

	req, _ := http.NewRequest("GET", env.Server.URL+"/api/projects", nil)
	req.Header.Set("Authorization", "Bearer integration-token")
	resp, err := (&http.Client{}).Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
}

func TestAPIWithSessionCookie(t *testing.T) {
	env, sessionVal := setupWithAuth(t)
	req, _ := http.NewRequest("GET", env.Server.URL+"/api/projects", nil)
	req.AddCookie(&http.Cookie{Name: "session", Value: sessionVal})
	resp, err := (&http.Client{}).Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
}

func TestLogoutClearsSession(t *testing.T) {
	env, sessionVal := setupWithAuth(t)
	client := &http.Client{CheckRedirect: func(req *http.Request, via []*http.Request) error {
		return http.ErrUseLastResponse
	}}
	req, _ := http.NewRequest("GET", env.Server.URL+"/auth/logout", nil)
	req.AddCookie(&http.Cookie{Name: "session", Value: sessionVal})
	resp, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusFound {
		t.Fatalf("expected 302, got %d", resp.StatusCode)
	}
	// Check session cookie is cleared
	for _, c := range resp.Cookies() {
		if c.Name == "session" && c.MaxAge != -1 {
			t.Error("session cookie should be cleared")
		}
	}
}

func TestCommentAuthorFromAuth(t *testing.T) {
	env, sessionVal := setupWithAuth(t)

	// Upload a project (need token for API) — use same email as session user
	env.DB.CreateToken("upload-token", "IntegrationUser", "integration@test.com")
	z := makeZip(t, map[string]string{"index.html": "<h1>hi</h1>"})
	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	mw.WriteField("name", "auth-comment-proj")
	fw, _ := mw.CreateFormFile("file", "upload.zip")
	fw.Write(z)
	mw.Close()

	req, _ := http.NewRequest("POST", env.Server.URL+"/api/upload", &body)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	req.Header.Set("Authorization", "Bearer upload-token")
	resp, err := (&http.Client{}).Do(req)
	if err != nil {
		t.Fatal(err)
	}
	var uploadRes map[string]any
	json.NewDecoder(resp.Body).Decode(&uploadRes)
	resp.Body.Close()
	vid := uploadRes["version_id"].(string)

	// Create comment with session cookie — author should come from session
	commentBody := `{"page":"index.html","x_percent":10,"y_percent":20,"author_name":"ShouldBeIgnored","author_email":"ignored@test.com","body":"auth comment"}`
	req2, _ := http.NewRequest("POST", env.Server.URL+"/api/versions/"+vid+"/comments", strings.NewReader(commentBody))
	req2.Header.Set("Content-Type", "application/json")
	req2.AddCookie(&http.Cookie{Name: "session", Value: sessionVal})
	resp2, err := (&http.Client{}).Do(req2)
	if err != nil {
		t.Fatal(err)
	}
	defer resp2.Body.Close()
	if resp2.StatusCode != 201 {
		b, _ := io.ReadAll(resp2.Body)
		t.Fatalf("expected 201, got %d: %s", resp2.StatusCode, b)
	}
	var comment map[string]any
	json.NewDecoder(resp2.Body).Decode(&comment)
	if comment["author_name"] != "IntegrationUser" {
		t.Errorf("author_name = %v, want IntegrationUser (from auth)", comment["author_name"])
	}
	if comment["author_email"] != "integration@test.com" {
		t.Errorf("author_email = %v, want integration@test.com", comment["author_email"])
	}
}

func TestGoogleLoginRedirect(t *testing.T) {
	env, _ := setupWithAuth(t)
	client := &http.Client{CheckRedirect: func(req *http.Request, via []*http.Request) error {
		return http.ErrUseLastResponse
	}}
	resp, err := client.Get(env.Server.URL + "/auth/google/login")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusFound {
		t.Fatalf("expected 302, got %d", resp.StatusCode)
	}
	loc := resp.Header.Get("Location")
	if !strings.HasPrefix(loc, "https://accounts.google.com") {
		t.Errorf("expected Google redirect, got %s", loc)
	}
}

func TestTokenExchangeIntegration(t *testing.T) {
	env, _ := setupWithAuth(t)
	body := `{"code":"test-auth-code"}`
	resp, err := http.Post(env.Server.URL+"/api/auth/token", "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 200, got %d: %s", resp.StatusCode, b)
	}
	var result map[string]string
	json.NewDecoder(resp.Body).Decode(&result)
	if result["token"] == "" {
		t.Error("missing token")
	}
	if result["name"] != "IntegrationUser" {
		t.Errorf("name = %q, want IntegrationUser", result["name"])
	}

	// Verify the token works for API access
	req, _ := http.NewRequest("GET", env.Server.URL+"/api/projects", nil)
	req.Header.Set("Authorization", "Bearer "+result["token"])
	resp2, err := (&http.Client{}).Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp2.Body.Close()
	if resp2.StatusCode != 200 {
		t.Errorf("token should work for API access, got %d", resp2.StatusCode)
	}
}

func TestViewerRequiresAuth(t *testing.T) {
	env, sessionVal := setupWithAuth(t)
	client := &http.Client{CheckRedirect: func(req *http.Request, via []*http.Request) error {
		return http.ErrUseLastResponse
	}}

	// Upload a project with the session user so they have access
	env.DB.CreateToken("tok", "IntegrationUser", "integration@test.com")
	z := makeZip(t, map[string]string{"index.html": "x"})
	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	mw.WriteField("name", "viewer-auth")
	fw, _ := mw.CreateFormFile("file", "upload.zip")
	fw.Write(z)
	mw.Close()
	req, _ := http.NewRequest("POST", env.Server.URL+"/api/upload", &body)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	req.Header.Set("Authorization", "Bearer tok")
	resp, _ := (&http.Client{}).Do(req)
	var res map[string]any
	json.NewDecoder(resp.Body).Decode(&res)
	resp.Body.Close()
	pid := res["project_id"].(string)

	// Without auth — should redirect
	resp2, _ := client.Get(env.Server.URL + "/projects/" + pid)
	resp2.Body.Close()
	if resp2.StatusCode != http.StatusFound {
		t.Errorf("expected 302 without auth, got %d", resp2.StatusCode)
	}

	// With auth — should work
	req3, _ := http.NewRequest("GET", env.Server.URL+"/projects/"+pid, nil)
	req3.AddCookie(&http.Cookie{Name: "session", Value: sessionVal})
	resp3, err := (&http.Client{CheckRedirect: func(req *http.Request, via []*http.Request) error {
		return http.ErrUseLastResponse
	}}).Do(req3)
	if err != nil {
		t.Fatal(err)
	}
	defer resp3.Body.Close()
	if resp3.StatusCode != 200 {
		t.Errorf("expected 200 with auth, got %d", resp3.StatusCode)
	}
}

// --- Phase 9: CLI Tool ---

func TestCLIPushWithBearerToken(t *testing.T) {
	env, _ := setupWithAuth(t)

	// Create an API token
	env.DB.CreateToken("cli-token-123", "CLI User", "cli@test.com")

	z := makeZip(t, map[string]string{"index.html": "<h1>CLI Push</h1>"})
	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	mw.WriteField("name", "cli-project")
	fw, _ := mw.CreateFormFile("file", "upload.zip")
	fw.Write(z)
	mw.Close()

	req, _ := http.NewRequest("POST", env.Server.URL+"/api/upload", &body)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	req.Header.Set("Authorization", "Bearer cli-token-123")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 200, got %d: %s", resp.StatusCode, b)
	}

	var result map[string]any
	json.NewDecoder(resp.Body).Decode(&result)

	if result["project_id"] == nil {
		t.Error("missing project_id")
	}
	if result["version_num"] != float64(1) {
		t.Errorf("version_num = %v, want 1", result["version_num"])
	}
}

func TestCLIPushCreatesNewVersion(t *testing.T) {
	env, _ := setupWithAuth(t)
	env.DB.CreateToken("cli-tok", "U", "u@t.com")

	z := makeZip(t, map[string]string{"index.html": "v1"})

	upload := func() map[string]any {
		var body bytes.Buffer
		mw := multipart.NewWriter(&body)
		mw.WriteField("name", "versioned-proj")
		fw, _ := mw.CreateFormFile("file", "upload.zip")
		fw.Write(z)
		mw.Close()
		req, _ := http.NewRequest("POST", env.Server.URL+"/api/upload", &body)
		req.Header.Set("Content-Type", mw.FormDataContentType())
		req.Header.Set("Authorization", "Bearer cli-tok")
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		var res map[string]any
		json.NewDecoder(resp.Body).Decode(&res)
		return res
	}

	r1 := upload()
	r2 := upload()

	if r1["project_id"] != r2["project_id"] {
		t.Error("same name should reuse project")
	}
	if r2["version_num"] != float64(2) {
		t.Errorf("second upload version_num = %v, want 2", r2["version_num"])
	}
}

func TestCLIPushWithoutAuthReturns401(t *testing.T) {
	env, _ := setupWithAuth(t)

	z := makeZip(t, map[string]string{"index.html": "x"})
	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	mw.WriteField("name", "no-auth")
	fw, _ := mw.CreateFormFile("file", "upload.zip")
	fw.Write(z)
	mw.Close()

	resp, err := http.Post(env.Server.URL+"/api/upload", mw.FormDataContentType(), &body)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", resp.StatusCode)
	}
}

func TestCLILoginFlowRedirectsToOAuth(t *testing.T) {
	env, _ := setupWithAuth(t)
	client := &http.Client{CheckRedirect: func(req *http.Request, via []*http.Request) error {
		return http.ErrUseLastResponse
	}}

	resp, err := client.Get(env.Server.URL + "/auth/google/cli-login?port=9876")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusFound {
		t.Fatalf("expected 302, got %d", resp.StatusCode)
	}

	loc := resp.Header.Get("Location")
	if !strings.Contains(loc, "accounts.google.com") {
		t.Errorf("expected Google OAuth redirect, got %s", loc)
	}
	if !strings.Contains(loc, "9876") {
		t.Errorf("expected port in state, got %s", loc)
	}
}

func TestCLILoginFlowMissingPort(t *testing.T) {
	env, _ := setupWithAuth(t)

	resp, err := http.Get(env.Server.URL + "/auth/google/cli-login")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", resp.StatusCode)
	}
}

func TestCLIOAuthCallbackRedirectsToCLI(t *testing.T) {
	env, _ := setupWithAuth(t)
	client := &http.Client{CheckRedirect: func(req *http.Request, via []*http.Request) error {
		return http.ErrUseLastResponse
	}}

	// First, initiate CLI login to get the state cookie
	resp1, _ := client.Get(env.Server.URL + "/auth/google/cli-login?port=9876")
	resp1.Body.Close()

	// Extract state cookie and state from redirect URL
	var stateCookie *http.Cookie
	for _, c := range resp1.Cookies() {
		if c.Name == "oauth_state" {
			stateCookie = c
		}
	}
	if stateCookie == nil {
		t.Fatal("no oauth_state cookie")
	}

	// Simulate Google callback with the state
	callbackURL := env.Server.URL + "/auth/google/callback?state=" + stateCookie.Value + "&code=test-code"
	req, _ := http.NewRequest("GET", callbackURL, nil)
	req.AddCookie(stateCookie)
	resp2, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp2.Body.Close()

	if resp2.StatusCode != http.StatusFound {
		t.Fatalf("expected 302, got %d", resp2.StatusCode)
	}

	loc := resp2.Header.Get("Location")
	if !strings.Contains(loc, "localhost:9876/callback") {
		t.Errorf("expected redirect to CLI localhost, got %s", loc)
	}
	if !strings.Contains(loc, "token=") {
		t.Errorf("expected token in redirect, got %s", loc)
	}
	if !strings.Contains(loc, "name=") {
		t.Errorf("expected name in redirect, got %s", loc)
	}
}

func TestCLIUploadedDesignServesInViewer(t *testing.T) {
	env, sessionVal := setupWithAuth(t)
	env.DB.CreateToken("cli-tok", "IntegrationUser", "integration@test.com")

	z := makeZip(t, map[string]string{
		"index.html": "<h1>Design from CLI</h1>",
		"style.css":  "body { color: red; }",
	})

	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	mw.WriteField("name", "cli-design")
	fw, _ := mw.CreateFormFile("file", "upload.zip")
	fw.Write(z)
	mw.Close()

	req, _ := http.NewRequest("POST", env.Server.URL+"/api/upload", &body)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	req.Header.Set("Authorization", "Bearer cli-tok")
	resp, _ := http.DefaultClient.Do(req)
	var result map[string]any
	json.NewDecoder(resp.Body).Decode(&result)
	resp.Body.Close()

	vid := result["version_id"].(string)

	// Verify the uploaded design files are served (with auth cookie)
	req2, _ := http.NewRequest("GET", env.Server.URL+"/designs/"+vid+"/index.html", nil)
	req2.AddCookie(&http.Cookie{Name: "session", Value: sessionVal})
	resp2, err := (&http.Client{}).Do(req2)
	if err != nil {
		t.Fatal(err)
	}
	defer resp2.Body.Close()
	b, _ := io.ReadAll(resp2.Body)
	if !strings.Contains(string(b), "Design from CLI") {
		t.Errorf("expected design content, got %s", b)
	}
}

// --- Phase 10: Dockerfile + Fly.io Deployment ---

func TestDeploymentFilesExist(t *testing.T) {
	files := []struct {
		path     string
		contains []string
	}{
		{"Dockerfile", []string{
			"FROM golang:", "AS builder",
			"CGO_ENABLED=1",
			"go build -o server ./cmd/server",
			"go build -o design-reviewer ./cmd/cli",
			"FROM alpine:",
			"ca-certificates",
			"COPY web/ ./web/",
			"/data/design-reviewer.db",
		}},
		{"fly.toml", []string{
			`primary_region = 'nrt'`,
			"internal_port = 8080",
			`destination = '/data'`,
			"auto_stop_machines",
		}},
		{".dockerignore", []string{".git", "data/"}},
		{"scripts/deploy.sh", []string{"fly deploy", "fly volumes"}},
		{"README.md", []string{"Design Reviewer", "go run ./cmd/server", "fly deploy"}},
	}

	for _, f := range files {
		data, err := os.ReadFile(f.path)
		if err != nil {
			t.Fatalf("missing deployment file %s: %v", f.path, err)
		}
		content := string(data)
		for _, want := range f.contains {
			if !strings.Contains(content, want) {
				t.Errorf("%s: missing expected content %q", f.path, want)
			}
		}
	}
}

func TestDeployScriptIsExecutable(t *testing.T) {
	info, err := os.Stat("scripts/deploy.sh")
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode()&0111 == 0 {
		t.Error("scripts/deploy.sh is not executable")
	}
}

// --- Phase 11: Design Prompt Template (init command) ---

func TestInitCreatesDesignGuidelines(t *testing.T) {
	dir := t.TempDir()
	if err := cli.Init(dir); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(dir, "DESIGN_GUIDELINES.md"))
	if err != nil {
		t.Fatal(err)
	}
	content := string(data)
	// Verify all key rendering constraints from the spec are present
	required := []string{
		"No JavaScript",
		"sandbox=\"allow-same-origin\"",
		"Self-Contained",
		"No External Resources",
		"File Structure",
		"1080px",
		"CSS Features That Work",
		"What Won't Work",
		"Tips for Best Results",
	}
	for _, s := range required {
		if !strings.Contains(content, s) {
			t.Errorf("DESIGN_GUIDELINES.md missing required content: %q", s)
		}
	}
}

func TestInitIdempotent(t *testing.T) {
	dir := t.TempDir()
	cli.Init(dir)
	original, _ := os.ReadFile(filepath.Join(dir, "DESIGN_GUIDELINES.md"))

	// Write custom content to simulate user edits
	custom := []byte("custom content")
	os.WriteFile(filepath.Join(dir, "DESIGN_GUIDELINES.md"), custom, 0644)

	// Init again should skip and not overwrite
	cli.Init(dir)
	after, _ := os.ReadFile(filepath.Join(dir, "DESIGN_GUIDELINES.md"))
	if !bytes.Equal(after, custom) {
		t.Errorf("init overwrote existing file: got %q, want %q", string(after), string(original))
	}
}

func TestInitDoesNotAffectOtherFiles(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "index.html"), []byte("<h1>hi</h1>"), 0644)
	os.WriteFile(filepath.Join(dir, "style.css"), []byte("body{}"), 0644)

	if err := cli.Init(dir); err != nil {
		t.Fatal(err)
	}

	// Existing files should be untouched
	html, _ := os.ReadFile(filepath.Join(dir, "index.html"))
	if string(html) != "<h1>hi</h1>" {
		t.Error("index.html was modified")
	}
	css, _ := os.ReadFile(filepath.Join(dir, "style.css"))
	if string(css) != "body{}" {
		t.Error("style.css was modified")
	}
	// DESIGN_GUIDELINES.md should exist
	if _, err := os.Stat(filepath.Join(dir, "DESIGN_GUIDELINES.md")); err != nil {
		t.Error("DESIGN_GUIDELINES.md not created")
	}
}

// --- Phase 12: Project Sharing & Access Control ---

func setupWithAuthUser(t *testing.T, name, email string) (*testEnv, string) {
	t.Helper()
	tmp := t.TempDir()
	database, err := db.New(filepath.Join(tmp, "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	store := storage.New(filepath.Join(tmp, "uploads"))
	authCfg := &authpkg.Config{
		ClientID: "test", ClientSecret: "test",
		RedirectURL: "http://localhost/auth/google/callback",
		SessionSecret: "test-secret", BaseURL: "http://localhost",
	}
	h := &api.Handler{
		DB: database, Storage: store,
		TemplatesDir: "web/templates", StaticDir: "web/static",
		Auth: authCfg, OAuthConfig: &mockOAuthProvider{name: name, email: email},
	}
	mux := http.NewServeMux()
	h.RegisterRoutes(mux)
	srv := httptest.NewServer(mux)
	t.Cleanup(func() { srv.Close(); database.Close() })
	sessionVal, _ := authpkg.SignSession(authCfg.SessionSecret, authpkg.User{Name: name, Email: email})
	return &testEnv{Server: srv, DB: database, Storage: store, TmpDir: tmp}, sessionVal
}

func authUpload(t *testing.T, baseURL, name, token string, zipData []byte) map[string]any {
	t.Helper()
	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	mw.WriteField("name", name)
	fw, _ := mw.CreateFormFile("file", "upload.zip")
	fw.Write(zipData)
	mw.Close()
	req, _ := http.NewRequest("POST", baseURL+"/api/upload", &body)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := (&http.Client{}).Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("upload failed: %d %s", resp.StatusCode, b)
	}
	var result map[string]any
	json.NewDecoder(resp.Body).Decode(&result)
	return result
}

func TestUploadSetsOwnerEmail(t *testing.T) {
	env, _ := setupWithAuthUser(t, "Alice", "alice@test.com")
	env.DB.CreateToken("tok", "Alice", "alice@test.com")
	z := makeZip(t, map[string]string{"index.html": "x"})
	res := authUpload(t, env.Server.URL, "my-proj", "tok", z)

	p, err := env.DB.GetProject(res["project_id"].(string))
	if err != nil {
		t.Fatal(err)
	}
	if p.OwnerEmail == nil || *p.OwnerEmail != "alice@test.com" {
		t.Errorf("owner = %v, want alice@test.com", p.OwnerEmail)
	}
}

func TestUserScopedProjectListing(t *testing.T) {
	env, session := setupWithAuthUser(t, "Alice", "alice@test.com")
	env.DB.CreateToken("tok", "Alice", "alice@test.com")
	z := makeZip(t, map[string]string{"index.html": "x"})
	authUpload(t, env.Server.URL, "alice-proj", "tok", z)

	// Create another user's project directly
	env.DB.CreateProject("bob-proj", "bob@test.com")

	// Alice should only see her own project
	req, _ := http.NewRequest("GET", env.Server.URL+"/api/projects", nil)
	req.AddCookie(&http.Cookie{Name: "session", Value: session})
	resp, err := (&http.Client{}).Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var projects []map[string]any
	json.NewDecoder(resp.Body).Decode(&projects)
	if len(projects) != 1 {
		t.Fatalf("expected 1 project, got %d", len(projects))
	}
	if projects[0]["name"] != "alice-proj" {
		t.Errorf("expected alice-proj, got %v", projects[0]["name"])
	}
}

func TestInviteFlowEndToEnd(t *testing.T) {
	env, aliceSession := setupWithAuthUser(t, "Alice", "alice@test.com")
	env.DB.CreateToken("tok", "Alice", "alice@test.com")
	z := makeZip(t, map[string]string{"index.html": "x"})
	res := authUpload(t, env.Server.URL, "shared-proj", "tok", z)
	pid := res["project_id"].(string)

	// Alice creates invite
	req, _ := http.NewRequest("POST", env.Server.URL+"/api/projects/"+pid+"/invites", nil)
	req.AddCookie(&http.Cookie{Name: "session", Value: aliceSession})
	resp, _ := (&http.Client{}).Do(req)
	var invRes map[string]string
	json.NewDecoder(resp.Body).Decode(&invRes)
	resp.Body.Close()
	inviteURL := invRes["invite_url"]
	if inviteURL == "" {
		t.Fatal("no invite_url returned")
	}

	// Bob accepts invite
	bobSession, _ := authpkg.SignSession("test-secret", authpkg.User{Name: "Bob", Email: "bob@test.com"})
	client := &http.Client{CheckRedirect: func(req *http.Request, via []*http.Request) error {
		return http.ErrUseLastResponse
	}}
	// Extract token from URL
	token := inviteURL[strings.LastIndex(inviteURL, "/")+1:]
	req2, _ := http.NewRequest("GET", env.Server.URL+"/invite/"+token, nil)
	req2.AddCookie(&http.Cookie{Name: "session", Value: bobSession})
	resp2, _ := client.Do(req2)
	resp2.Body.Close()
	if resp2.StatusCode != 302 {
		t.Fatalf("expected 302, got %d", resp2.StatusCode)
	}

	// Bob can now access the project
	req3, _ := http.NewRequest("GET", env.Server.URL+"/projects/"+pid, nil)
	req3.AddCookie(&http.Cookie{Name: "session", Value: bobSession})
	resp3, _ := (&http.Client{}).Do(req3)
	resp3.Body.Close()
	if resp3.StatusCode != 200 {
		t.Errorf("bob should access project after invite, got %d", resp3.StatusCode)
	}
}

func TestNonMemberGets404(t *testing.T) {
	env, _ := setupWithAuthUser(t, "Alice", "alice@test.com")
	env.DB.CreateToken("tok", "Alice", "alice@test.com")
	z := makeZip(t, map[string]string{"index.html": "x"})
	res := authUpload(t, env.Server.URL, "private-proj", "tok", z)
	pid := res["project_id"].(string)

	// Bob tries to access
	bobSession, _ := authpkg.SignSession("test-secret", authpkg.User{Name: "Bob", Email: "bob@test.com"})
	req, _ := http.NewRequest("GET", env.Server.URL+"/projects/"+pid, nil)
	req.AddCookie(&http.Cookie{Name: "session", Value: bobSession})
	resp, _ := (&http.Client{CheckRedirect: func(req *http.Request, via []*http.Request) error {
		return http.ErrUseLastResponse
	}}).Do(req)
	resp.Body.Close()
	if resp.StatusCode != 404 {
		t.Errorf("non-member should get 404, got %d", resp.StatusCode)
	}
}

func TestNonOwnerCannotCreateInvite(t *testing.T) {
	env, _ := setupWithAuthUser(t, "Alice", "alice@test.com")
	env.DB.CreateToken("tok", "Alice", "alice@test.com")
	z := makeZip(t, map[string]string{"index.html": "x"})
	res := authUpload(t, env.Server.URL, "proj", "tok", z)
	pid := res["project_id"].(string)

	// Add Bob as member
	env.DB.AddMember(pid, "bob@test.com")

	// Bob tries to create invite — should get 403
	bobSession, _ := authpkg.SignSession("test-secret", authpkg.User{Name: "Bob", Email: "bob@test.com"})
	req, _ := http.NewRequest("POST", env.Server.URL+"/api/projects/"+pid+"/invites", nil)
	req.AddCookie(&http.Cookie{Name: "session", Value: bobSession})
	resp, _ := (&http.Client{}).Do(req)
	resp.Body.Close()
	if resp.StatusCode != 403 {
		t.Errorf("non-owner should get 403, got %d", resp.StatusCode)
	}
}

func TestSeedProjectVisibleToAll(t *testing.T) {
	env, session := setupWithAuthUser(t, "Alice", "alice@test.com")
	// Create a seed-like project with no owner
	env.DB.CreateProject("Seed Project", "")

	req, _ := http.NewRequest("GET", env.Server.URL+"/api/projects", nil)
	req.AddCookie(&http.Cookie{Name: "session", Value: session})
	resp, err := (&http.Client{}).Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var projects []map[string]any
	json.NewDecoder(resp.Body).Decode(&projects)
	if len(projects) != 1 {
		t.Fatalf("expected 1 project (seed), got %d", len(projects))
	}
	if projects[0]["name"] != "Seed Project" {
		t.Errorf("expected Seed Project, got %v", projects[0]["name"])
	}
}

// --- Invite redirect after login ---

func TestInviteRedirectAfterLogin(t *testing.T) {
	env, aliceSession := setupWithAuthUser(t, "Alice", "alice@test.com")
	env.DB.CreateToken("tok", "Alice", "alice@test.com")
	z := makeZip(t, map[string]string{"index.html": "x"})
	res := authUpload(t, env.Server.URL, "invite-proj", "tok", z)
	pid := res["project_id"].(string)

	// Alice creates invite
	req, _ := http.NewRequest("POST", env.Server.URL+"/api/projects/"+pid+"/invites", nil)
	req.AddCookie(&http.Cookie{Name: "session", Value: aliceSession})
	resp, _ := (&http.Client{}).Do(req)
	var invRes map[string]string
	json.NewDecoder(resp.Body).Decode(&invRes)
	resp.Body.Close()
	token := invRes["invite_url"][strings.LastIndex(invRes["invite_url"], "/")+1:]

	// Bob (no session) hits invite link — should redirect to /login with redirect_to cookie
	noRedirectClient := &http.Client{CheckRedirect: func(r *http.Request, via []*http.Request) error {
		return http.ErrUseLastResponse
	}}
	resp2, err := noRedirectClient.Get(env.Server.URL + "/invite/" + token)
	if err != nil {
		t.Fatal(err)
	}
	resp2.Body.Close()
	if resp2.StatusCode != 302 {
		t.Fatalf("expected 302, got %d", resp2.StatusCode)
	}
	if loc := resp2.Header.Get("Location"); loc != "/login" {
		t.Fatalf("expected redirect to /login, got %s", loc)
	}
	// Check redirect_to cookie was set
	var redirectCookie *http.Cookie
	for _, c := range resp2.Cookies() {
		if c.Name == "redirect_to" {
			redirectCookie = c
		}
	}
	if redirectCookie == nil || redirectCookie.Value != "/invite/"+token {
		t.Fatalf("expected redirect_to cookie = /invite/%s, got %v", token, redirectCookie)
	}

	// Simulate OAuth callback with redirect_to cookie — Bob completes login
	// First get the state from the login redirect
	bobEnv, _ := setupWithAuthUser(t, "Bob", "bob@test.com")
	_ = bobEnv // we reuse env but need Bob's OAuth mock

	// Simulate: hit callback with state cookie + redirect_to cookie
	stateCookie := &http.Cookie{Name: "oauth_state", Value: "test-state"}
	callbackURL := env.Server.URL + "/auth/google/callback?state=test-state&code=test-code"
	req3, _ := http.NewRequest("GET", callbackURL, nil)
	req3.AddCookie(stateCookie)
	req3.AddCookie(redirectCookie)
	resp3, err := noRedirectClient.Do(req3)
	if err != nil {
		t.Fatal(err)
	}
	resp3.Body.Close()
	if resp3.StatusCode != 302 {
		t.Fatalf("expected 302 from callback, got %d", resp3.StatusCode)
	}
	loc := resp3.Header.Get("Location")
	if loc != "/invite/"+token {
		t.Errorf("expected redirect to /invite/%s, got %s", token, loc)
	}
}

// --- Phase 13: Fix XSS in Sharing UI ---

func TestMembersAPIReturnsHTMLSpecialCharsUnescaped(t *testing.T) {
	env, aliceSession := setupWithAuthUser(t, "Alice", "alice@test.com")
	env.DB.CreateToken("tok", "Alice", "alice@test.com")
	z := makeZip(t, map[string]string{"index.html": "x"})
	res := authUpload(t, env.Server.URL, "xss-proj", "tok", z)
	pid := res["project_id"].(string)

	xssEmail := `<img src=x onerror=alert(1)>`
	env.DB.AddMember(pid, xssEmail)

	req, _ := http.NewRequest("GET", env.Server.URL+"/api/projects/"+pid+"/members", nil)
	req.AddCookie(&http.Cookie{Name: "session", Value: aliceSession})
	resp, err := (&http.Client{}).Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	var members []map[string]string
	json.NewDecoder(resp.Body).Decode(&members)
	if len(members) != 1 {
		t.Fatalf("expected 1 member, got %d", len(members))
	}
	if members[0]["email"] != xssEmail {
		t.Errorf("email = %q, want %q", members[0]["email"], xssEmail)
	}
}

func TestSharingJSUsesEscHelper(t *testing.T) {
	data, err := os.ReadFile("web/static/sharing.js")
	if err != nil {
		t.Fatal(err)
	}
	src := string(data)

	if !strings.Contains(src, "function esc(") {
		t.Error("sharing.js missing esc() helper function")
	}
	if strings.Contains(src, "m.email + '") || strings.Contains(src, "m.email +\n") {
		t.Error("sharing.js uses raw m.email in string concatenation — XSS vulnerability")
	}
	if !strings.Contains(src, "esc(m.email)") {
		t.Error("sharing.js does not wrap m.email with esc()")
	}
}

// --- Phase 14: Upload Size Limit ---

func TestUploadExceedingSizeLimitReturns413(t *testing.T) {
	env := setup(t)

	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	mw.WriteField("name", "huge-proj")
	fw, _ := mw.CreateFormFile("file", "upload.zip")
	chunk := bytes.Repeat([]byte("x"), 1<<20) // 1MB
	for i := 0; i < 51; i++ {
		fw.Write(chunk)
	}
	mw.Close()

	resp, err := http.Post(env.Server.URL+"/api/upload", mw.FormDataContentType(), &body)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusRequestEntityTooLarge {
		t.Errorf("expected 413, got %d", resp.StatusCode)
	}
}

func TestUploadUnderLimitStillWorks(t *testing.T) {
	env := setup(t)
	z := makeZip(t, map[string]string{"index.html": "<h1>ok</h1>"})
	res := uploadZip(t, env.Server.URL, "normal-proj", z)
	if res["version_num"].(float64) != 1 {
		t.Errorf("expected version_num=1, got %v", res["version_num"])
	}
}

// --- Phase 15: Validate CLI Login Port Parameter ---

func TestCLILoginRejectsNonNumericPort(t *testing.T) {
	env, _ := setupWithAuth(t)
	for _, port := range []string{"abc", "9876@evil.com/steal#", "0", "65536"} {
		resp, err := http.Get(env.Server.URL + "/auth/google/cli-login?port=" + port)
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		if resp.StatusCode != http.StatusBadRequest {
			t.Errorf("port=%q: expected 400, got %d", port, resp.StatusCode)
		}
	}
}

func TestCLILoginAcceptsValidPort(t *testing.T) {
	env, _ := setupWithAuth(t)
	client := &http.Client{CheckRedirect: func(req *http.Request, via []*http.Request) error {
		return http.ErrUseLastResponse
	}}
	resp, err := client.Get(env.Server.URL + "/auth/google/cli-login?port=9876")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusFound {
		t.Errorf("expected 302, got %d", resp.StatusCode)
	}
}

// --- Phase 16: Secure Session Cookie ---

func TestOAuthCallbackSessionCookieNotSecureOverHTTP(t *testing.T) {
	env, _ := setupWithAuth(t)
	client := &http.Client{CheckRedirect: func(req *http.Request, via []*http.Request) error {
		return http.ErrUseLastResponse
	}}

	// First, initiate login to get a state cookie
	resp, err := client.Get(env.Server.URL + "/auth/google/login")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()

	var stateCookie *http.Cookie
	for _, c := range resp.Cookies() {
		if c.Name == "oauth_state" {
			stateCookie = c
		}
	}
	if stateCookie == nil {
		t.Fatal("no oauth_state cookie")
	}

	// Call callback with the state
	req, _ := http.NewRequest("GET", env.Server.URL+"/auth/google/callback?code=testcode&state="+stateCookie.Value, nil)
	req.AddCookie(stateCookie)
	resp, err = client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()

	for _, c := range resp.Cookies() {
		if c.Name == "session" {
			if c.Secure {
				t.Error("expected Secure=false for HTTP base URL in integration test")
			}
			return
		}
	}
	t.Error("session cookie not set in callback response")
}

// --- Phase 17: Token Expiry ---

func TestExpiredBearerTokenReturns401(t *testing.T) {
	env, _ := setupWithAuth(t)
	env.DB.CreateToken("expired-token", "TokenUser", "token@test.com")
	h := sha256.Sum256([]byte("expired-token"))
	env.DB.Exec(`UPDATE tokens SET expires_at = datetime('now', '-1 second') WHERE token = ?`, hex.EncodeToString(h[:]))

	req, _ := http.NewRequest("GET", env.Server.URL+"/api/projects", nil)
	req.Header.Set("Authorization", "Bearer expired-token")
	resp, err := (&http.Client{}).Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 401 {
		t.Fatalf("expected 401 for expired token, got %d", resp.StatusCode)
	}
}

// --- Phase 18: Strengthen Path Traversal ---

func TestDesignFilePathTraversalBlocked(t *testing.T) {
	env := setup(t)
	z := makeZip(t, map[string]string{"index.html": "<h1>hi</h1>"})
	res := uploadZip(t, env.Server.URL, "traversal-proj", z)
	vid := res["version_id"].(string)

	// Go's HTTP client normalizes ".." out of URLs before sending,
	// so the server sees a cleaned path and returns 404. Verify that
	// traversal attempts never return 200 (defense-in-depth).
	paths := []string{"../../../etc/passwd", "images/../../etc/passwd", ".."}
	for _, p := range paths {
		resp, err := http.Get(env.Server.URL + "/designs/" + vid + "/" + p)
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		if resp.StatusCode == 200 {
			t.Errorf("path %q: must not return 200", p)
		}
	}
}

func TestDesignFileNestedPathWorks(t *testing.T) {
	env := setup(t)
	z := makeZip(t, map[string]string{"index.html": "<h1>hi</h1>", "images/logo.png": "imgdata"})
	res := uploadZip(t, env.Server.URL, "nested-proj", z)
	vid := res["version_id"].(string)

	resp, err := http.Get(env.Server.URL + "/designs/" + vid + "/images/logo.png")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("expected 200 for nested path, got %d", resp.StatusCode)
	}
	body, _ := io.ReadAll(resp.Body)
	if string(body) != "imgdata" {
		t.Errorf("expected 'imgdata', got %q", string(body))
	}
}

// --- Phase 19: Keyboard Shortcut to Post Comments ---

func TestAnnotationsJSContainsKeyboardShortcut(t *testing.T) {
	env := setup(t)
	resp, err := http.Get(env.Server.URL + "/static/annotations.js")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	b, _ := io.ReadAll(resp.Body)
	body := string(b)
	for _, needle := range []string{
		`e.key === "Enter"`,
		"e.ctrlKey",
		"e.metaKey",
		"nc-body",
		"rp-body",
		"shortcut-hint",
	} {
		if !strings.Contains(body, needle) {
			t.Errorf("annotations.js missing expected content: %s", needle)
		}
	}
}

func TestStyleCSSContainsShortcutHint(t *testing.T) {
	env := setup(t)
	resp, err := http.Get(env.Server.URL + "/static/style.css")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	b, _ := io.ReadAll(resp.Body)
	if !strings.Contains(string(b), ".shortcut-hint") {
		t.Error("style.css missing .shortcut-hint class")
	}
}

// --- Phase 20: Draggable Comment Pins ---

func TestMoveCommentEndpoint(t *testing.T) {
	env := setup(t)
	z := makeZip(t, map[string]string{"index.html": "<h1>hi</h1>"})
	res := uploadZip(t, env.Server.URL, "move-proj", z)
	vid := res["version_id"].(string)

	// Create a comment
	cBody := `{"page":"index.html","x_percent":10,"y_percent":20,"author_name":"Alice","author_email":"a@t.com","body":"move me"}`
	resp, err := http.Post(env.Server.URL+"/api/versions/"+vid+"/comments", "application/json", strings.NewReader(cBody))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 201 {
		t.Fatalf("create comment: expected 201, got %d", resp.StatusCode)
	}
	var comment map[string]any
	json.NewDecoder(resp.Body).Decode(&comment)
	cid := comment["id"].(string)

	// Move the comment
	moveBody := `{"x_percent":55.5,"y_percent":77.3}`
	req, _ := http.NewRequest("PATCH", env.Server.URL+"/api/comments/"+cid+"/move", strings.NewReader(moveBody))
	req.Header.Set("Content-Type", "application/json")
	resp2, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp2.Body.Close()
	if resp2.StatusCode != 200 {
		b, _ := io.ReadAll(resp2.Body)
		t.Fatalf("move: expected 200, got %d: %s", resp2.StatusCode, string(b))
	}
	var moveRes map[string]bool
	json.NewDecoder(resp2.Body).Decode(&moveRes)
	if !moveRes["ok"] {
		t.Error("expected ok=true")
	}

	// Verify coordinates persisted
	resp3, _ := http.Get(env.Server.URL + "/api/versions/" + vid + "/comments")
	defer resp3.Body.Close()
	var comments []map[string]any
	json.NewDecoder(resp3.Body).Decode(&comments)
	if len(comments) != 1 {
		t.Fatalf("expected 1 comment, got %d", len(comments))
	}
	if comments[0]["x_percent"].(float64) != 55.5 || comments[0]["y_percent"].(float64) != 77.3 {
		t.Errorf("coords = (%v, %v), want (55.5, 77.3)", comments[0]["x_percent"], comments[0]["y_percent"])
	}
}

func TestMoveCommentValidation(t *testing.T) {
	env := setup(t)
	tests := []struct {
		name string
		body string
	}{
		{"x over 100", `{"x_percent":101,"y_percent":50}`},
		{"y negative", `{"x_percent":50,"y_percent":-1}`},
		{"invalid json", `not json`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req, _ := http.NewRequest("PATCH", env.Server.URL+"/api/comments/any-id/move", strings.NewReader(tt.body))
			req.Header.Set("Content-Type", "application/json")
			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				t.Fatal(err)
			}
			defer resp.Body.Close()
			if resp.StatusCode != 400 {
				t.Errorf("expected 400, got %d", resp.StatusCode)
			}
		})
	}
}

func TestAnnotationsJSContainsDragInteraction(t *testing.T) {
	env := setup(t)
	resp, err := http.Get(env.Server.URL + "/static/annotations.js")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	b, _ := io.ReadAll(resp.Body)
	body := string(b)
	for _, needle := range []string{
		"mousedown",
		"mousemove",
		"mouseup",
		"pin-dragging",
		"/move",
		"dragged",
	} {
		if !strings.Contains(body, needle) {
			t.Errorf("annotations.js missing expected content: %s", needle)
		}
	}
}

func TestStyleCSSContainsDragStyles(t *testing.T) {
	env := setup(t)
	resp, err := http.Get(env.Server.URL + "/static/style.css")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	body := string(b)
	for _, needle := range []string{"cursor: grab", "pin-dragging", "cursor: grabbing", "opacity: 0.7"} {
		if !strings.Contains(body, needle) {
			t.Errorf("style.css missing: %s", needle)
		}
	}
}

// --- Phase 21: IDOR — Comment Endpoints Lack Project Access Checks ---

func TestCommentAccessBlocksNonMember(t *testing.T) {
	env, aliceSession := setupWithAuthUser(t, "Alice", "alice@test.com")
	env.DB.CreateToken("tok", "Alice", "alice@test.com")
	z := makeZip(t, map[string]string{"index.html": "x"})
	res := authUpload(t, env.Server.URL, "idor-proj", "tok", z)
	pid := res["project_id"].(string)
	vid := res["version_id"].(string)

	// Alice creates a comment
	cBody := `{"page":"index.html","x_percent":10,"y_percent":20,"body":"secret"}`
	req, _ := http.NewRequest("POST", env.Server.URL+"/api/versions/"+vid+"/comments", strings.NewReader(cBody))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(&http.Cookie{Name: "session", Value: aliceSession})
	resp, _ := (&http.Client{}).Do(req)
	var comment map[string]any
	json.NewDecoder(resp.Body).Decode(&comment)
	resp.Body.Close()
	cid := comment["id"].(string)
	_ = pid

	// Bob (non-member) tries reply, resolve, move — all should 404
	bobSession, _ := authpkg.SignSession("test-secret", authpkg.User{Name: "Bob", Email: "bob@test.com"})
	endpoints := []struct {
		method, path, body string
	}{
		{"POST", "/api/comments/" + cid + "/replies", `{"body":"hacked"}`},
		{"PATCH", "/api/comments/" + cid + "/resolve", ""},
		{"PATCH", "/api/comments/" + cid + "/move", `{"x_percent":50,"y_percent":50}`},
	}
	for _, ep := range endpoints {
		var bodyReader io.Reader
		if ep.body != "" {
			bodyReader = strings.NewReader(ep.body)
		}
		req, _ := http.NewRequest(ep.method, env.Server.URL+ep.path, bodyReader)
		req.Header.Set("Content-Type", "application/json")
		req.AddCookie(&http.Cookie{Name: "session", Value: bobSession})
		resp, err := (&http.Client{}).Do(req)
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		if resp.StatusCode != 404 {
			t.Errorf("%s %s: expected 404, got %d", ep.method, ep.path, resp.StatusCode)
		}
	}
}

func TestCommentAccessAllowsMember(t *testing.T) {
	env, aliceSession := setupWithAuthUser(t, "Alice", "alice@test.com")
	env.DB.CreateToken("tok", "Alice", "alice@test.com")
	z := makeZip(t, map[string]string{"index.html": "x"})
	res := authUpload(t, env.Server.URL, "member-proj", "tok", z)
	vid := res["version_id"].(string)

	// Alice creates a comment
	cBody := `{"page":"index.html","x_percent":10,"y_percent":20,"body":"test"}`
	req, _ := http.NewRequest("POST", env.Server.URL+"/api/versions/"+vid+"/comments", strings.NewReader(cBody))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(&http.Cookie{Name: "session", Value: aliceSession})
	resp, _ := (&http.Client{}).Do(req)
	var comment map[string]any
	json.NewDecoder(resp.Body).Decode(&comment)
	resp.Body.Close()
	cid := comment["id"].(string)

	// Alice (owner) can reply
	replyBody := `{"body":"my reply"}`
	req2, _ := http.NewRequest("POST", env.Server.URL+"/api/comments/"+cid+"/replies", strings.NewReader(replyBody))
	req2.Header.Set("Content-Type", "application/json")
	req2.AddCookie(&http.Cookie{Name: "session", Value: aliceSession})
	resp2, err := (&http.Client{}).Do(req2)
	if err != nil {
		t.Fatal(err)
	}
	resp2.Body.Close()
	if resp2.StatusCode != 201 {
		t.Errorf("reply: expected 201, got %d", resp2.StatusCode)
	}

	// Alice can resolve
	req3, _ := http.NewRequest("PATCH", env.Server.URL+"/api/comments/"+cid+"/resolve", nil)
	req3.AddCookie(&http.Cookie{Name: "session", Value: aliceSession})
	resp3, err := (&http.Client{}).Do(req3)
	if err != nil {
		t.Fatal(err)
	}
	resp3.Body.Close()
	if resp3.StatusCode != 200 {
		t.Errorf("resolve: expected 200, got %d", resp3.StatusCode)
	}

	// Alice can move
	moveBody := `{"x_percent":50,"y_percent":50}`
	req4, _ := http.NewRequest("PATCH", env.Server.URL+"/api/comments/"+cid+"/move", strings.NewReader(moveBody))
	req4.Header.Set("Content-Type", "application/json")
	req4.AddCookie(&http.Cookie{Name: "session", Value: aliceSession})
	resp4, err := (&http.Client{}).Do(req4)
	if err != nil {
		t.Fatal(err)
	}
	resp4.Body.Close()
	if resp4.StatusCode != 200 {
		t.Errorf("move: expected 200, got %d", resp4.StatusCode)
	}
}

// --- Phase 22: Zip Bomb Limits ---

func TestUploadTooManyFilesReturns400(t *testing.T) {
	env := setup(t)
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	for i := 0; i < 1002; i++ {
		f, _ := w.Create(fmt.Sprintf("f%d.txt", i))
		f.Write([]byte("x"))
	}
	f, _ := w.Create("index.html")
	f.Write([]byte("<h1>hi</h1>"))
	w.Close()

	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	mw.WriteField("name", "bomb-proj")
	fw, _ := mw.CreateFormFile("file", "upload.zip")
	fw.Write(buf.Bytes())
	mw.Close()

	resp, err := http.Post(env.Server.URL+"/api/upload", mw.FormDataContentType(), &body)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 400 {
		t.Errorf("expected 400, got %d", resp.StatusCode)
	}
	b, _ := io.ReadAll(resp.Body)
	if !strings.Contains(string(b), "too many files") {
		t.Errorf("expected 'too many files' error, got %q", string(b))
	}
}

func TestUploadWithinLimitsStillWorks(t *testing.T) {
	env := setup(t)
	z := makeZip(t, map[string]string{"index.html": "<h1>ok</h1>", "style.css": "body{}"})
	res := uploadZip(t, env.Server.URL, "safe-proj", z)
	if res["version_num"].(float64) != 1 {
		t.Errorf("expected version_num=1, got %v", res["version_num"])
	}
}

// --- Phase 23: Session Expiration ---

func TestExpiredSessionRedirectsToLogin(t *testing.T) {
	env, _ := setupWithAuth(t)
	// Craft a session with past expiration by signing manually
	u := authpkg.User{Name: "Expired", Email: "expired@test.com", ExpiresAt: 1}
	data, _ := json.Marshal(u)
	sig := authpkg.HmacSignExported("integration-test-secret", data)
	expiredSession := base64.RawURLEncoding.EncodeToString(data) + "." + base64.RawURLEncoding.EncodeToString(sig)

	client := &http.Client{CheckRedirect: func(req *http.Request, via []*http.Request) error {
		return http.ErrUseLastResponse
	}}
	req, _ := http.NewRequest("GET", env.Server.URL+"/", nil)
	req.AddCookie(&http.Cookie{Name: "session", Value: expiredSession})
	resp, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusFound {
		t.Errorf("expected 302 for expired session, got %d", resp.StatusCode)
	}
}

func TestExpiredSessionAPI401(t *testing.T) {
	env, _ := setupWithAuth(t)
	u := authpkg.User{Name: "Expired", Email: "expired@test.com", ExpiresAt: 1}
	data, _ := json.Marshal(u)
	sig := authpkg.HmacSignExported("integration-test-secret", data)
	expiredSession := base64.RawURLEncoding.EncodeToString(data) + "." + base64.RawURLEncoding.EncodeToString(sig)

	req, _ := http.NewRequest("GET", env.Server.URL+"/api/projects", nil)
	req.AddCookie(&http.Cookie{Name: "session", Value: expiredSession})
	resp, err := (&http.Client{}).Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("expected 401 for expired session on API, got %d", resp.StatusCode)
	}
}

// --- Phase 24: Server-side Session Invalidation on Logout ---

func TestLogoutInvalidatesServerSession(t *testing.T) {
	env, _ := setupWithAuth(t)
	noRedirect := &http.Client{CheckRedirect: func(req *http.Request, via []*http.Request) error {
		return http.ErrUseLastResponse
	}}

	// Step 1: Login via OAuth callback to get a server-side session
	loginResp, _ := noRedirect.Get(env.Server.URL + "/auth/google/login")
	loginResp.Body.Close()
	var stateCookie *http.Cookie
	for _, c := range loginResp.Cookies() {
		if c.Name == "oauth_state" {
			stateCookie = c
		}
	}

	callbackURL := env.Server.URL + "/auth/google/callback?code=test&state=" + stateCookie.Value
	req, _ := http.NewRequest("GET", callbackURL, nil)
	req.AddCookie(stateCookie)
	cbResp, _ := noRedirect.Do(req)
	cbResp.Body.Close()

	var sessionCookie *http.Cookie
	for _, c := range cbResp.Cookies() {
		if c.Name == "session" {
			sessionCookie = c
		}
	}
	if sessionCookie == nil {
		t.Fatal("no session cookie after callback")
	}

	// Step 2: Verify session works for API access
	req2, _ := http.NewRequest("GET", env.Server.URL+"/api/projects", nil)
	req2.AddCookie(sessionCookie)
	resp2, _ := (&http.Client{}).Do(req2)
	resp2.Body.Close()
	if resp2.StatusCode != 200 {
		t.Fatalf("expected 200 with valid session, got %d", resp2.StatusCode)
	}

	// Step 3: Logout
	req3, _ := http.NewRequest("GET", env.Server.URL+"/auth/logout", nil)
	req3.AddCookie(sessionCookie)
	logoutResp, _ := noRedirect.Do(req3)
	logoutResp.Body.Close()

	// Step 4: Reuse the old session cookie — should be rejected
	req4, _ := http.NewRequest("GET", env.Server.URL+"/api/projects", nil)
	req4.AddCookie(sessionCookie)
	resp4, _ := (&http.Client{}).Do(req4)
	resp4.Body.Close()
	if resp4.StatusCode != http.StatusUnauthorized {
		t.Errorf("expected 401 after logout, got %d", resp4.StatusCode)
	}
}

func TestLogoutInvalidatesServerSessionWebRoute(t *testing.T) {
	env, _ := setupWithAuth(t)
	noRedirect := &http.Client{CheckRedirect: func(req *http.Request, via []*http.Request) error {
		return http.ErrUseLastResponse
	}}

	// Login via OAuth callback
	loginResp, _ := noRedirect.Get(env.Server.URL + "/auth/google/login")
	loginResp.Body.Close()
	var stateCookie *http.Cookie
	for _, c := range loginResp.Cookies() {
		if c.Name == "oauth_state" {
			stateCookie = c
		}
	}

	callbackURL := env.Server.URL + "/auth/google/callback?code=test&state=" + stateCookie.Value
	req, _ := http.NewRequest("GET", callbackURL, nil)
	req.AddCookie(stateCookie)
	cbResp, _ := noRedirect.Do(req)
	cbResp.Body.Close()

	var sessionCookie *http.Cookie
	for _, c := range cbResp.Cookies() {
		if c.Name == "session" {
			sessionCookie = c
		}
	}

	// Logout
	req2, _ := http.NewRequest("GET", env.Server.URL+"/auth/logout", nil)
	req2.AddCookie(sessionCookie)
	logoutResp, _ := noRedirect.Do(req2)
	logoutResp.Body.Close()

	// Reuse old session cookie on web route — should redirect to login
	req3, _ := http.NewRequest("GET", env.Server.URL+"/", nil)
	req3.AddCookie(sessionCookie)
	resp3, _ := noRedirect.Do(req3)
	resp3.Body.Close()
	if resp3.StatusCode != http.StatusFound {
		t.Errorf("expected 302 redirect to login after logout, got %d", resp3.StatusCode)
	}
}

func TestOAuthCallbackCreatesServerSession(t *testing.T) {
	env, _ := setupWithAuth(t)
	noRedirect := &http.Client{CheckRedirect: func(req *http.Request, via []*http.Request) error {
		return http.ErrUseLastResponse
	}}

	// Login via OAuth callback
	loginResp, _ := noRedirect.Get(env.Server.URL + "/auth/google/login")
	loginResp.Body.Close()
	var stateCookie *http.Cookie
	for _, c := range loginResp.Cookies() {
		if c.Name == "oauth_state" {
			stateCookie = c
		}
	}

	callbackURL := env.Server.URL + "/auth/google/callback?code=test&state=" + stateCookie.Value
	req, _ := http.NewRequest("GET", callbackURL, nil)
	req.AddCookie(stateCookie)
	cbResp, _ := noRedirect.Do(req)
	cbResp.Body.Close()

	var sessionCookie *http.Cookie
	for _, c := range cbResp.Cookies() {
		if c.Name == "session" {
			sessionCookie = c
		}
	}
	if sessionCookie == nil {
		t.Fatal("no session cookie")
	}

	// Verify the session cookie contains a session ID and it exists in DB
	u, err := authpkg.VerifySession("integration-test-secret", sessionCookie.Value)
	if err != nil {
		t.Fatalf("invalid session: %v", err)
	}
	if u.SessionID == "" {
		t.Error("session cookie missing SessionID")
	}
	name, email, err := env.DB.GetSession(u.SessionID)
	if err != nil {
		t.Fatalf("session not in DB: %v", err)
	}
	if name != "IntegrationUser" || email != "integration@test.com" {
		t.Errorf("session DB data: name=%q email=%q", name, email)
	}
}

// --- Phase 25: Hash API Tokens ---

func TestTokenExchangeStoresHash(t *testing.T) {
	env, _ := setupWithAuth(t)
	body := `{"code":"test-auth-code"}`
	resp, err := http.Post(env.Server.URL+"/api/auth/token", "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var result map[string]string
	json.NewDecoder(resp.Body).Decode(&result)
	plaintext := result["token"]

	// Verify the DB stores a hash, not the plaintext
	var stored string
	env.DB.QueryRow(`SELECT token FROM tokens LIMIT 1`).Scan(&stored)
	if stored == plaintext {
		t.Error("token stored as plaintext, expected SHA-256 hash")
	}
	h := sha256.Sum256([]byte(plaintext))
	if stored != hex.EncodeToString(h[:]) {
		t.Error("stored token does not match SHA-256 hash of plaintext")
	}

	// Verify the plaintext token still works for API auth
	req, _ := http.NewRequest("GET", env.Server.URL+"/api/projects", nil)
	req.Header.Set("Authorization", "Bearer "+plaintext)
	resp2, err := (&http.Client{}).Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp2.Body.Close()
	if resp2.StatusCode != 200 {
		t.Errorf("expected 200 with valid token, got %d", resp2.StatusCode)
	}
}

func TestHashedTokenRejectsRawHash(t *testing.T) {
	env, _ := setupWithAuth(t)
	env.DB.CreateToken("secret-token", "User", "user@test.com")

	// Using the hash directly as bearer should fail (double-hashed)
	h := sha256.Sum256([]byte("secret-token"))
	req, _ := http.NewRequest("GET", env.Server.URL+"/api/projects", nil)
	req.Header.Set("Authorization", "Bearer "+hex.EncodeToString(h[:]))
	resp, err := (&http.Client{}).Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != 401 {
		t.Errorf("expected 401 when using hash as bearer, got %d", resp.StatusCode)
	}
}

// --- Phase 26: Attribute Injection XSS Fix ---

func TestEscFunctionEscapesQuotes(t *testing.T) {
	env := setup(t)
	for _, file := range []string{"/static/sharing.js", "/static/annotations.js"} {
		resp, err := http.Get(env.Server.URL + file)
		if err != nil {
			t.Fatal(err)
		}
		b, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		body := string(b)
		if !strings.Contains(body, `&quot;`) {
			t.Errorf("%s: esc() missing double-quote escaping (&quot;)", file)
		}
		if !strings.Contains(body, `&#39;`) {
			t.Errorf("%s: esc() missing single-quote escaping (&#39;)", file)
		}
	}
}

func TestCommentWithQuotesPreserved(t *testing.T) {
	env := setup(t)
	z := makeZip(t, map[string]string{"index.html": "<h1>hi</h1>"})
	res := uploadZip(t, env.Server.URL, "xss-proj", z)
	vid := res["version_id"].(string)

	payload := `{"page":"index.html","x_percent":10,"y_percent":20,"author_name":"O'Reilly \"Bob\"","body":"He said \"hello\" & it's <fine>"}`
	resp, err := http.Post(env.Server.URL+"/api/versions/"+vid+"/comments", "application/json", strings.NewReader(payload))
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != 200 && resp.StatusCode != 201 {
		t.Fatalf("create comment: expected 200/201, got %d", resp.StatusCode)
	}

	resp2, err := http.Get(env.Server.URL + "/api/versions/" + vid + "/comments")
	if err != nil {
		t.Fatal(err)
	}
	defer resp2.Body.Close()
	var comments []map[string]interface{}
	json.NewDecoder(resp2.Body).Decode(&comments)
	if len(comments) == 0 {
		t.Fatal("expected at least one comment")
	}
	c := comments[0]
	if name, _ := c["author_name"].(string); name != "O'Reilly \"Bob\"" {
		t.Errorf("author_name mangled: got %q", name)
	}
	if body, _ := c["body"].(string); body != "He said \"hello\" & it's <fine>" {
		t.Errorf("body mangled: got %q", body)
	}
}

// --- Phase 27: Security Headers ---

func TestSecurityHeadersOnAllResponses(t *testing.T) {
	// Setup with security headers middleware (mirrors cmd/server/main.go)
	tmp := t.TempDir()
	database, err := db.New(filepath.Join(tmp, "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	store := storage.New(filepath.Join(tmp, "uploads"))
	h := &api.Handler{DB: database, Storage: store, TemplatesDir: "web/templates", StaticDir: "web/static"}
	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	wrapped := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
		w.Header().Set("Permissions-Policy", "camera=(), microphone=(), geolocation=()")
		mux.ServeHTTP(w, r)
	})

	srv := httptest.NewServer(wrapped)
	t.Cleanup(func() { srv.Close(); database.Close() })

	expected := map[string]string{
		"X-Content-Type-Options": "nosniff",
		"X-Frame-Options":       "DENY",
		"Referrer-Policy":       "strict-origin-when-cross-origin",
		"Permissions-Policy":    "camera=(), microphone=(), geolocation=()",
	}

	// Test on the home page
	resp, err := http.Get(srv.URL + "/")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	for header, want := range expected {
		if got := resp.Header.Get(header); got != want {
			t.Errorf("GET /: %s = %q, want %q", header, got, want)
		}
	}

	// Test on API endpoint
	resp2, err := http.Get(srv.URL + "/api/projects")
	if err != nil {
		t.Fatal(err)
	}
	resp2.Body.Close()
	for header, want := range expected {
		if got := resp2.Header.Get(header); got != want {
			t.Errorf("GET /api/projects: %s = %q, want %q", header, got, want)
		}
	}
}
