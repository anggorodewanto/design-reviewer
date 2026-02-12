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

	h := &api.Handler{DB: database, Storage: store}
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
