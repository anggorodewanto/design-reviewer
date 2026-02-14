package cli

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// --- Config Tests ---

func setTestConfig(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), ".design-reviewer.yaml")
	ConfigPathOverride = path
	t.Cleanup(func() { ConfigPathOverride = "" })
	return path
}

func TestLoadConfigFileNotExist(t *testing.T) {
	setTestConfig(t)
	cfg, err := LoadConfig()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Server != "" || cfg.Token != "" {
		t.Errorf("expected empty config, got %+v", cfg)
	}
}

func TestSaveAndLoadConfig(t *testing.T) {
	setTestConfig(t)
	cfg := &Config{Server: "http://example.com", Token: "abc123"}
	if err := SaveConfig(cfg); err != nil {
		t.Fatal(err)
	}
	loaded, err := LoadConfig()
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Server != cfg.Server || loaded.Token != cfg.Token {
		t.Errorf("got %+v, want %+v", loaded, cfg)
	}
}

func TestSaveConfigFilePermissions(t *testing.T) {
	path := setTestConfig(t)
	SaveConfig(&Config{Token: "secret"})
	info, _ := os.Stat(path)
	if info.Mode().Perm() != 0600 {
		t.Errorf("permissions = %o, want 0600", info.Mode().Perm())
	}
}

func TestLoadConfigInvalidYAML(t *testing.T) {
	path := setTestConfig(t)
	os.WriteFile(path, []byte(":::invalid"), 0600)
	_, err := LoadConfig()
	if err == nil {
		t.Error("expected error for invalid YAML")
	}
}

func TestSaveConfigOverwrites(t *testing.T) {
	setTestConfig(t)
	SaveConfig(&Config{Server: "http://old.com", Token: "old"})
	SaveConfig(&Config{Server: "http://new.com", Token: "new"})
	cfg, _ := LoadConfig()
	if cfg.Server != "http://new.com" || cfg.Token != "new" {
		t.Errorf("got %+v", cfg)
	}
}

// --- ZipDirectory Tests ---

func TestZipDirectoryBasic(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "index.html"), []byte("<h1>hi</h1>"), 0644)
	os.WriteFile(filepath.Join(dir, "style.css"), []byte("body{}"), 0644)

	buf, err := ZipDirectory(dir)
	if err != nil {
		t.Fatal(err)
	}

	files := readZip(t, buf)
	if files["index.html"] != "<h1>hi</h1>" {
		t.Errorf("index.html = %q", files["index.html"])
	}
	if files["style.css"] != "body{}" {
		t.Errorf("style.css = %q", files["style.css"])
	}
}

func TestZipDirectorySkipsHiddenFiles(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "index.html"), []byte("ok"), 0644)
	os.WriteFile(filepath.Join(dir, ".hidden"), []byte("secret"), 0644)

	buf, err := ZipDirectory(dir)
	if err != nil {
		t.Fatal(err)
	}

	files := readZip(t, buf)
	if _, ok := files[".hidden"]; ok {
		t.Error("hidden file should be skipped")
	}
	if _, ok := files["index.html"]; !ok {
		t.Error("index.html should be included")
	}
}

func TestZipDirectorySkipsHiddenDirs(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "index.html"), []byte("ok"), 0644)
	hiddenDir := filepath.Join(dir, ".git")
	os.MkdirAll(hiddenDir, 0755)
	os.WriteFile(filepath.Join(hiddenDir, "config"), []byte("git"), 0644)

	buf, err := ZipDirectory(dir)
	if err != nil {
		t.Fatal(err)
	}

	files := readZip(t, buf)
	for name := range files {
		if strings.HasPrefix(name, ".git") {
			t.Errorf("hidden dir file included: %s", name)
		}
	}
}

func TestZipDirectoryPreservesSubdirs(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "images"), 0755)
	os.WriteFile(filepath.Join(dir, "index.html"), []byte("ok"), 0644)
	os.WriteFile(filepath.Join(dir, "images", "logo.png"), []byte("png"), 0644)

	buf, err := ZipDirectory(dir)
	if err != nil {
		t.Fatal(err)
	}

	files := readZip(t, buf)
	if files["images/logo.png"] != "png" {
		t.Errorf("expected images/logo.png in zip, got files: %v", keys(files))
	}
}

