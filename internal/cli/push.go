package cli

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

func Push(dir, name, serverURL string) error {
	cfg, err := LoadConfig()
	if err != nil {
		return err
	}
	if cfg.Token == "" {
		return fmt.Errorf("Not logged in. Run `design-reviewer login` first.")
	}
	if serverURL == "" {
		serverURL = cfg.Server
	}
	if serverURL == "" {
		serverURL = "http://localhost:8080"
	}

	// Validate directory
	info, err := os.Stat(dir)
	if err != nil || !info.IsDir() {
		return fmt.Errorf("directory does not exist: %s", dir)
	}

	// Check for at least one .html file
	hasHTML := false
	filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err == nil && !info.IsDir() && strings.HasSuffix(strings.ToLower(info.Name()), ".html") {
			hasHTML = true
		}
		return nil
	})
	if !hasHTML {
		return fmt.Errorf("Directory must contain at least one .html file")
	}

	if name == "" {
		name = filepath.Base(dir)
	}

	// Create zip
	zipBuf, err := ZipDirectory(dir)
	if err != nil {
		return fmt.Errorf("failed to create zip: %w", err)
	}

	// Build multipart request
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("file", "upload.zip")
	if err != nil {
		return err
	}
	io.Copy(part, zipBuf)
	writer.WriteField("name", name)
	writer.Close()

	req, err := http.NewRequest("POST", serverURL+"/api/upload", &body)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("Authorization", "Bearer "+cfg.Token)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("upload failed: %w", err)
	}
	defer resp.Body.Close()

	var result map[string]any
	respBody, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK {
		if err := json.Unmarshal(respBody, &result); err == nil {
			if errMsg, ok := result["error"].(string); ok {
				return fmt.Errorf("%s", errMsg)
			}
		}
		msg := strings.TrimSpace(string(respBody))
		if msg == "" {
			msg = "upload failed"
		}
		return fmt.Errorf("%s", msg)
	}

	json.Unmarshal(respBody, &result)

	versionNum := result["version_num"]
	projectID := result["project_id"]
	fmt.Printf("Uploaded %s v%.0f\n", name, versionNum)
	fmt.Printf("Review URL: %s/projects/%s\n", serverURL, projectID)
	return nil
}

func ZipDirectory(dir string) (*bytes.Buffer, error) {
	buf := &bytes.Buffer{}
	w := zip.NewWriter(buf)

	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		// Skip hidden files/dirs
		if strings.HasPrefix(info.Name(), ".") && info.Name() != "." {
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if info.IsDir() {
			return nil
		}

		rel, err := filepath.Rel(dir, path)
		if err != nil {
			return err
		}

		f, err := w.Create(rel)
		if err != nil {
			return err
		}

		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		_, err = f.Write(data)
		return err
	})
	if err != nil {
		return nil, err
	}

	if err := w.Close(); err != nil {
		return nil, err
	}
	return buf, nil
}
