package integration

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ab/design-reviewer/internal/api"
	"github.com/ab/design-reviewer/internal/db"
	"github.com/ab/design-reviewer/internal/storage"
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
	resp3, _ := http.Get(env.Server.URL + "/api/versions/" + vid + "/comments")
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
	resp3, _ := client.Do(req2)
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
	resp3, _ := http.Get(env.Server.URL + "/api/versions/" + vid2 + "/comments")
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
	p, _ := env.DB.CreateProject("no-versions")

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
	resp, _ := http.Get(env.Server.URL + "/api/projects")
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
	resp, _ := http.Get(env.Server.URL + "/")
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
	resp, _ := http.Get(env.Server.URL + "/projects/" + pid)
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

	resp, _ := http.Get(env.Server.URL + "/api/projects")
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