func TestZipDirectoryNonexistent(t *testing.T) {
	_, err := ZipDirectory("/nonexistent/path")
	if err == nil {
		t.Error("expected error for nonexistent directory")
	}
}

// --- Push Tests ---

func TestPushNotLoggedIn(t *testing.T) {
	setTestConfig(t)
	err := Push(t.TempDir(), "test", "")
	if err == nil || !strings.Contains(err.Error(), "Not logged in") {
		t.Errorf("expected 'Not logged in' error, got: %v", err)
	}
}

func TestPushDirNotExist(t *testing.T) {
	setTestConfig(t)
	SaveConfig(&Config{Token: "tok", Server: "http://localhost"})
	err := Push("/nonexistent", "test", "")
	if err == nil || !strings.Contains(err.Error(), "does not exist") {
		t.Errorf("expected 'does not exist' error, got: %v", err)
	}
}

func TestPushNoHTMLFiles(t *testing.T) {
	setTestConfig(t)
	SaveConfig(&Config{Token: "tok", Server: "http://localhost"})
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "readme.txt"), []byte("no html"), 0644)
	err := Push(dir, "test", "")
	if err == nil || !strings.Contains(err.Error(), ".html file") {
		t.Errorf("expected '.html file' error, got: %v", err)
	}
}

func TestPushDefaultName(t *testing.T) {
	setTestConfig(t)
	var receivedName string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.ParseMultipartForm(10 << 20)
		receivedName = r.FormValue("name")
		json.NewEncoder(w).Encode(map[string]any{
			"project_id": "p1", "version_id": "v1", "version_num": 1,
		})
	}))
	defer srv.Close()

	SaveConfig(&Config{Token: "tok", Server: srv.URL})
	dir := filepath.Join(t.TempDir(), "my-project")
	os.MkdirAll(dir, 0755)
	os.WriteFile(filepath.Join(dir, "index.html"), []byte("ok"), 0644)

	Push(dir, "", "")
	if receivedName != "my-project" {
		t.Errorf("name = %q, want 'my-project'", receivedName)
	}
}

func TestPushSuccess(t *testing.T) {
	setTestConfig(t)
	var gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		// Verify it's a multipart upload with file and name
		r.ParseMultipartForm(10 << 20)
		file, _, _ := r.FormFile("file")
		if file == nil {
			t.Error("missing file in upload")
		}
		name := r.FormValue("name")
		if name != "test-proj" {
			t.Errorf("name = %q, want 'test-proj'", name)
		}
		json.NewEncoder(w).Encode(map[string]any{
			"project_id": "p123", "version_id": "v1", "version_num": float64(1),
		})
	}))
	defer srv.Close()

	SaveConfig(&Config{Token: "mytoken", Server: srv.URL})
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "index.html"), []byte("<h1>test</h1>"), 0644)

	err := Push(dir, "test-proj", "")
	if err != nil {
		t.Fatal(err)
	}
	if gotAuth != "Bearer mytoken" {
		t.Errorf("auth = %q, want 'Bearer mytoken'", gotAuth)
	}
}

func TestPushServerError(t *testing.T) {
	setTestConfig(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprint(w, "server broke")
	}))
	defer srv.Close()

	SaveConfig(&Config{Token: "tok", Server: srv.URL})
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "index.html"), []byte("ok"), 0644)

	err := Push(dir, "test", "")
	if err == nil {
		t.Error("expected error for server error")
	}
}

func TestPushServerOverride(t *testing.T) {
	setTestConfig(t)
	var called bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		json.NewEncoder(w).Encode(map[string]any{
			"project_id": "p1", "version_id": "v1", "version_num": 1,
		})
	}))
	defer srv.Close()

	SaveConfig(&Config{Token: "tok", Server: "http://wrong-server"})
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "index.html"), []byte("ok"), 0644)

	Push(dir, "test", srv.URL)
	if !called {
		t.Error("server override not used")
	}
}

