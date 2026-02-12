# Phase 9: CLI Tool

## Goal
Build the CLI binary with `login`, `logout`, and `push` commands.

## Prerequisites
Phases 1-8 complete — server with auth, upload API, and CLI OAuth endpoints all work.

## What to Build

### 1. CLI Entry Point (`cmd/cli/main.go`)

A single Go binary called `design-reviewer` with subcommands:
```
design-reviewer login [--server URL]
design-reviewer logout
design-reviewer push <directory> [--name <project-name>] [--server URL]
```

Use Go's `flag` package or simple `os.Args` parsing (no external CLI framework needed).

### 2. Configuration (`internal/cli/config.go`)

Config file: `~/.design-reviewer.yaml`

Format:
```yaml
server: http://localhost:8080
token: abc123...
```

Functions:
- `LoadConfig() (*Config, error)` — reads from `~/.design-reviewer.yaml`
- `SaveConfig(cfg *Config) error` — writes to `~/.design-reviewer.yaml`
- `--server` flag overrides the `server` value from config

Dependencies to add:
- `gopkg.in/yaml.v3`

### 3. Login Command (`internal/cli/login.go`)

`design-reviewer login`

Flow:
1. Read server URL from config or `--server` flag (default: `http://localhost:8080`)
2. Start a temporary HTTP server on `localhost:9876`
3. Open the user's browser to `{server}/auth/google/cli-login?port=9876`
4. Wait for callback at `localhost:9876/callback?token={token}`
5. Extract the token from the query parameter
6. Save token and server URL to `~/.design-reviewer.yaml`
7. Print "Logged in successfully as {name}" and shut down the temp server

Browser opening: use `exec.Command` with `xdg-open` (Linux), `open` (macOS), or `cmd /c start` (Windows).

Timeout: if no callback received within 2 minutes, print error and exit.

### 4. Logout Command (`internal/cli/logout.go`)

`design-reviewer logout`

- Remove the token from `~/.design-reviewer.yaml`
- Print "Logged out"

### 5. Push Command (`internal/cli/push.go`)

`design-reviewer push <directory> [--name <project-name>]`

Flow:
1. Validate the directory exists and contains at least one `.html` file
2. If `--name` not provided, use the directory name as the project name
3. Create a zip of the directory contents in memory (all files: HTML, CSS, JS, images, fonts)
4. Read token from config
5. POST to `{server}/api/upload` with:
   - Multipart form: `file` (zip) + `name` (project name)
   - Header: `Authorization: Bearer {token}`
6. Parse response JSON
7. Print: `Uploaded {name} v{version_num}\nReview URL: {server}/projects/{project_id}`

Error handling:
- No token → print "Not logged in. Run `design-reviewer login` first."
- Directory doesn't exist → print error
- No HTML files → print "Directory must contain at least one .html file"
- Server error → print the error message from response

### 6. Zip Creation Helper

A function to zip a directory:
```go
func ZipDirectory(dir string) (*bytes.Buffer, error)
```
- Walks the directory recursively
- Adds all files to the zip
- Preserves relative paths
- Skips hidden files (starting with `.`)

## Verification
- `go build -o design-reviewer ./cmd/cli` produces a binary
- `./design-reviewer login` opens browser, completes OAuth, saves token
- `./design-reviewer push ./test-designs --name my-project` uploads and returns review URL
- `./design-reviewer push ./test-designs --name my-project` again → creates version 2
- `./design-reviewer logout` removes token
- `./design-reviewer push` without login → shows "not logged in" error
