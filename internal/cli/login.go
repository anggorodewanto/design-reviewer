package cli

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os/exec"
	"runtime"
	"strings"
	"time"
)

func Login(serverURL string) error {
	cfg, err := LoadConfig()
	if err != nil {
		return err
	}
	if serverURL == "" {
		serverURL = cfg.Server
	}
	if serverURL == "" {
		serverURL = "http://localhost:8080"
	}
	serverURL = strings.TrimRight(serverURL, "/")

	tokenCh := make(chan string, 1)
	errCh := make(chan error, 1)

	mux := http.NewServeMux()
	nameCh := make(chan string, 1)

	mux.HandleFunc("/callback", func(w http.ResponseWriter, r *http.Request) {
		token := r.URL.Query().Get("token")
		if token == "" {
			http.Error(w, "missing token", http.StatusBadRequest)
			return
		}
		fmt.Fprint(w, "Login successful! You can close this tab.")
		nameCh <- r.URL.Query().Get("name")
		tokenCh <- token
	})

	listener, err := net.Listen("tcp", "localhost:0")
	if err != nil {
		return fmt.Errorf("failed to start local server: %w", err)
	}
	port := listener.Addr().(*net.TCPAddr).Port

	srv := &http.Server{Handler: mux}
	go func() {
		if err := srv.Serve(listener); err != nil && err != http.ErrServerClosed {
			errCh <- err
		}
	}()

	url := fmt.Sprintf("%s/auth/google/cli-login?port=%d", serverURL, port)
	fmt.Printf("Open this URL in your browser:\n%s\n", url)
	openBrowser(url)

	select {
	case token := <-tokenCh:
		name := <-nameCh
		cfg.Server = serverURL
		cfg.Token = token
		if err := SaveConfig(cfg); err != nil {
			return fmt.Errorf("failed to save config: %w", err)
		}
		if name != "" {
			fmt.Printf("Logged in successfully as %s\n", name)
		} else {
			fmt.Println("Logged in successfully")
		}
	case err := <-errCh:
		return err
	case <-time.After(2 * time.Minute):
		return fmt.Errorf("login timed out (no callback received within 2 minutes)")
	}

	srv.Shutdown(context.Background())
	return nil
}

func openBrowser(url string) error {
	switch runtime.GOOS {
	case "linux":
		return exec.Command("xdg-open", url).Start()
	case "darwin":
		return exec.Command("open", url).Start()
	case "windows":
		return exec.Command("cmd", "/c", "start", url).Start()
	default:
		return fmt.Errorf("unsupported platform")
	}
}