func TestPushUploadContainsValidZip(t *testing.T) {
	setTestConfig(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.ParseMultipartForm(10 << 20)
		file, _, _ := r.FormFile("file")
		data, _ := io.ReadAll(file)
		// Verify it's a valid zip
		zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
		if err != nil {
			t.Errorf("invalid zip: %v", err)
		}
		found := false
		for _, f := range zr.File {
			if f.Name == "index.html" {
				found = true
			}
		}
		if !found {
			t.Error("zip missing index.html")
		}
		json.NewEncoder(w).Encode(map[string]any{
			"project_id": "p1", "version_id": "v1", "version_num": 1,
		})
	}))
	defer srv.Close()

	SaveConfig(&Config{Token: "tok", Server: srv.URL})
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "index.html"), []byte("<h1>test</h1>"), 0644)

	err := Push(dir, "test", "")
	if err != nil {
		t.Fatal(err)
	}
}

// --- Logout Tests ---

func TestLogout(t *testing.T) {
	setTestConfig(t)
	SaveConfig(&Config{Server: "http://example.com", Token: "abc"})
	if err := Logout(); err != nil {
		t.Fatal(err)
	}
	cfg, _ := LoadConfig()
	if cfg.Token != "" {
		t.Errorf("token should be empty after logout, got %q", cfg.Token)
	}
	if cfg.Server != "http://example.com" {
		t.Errorf("server should be preserved, got %q", cfg.Server)
	}
}

func TestLogoutNoConfig(t *testing.T) {
	setTestConfig(t)
	if err := Logout(); err != nil {
		t.Fatal(err)
	}
}

// --- Login Tests ---

// fakeOAuthServer returns an httptest.Server that simulates the server-side
// cli-login endpoint: it reads the port from the query param and sends a
// callback to localhost:{port}/callback with the given token and name.
func fakeOAuthServer(t *testing.T, token, name string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		port := r.URL.Query().Get("port")
		if port == "" {
			http.Error(w, "missing port", 400)
			return
		}
		go func() {
			url := fmt.Sprintf("http://localhost:%s/callback?token=%s&name=%s", port, token, name)
			for i := 0; i < 50; i++ {
				resp, err := http.Get(url)
				if err == nil {
					resp.Body.Close()
					return
				}
			}
		}()
		fmt.Fprint(w, "ok")
	}))
}

func TestLoginCallbackReceivesToken(t *testing.T) {
	setTestConfig(t)
	srv := fakeOAuthServer(t, "test-token", "Test+User")
	defer srv.Close()

	err := Login(srv.URL)
	if err != nil {
		t.Fatal(err)
	}

	cfg, _ := LoadConfig()
	if cfg.Token != "test-token" {
		t.Errorf("token = %q, want 'test-token'", cfg.Token)
	}
	if cfg.Server != srv.URL {
		t.Errorf("server = %q, want %q", cfg.Server, srv.URL)
	}
}

func TestSaveConfigEmptyToken(t *testing.T) {
	setTestConfig(t)
	SaveConfig(&Config{Server: "http://example.com"})
	cfg, _ := LoadConfig()
	if cfg.Token != "" {
		t.Errorf("token should be empty, got %q", cfg.Token)
	}
	if cfg.Server != "http://example.com" {
		t.Errorf("server = %q", cfg.Server)
	}
}

func TestLogoutPreservesServer(t *testing.T) {
	setTestConfig(t)
	SaveConfig(&Config{Server: "http://myserver.com", Token: "tok"})
	Logout()
	cfg, _ := LoadConfig()
	if cfg.Server != "http://myserver.com" {
		t.Errorf("server = %q, want 'http://myserver.com'", cfg.Server)
	}
}

func TestPushUsesConfigServer(t *testing.T) {
	setTestConfig(t)
	var called bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		json.NewEncoder(w).Encode(map[string]any{
			"project_id": "p1", "version_id": "v1", "version_num": 1,
		})
	}))
	defer srv.Close()

	SaveConfig(&Config{Token: "tok", Server: srv.URL})
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "index.html"), []byte("ok"), 0644)

	Push(dir, "test", "")
	if !called {
		t.Error("config server not used")
	}
}

func TestZipDirectoryEmptyDir(t *testing.T) {
	dir := t.TempDir()
	buf, err := ZipDirectory(dir)
	if err != nil {
		t.Fatal(err)
	}
	files := readZip(t, buf)
	if len(files) != 0 {
		t.Errorf("expected empty zip, got %d files", len(files))
	}
}

