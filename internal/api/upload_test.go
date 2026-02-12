package api

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
)

func TestHandleUploadSuccess(t *testing.T) {
	h := setupTestHandler(t)

	var zipBuf bytes.Buffer
	zw := zip.NewWriter(&zipBuf)
	f, _ := zw.Create("index.html")
	f.Write([]byte("<h1>hi</h1>"))
	zw.Close()

	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	mw.WriteField("name", "test-proj")
	fw, _ := mw.CreateFormFile("file", "upload.zip")
	fw.Write(zipBuf.Bytes())
	mw.Close()

	req := httptest.NewRequest("POST", "/api/upload", &body)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	w := httptest.NewRecorder()
	h.handleUpload(w, req)

	if w.Code != 200 {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var res map[string]any
	json.NewDecoder(w.Body).Decode(&res)
	if res["project_id"] == nil || res["version_id"] == nil {
		t.Error("missing project_id or version_id")
	}
	if res["version_num"].(float64) != 1 {
		t.Errorf("version_num = %v, want 1", res["version_num"])
	}
}

func TestHandleUploadMissingFile(t *testing.T) {
	h := setupTestHandler(t)
	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	mw.WriteField("name", "test")
	mw.Close()

	req := httptest.NewRequest("POST", "/api/upload", &body)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	w := httptest.NewRecorder()
	h.handleUpload(w, req)

	if w.Code != 400 {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestHandleUploadMissingName(t *testing.T) {
	h := setupTestHandler(t)
	var zipBuf bytes.Buffer
	zw := zip.NewWriter(&zipBuf)
	f, _ := zw.Create("index.html")
	f.Write([]byte("x"))
	zw.Close()

	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	fw, _ := mw.CreateFormFile("file", "upload.zip")
	fw.Write(zipBuf.Bytes())
	mw.Close()

	req := httptest.NewRequest("POST", "/api/upload", &body)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	w := httptest.NewRecorder()
	h.handleUpload(w, req)

	if w.Code != 400 {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestHandleUploadBadZip(t *testing.T) {
	h := setupTestHandler(t)
	var zipBuf bytes.Buffer
	zw := zip.NewWriter(&zipBuf)
	f, _ := zw.Create("readme.txt")
	f.Write([]byte("no html"))
	zw.Close()

	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	mw.WriteField("name", "bad-proj")
	fw, _ := mw.CreateFormFile("file", "upload.zip")
	fw.Write(zipBuf.Bytes())
	mw.Close()

	req := httptest.NewRequest("POST", "/api/upload", &body)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	w := httptest.NewRecorder()
	h.handleUpload(w, req)

	if w.Code != 400 {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestHandleUploadExistingProject(t *testing.T) {
	h := setupTestHandler(t)

	makeUpload := func() *httptest.ResponseRecorder {
		var zipBuf bytes.Buffer
		zw := zip.NewWriter(&zipBuf)
		f, _ := zw.Create("index.html")
		f.Write([]byte("x"))
		zw.Close()
		var body bytes.Buffer
		mw := multipart.NewWriter(&body)
		mw.WriteField("name", "same-proj")
		fw, _ := mw.CreateFormFile("file", "upload.zip")
		fw.Write(zipBuf.Bytes())
		mw.Close()
		req := httptest.NewRequest("POST", "/api/upload", &body)
		req.Header.Set("Content-Type", mw.FormDataContentType())
		w := httptest.NewRecorder()
		h.handleUpload(w, req)
		return w
	}

	w1 := makeUpload()
	w2 := makeUpload()
	if w1.Code != 200 || w2.Code != 200 {
		t.Fatalf("uploads failed: %d, %d", w1.Code, w2.Code)
	}
	var r1, r2 map[string]any
	json.NewDecoder(w1.Body).Decode(&r1)
	json.NewDecoder(w2.Body).Decode(&r2)
	if r1["project_id"] != r2["project_id"] {
		t.Error("same project name should reuse project_id")
	}
	if r2["version_num"].(float64) != 2 {
		t.Errorf("second upload version_num = %v, want 2", r2["version_num"])
	}
}

func TestHandleDesignFileSuccess(t *testing.T) {
	h := setupTestHandler(t)
	pid, vid := seedProject(t, h, map[string]string{"index.html": "<h1>hello</h1>"})
	_ = pid

	req := httptest.NewRequest("GET", "/designs/"+vid+"/index.html", nil)
	req.SetPathValue("version_id", vid)
	req.SetPathValue("filepath", "index.html")
	w := httptest.NewRecorder()
	h.handleDesignFile(w, req)

	if w.Code != 200 {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if !bytes.Contains(w.Body.Bytes(), []byte("<h1>hello</h1>")) {
		t.Error("missing content")
	}
}

func TestHandleDesignFileNotFound(t *testing.T) {
	h := setupTestHandler(t)
	_, vid := seedProject(t, h, map[string]string{"index.html": "x"})

	req := httptest.NewRequest("GET", "/designs/"+vid+"/nope.html", nil)
	req.SetPathValue("version_id", vid)
	req.SetPathValue("filepath", "nope.html")
	w := httptest.NewRecorder()
	h.handleDesignFile(w, req)

	if w.Code != 404 {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

func TestHandleDesignFilePathTraversal(t *testing.T) {
	h := setupTestHandler(t)

	req := httptest.NewRequest("GET", "/designs/v1/../../../etc/passwd", nil)
	req.SetPathValue("version_id", "v1")
	req.SetPathValue("filepath", "../../../etc/passwd")
	w := httptest.NewRecorder()
	h.handleDesignFile(w, req)

	if w.Code != 400 {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestHandleDesignFileDirectory(t *testing.T) {
	h := setupTestHandler(t)
	_, vid := seedProject(t, h, map[string]string{"index.html": "x"})

	// Create a subdirectory
	dir := h.Storage.GetFilePath(vid, "subdir")
	os.MkdirAll(dir, 0o755)

	req := httptest.NewRequest("GET", "/designs/"+vid+"/subdir", nil)
	req.SetPathValue("version_id", vid)
	req.SetPathValue("filepath", "subdir")
	w := httptest.NewRecorder()
	h.handleDesignFile(w, req)

	if w.Code != 404 {
		t.Errorf("expected 404 for directory, got %d", w.Code)
	}
}

func TestRegisterRoutes(t *testing.T) {
	h := setupTestHandler(t)
	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	routes := []struct {
		method string
		path   string
	}{
		{"GET", "/"},
		{"GET", "/api/projects"},
		{"POST", "/api/upload"},
		{"GET", "/projects/test-id"},
		{"GET", "/api/projects/test-id/versions"},
		{"GET", "/api/versions/test-id/comments"},
		{"PATCH", "/api/projects/test-id/status"},
	}
	for _, r := range routes {
		req := httptest.NewRequest(r.method, r.path, nil)
		_, pattern := mux.Handler(req)
		if pattern == "" {
			t.Errorf("no route matched %s %s", r.method, r.path)
		}
	}
}