func TestLoginServerURLFromConfig(t *testing.T) {
	setTestConfig(t)
	srv := fakeOAuthServer(t, "tok2", "User2")
	defer srv.Close()
	SaveConfig(&Config{Server: srv.URL})

	err := Login("")
	if err != nil {
		t.Fatal(err)
	}
	cfg, _ := LoadConfig()
	if cfg.Token != "tok2" {
		t.Errorf("token = %q", cfg.Token)
	}
}

func TestLoginCallbackMissingToken(t *testing.T) {
	setTestConfig(t)

	// Server that first sends a callback without a token, then with one
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		port := r.URL.Query().Get("port")
		go func() {
			base := fmt.Sprintf("http://localhost:%s/callback", port)
			for i := 0; i < 50; i++ {
				resp, err := http.Get(base)
				if err == nil {
					resp.Body.Close()
					resp2, err2 := http.Get(base + "?token=valid")
					if err2 == nil {
						resp2.Body.Close()
					}
					return
				}
			}
		}()
		fmt.Fprint(w, "ok")
	}))
	defer srv.Close()

	err := Login(srv.URL)
	if err != nil {
		t.Fatal(err)
	}
}

// --- Phase 32: Random Port ---

func TestLoginUsesRandomPort(t *testing.T) {
	setTestConfig(t)
	var receivedPort string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedPort = r.URL.Query().Get("port")
		go func() {
			url := fmt.Sprintf("http://localhost:%s/callback?token=t1&name=U1", receivedPort)
			for i := 0; i < 50; i++ {
				resp, err := http.Get(url)
				if err == nil {
					resp.Body.Close()
					return
				}
			}
		}()
		fmt.Fprint(w, "ok")
	}))
	defer srv.Close()

	err := Login(srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	if receivedPort == "" {
		t.Fatal("server never received port parameter")
	}
	if receivedPort == "9876" {
		t.Error("port should be random, not the old hardcoded 9876")
	}
}

// --- Config edge case tests ---

func TestConfigPathDefault(t *testing.T) {
	old := ConfigPathOverride
	ConfigPathOverride = ""
	defer func() { ConfigPathOverride = old }()
	p := configPath()
	if p == "" {
		t.Error("configPath should return non-empty default")
	}
	if !strings.Contains(p, ".design-reviewer.yaml") {
		t.Errorf("configPath = %q, expected to contain .design-reviewer.yaml", p)
	}
}

func TestSaveConfigWriteError(t *testing.T) {
	ConfigPathOverride = "/nonexistent/dir/.design-reviewer.yaml"
	defer func() { ConfigPathOverride = "" }()
	err := SaveConfig(&Config{Token: "x"})
	if err == nil {
		t.Error("expected error writing to nonexistent dir")
	}
}

func TestLoadConfigReadError(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".design-reviewer.yaml")
	os.MkdirAll(path, 0755) // create a directory where a file is expected
	ConfigPathOverride = path
	defer func() { ConfigPathOverride = "" }()
	_, err := LoadConfig()
	if err == nil {
		t.Error("expected error reading directory as file")
	}
}

func TestLogoutSaveError(t *testing.T) {
	// First save a valid config, then make path unwritable
	path := setTestConfig(t)
	SaveConfig(&Config{Server: "http://x.com", Token: "tok"})
	dir := filepath.Dir(path)
	os.Chmod(dir, 0444)
	t.Cleanup(func() { os.Chmod(dir, 0755) })
	err := Logout()
	if err == nil {
		t.Error("expected error from Logout when config is unwritable")
	}
}

// --- Push edge case tests ---

func TestPushDirIsFile(t *testing.T) {
	setTestConfig(t)
	SaveConfig(&Config{Token: "tok", Server: "http://localhost"})
	f := filepath.Join(t.TempDir(), "file.txt")
	os.WriteFile(f, []byte("x"), 0644)
	err := Push(f, "test", "")
	if err == nil || !strings.Contains(err.Error(), "does not exist") {
		t.Errorf("expected 'does not exist' error for file, got: %v", err)
	}
}

func TestPushServerBadJSON(t *testing.T) {
	setTestConfig(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(400)
		fmt.Fprint(w, "not json")
	}))
	defer srv.Close()
	SaveConfig(&Config{Token: "tok", Server: srv.URL})
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "index.html"), []byte("ok"), 0644)
	err := Push(dir, "test", "")
	if err == nil {
		t.Error("expected error for bad server response")
	}
}

func TestPushServerJSONError(t *testing.T) {
	setTestConfig(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(400)
		json.NewEncoder(w).Encode(map[string]string{"error": "bad upload"})
	}))
	defer srv.Close()
	SaveConfig(&Config{Token: "tok", Server: srv.URL})
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "index.html"), []byte("ok"), 0644)
	err := Push(dir, "test", "")
	if err == nil || !strings.Contains(err.Error(), "bad upload") {
		t.Errorf("expected 'bad upload' error, got: %v", err)
	}
}

func TestPushLoadConfigError(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".design-reviewer.yaml")
	os.MkdirAll(path, 0755) // directory instead of file
	ConfigPathOverride = path
	defer func() { ConfigPathOverride = "" }()
	err := Push(t.TempDir(), "test", "")
	if err == nil {
		t.Error("expected error from LoadConfig")
	}
}

func TestLogoutLoadConfigError(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".design-reviewer.yaml")
	os.MkdirAll(path, 0755)
	ConfigPathOverride = path
	defer func() { ConfigPathOverride = "" }()
	err := Logout()
	if err == nil {
		t.Error("expected error from LoadConfig in Logout")
	}
}

func TestZipDirectoryUnreadableFile(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "secret.html")
	os.WriteFile(f, []byte("x"), 0644)
	os.Chmod(f, 0000)
	t.Cleanup(func() { os.Chmod(f, 0644) })
	_, err := ZipDirectory(dir)
	if err == nil {
		t.Error("expected error for unreadable file")
	}
}

// --- Init Tests ---

func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w
	fn()
	w.Close()
	os.Stdout = old
	out, _ := io.ReadAll(r)
	return string(out)
}

func TestInitCreatesFile(t *testing.T) {
	dir := t.TempDir()
	out := captureStdout(t, func() {
		if err := Init(dir); err != nil {
			t.Fatal(err)
		}
	})
	data, err := os.ReadFile(filepath.Join(dir, "DESIGN_GUIDELINES.md"))
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != designGuidelinesContent {
		t.Error("file content does not match template")
	}
	if !strings.Contains(out, "Created DESIGN_GUIDELINES.md") {
		t.Errorf("unexpected output: %q", out)
	}
}

func TestInitSkipsExisting(t *testing.T) {
	dir := t.TempDir()
	existing := "original content"
	os.WriteFile(filepath.Join(dir, "DESIGN_GUIDELINES.md"), []byte(existing), 0644)

	out := captureStdout(t, func() {
		if err := Init(dir); err != nil {
			t.Fatal(err)
		}
	})
	data, _ := os.ReadFile(filepath.Join(dir, "DESIGN_GUIDELINES.md"))
	if string(data) != existing {
		t.Error("file was overwritten")
	}
	if !strings.Contains(out, "already exists, skipping") {
		t.Errorf("unexpected output: %q", out)
	}
}

func TestInitNonexistentDir(t *testing.T) {
	err := Init("/nonexistent/path/that/does/not/exist")
	if err == nil {
		t.Error("expected error for nonexistent directory")
	}
}

func TestInitContentHasRequiredSections(t *testing.T) {
	required := []string{
		"No JavaScript",
		"Self-Contained",
		"No External Resources",
		"File Structure",
		"1080px",
		"CSS Features That Work",
		"What Won't Work",
		"Tips for Best Results",
		"sandbox=\"allow-same-origin\"",
	}
	for _, s := range required {
		if !strings.Contains(designGuidelinesContent, s) {
			t.Errorf("template missing required section: %q", s)
		}
	}
}

// --- Helpers ---

func readZip(t *testing.T, buf *bytes.Buffer) map[string]string {
	t.Helper()
	r, err := zip.NewReader(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
	if err != nil {
		t.Fatal(err)
	}
	files := make(map[string]string)
	for _, f := range r.File {
		rc, _ := f.Open()
		data, _ := io.ReadAll(rc)
		rc.Close()
		files[f.Name] = string(data)
	}
	return files
}

func keys(m map[string]string) []string {
	var ks []string
	for k := range m {
		ks = append(ks, k)
	}
	return ks
}
