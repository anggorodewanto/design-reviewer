# Testing Guide

## Integration Tests

The integration test suite lives at the project root in `integration_test.go`. It uses `httptest.Server` to spin up the full stack (DB + storage + HTTP handlers) in-process — no port binding, no cleanup needed.

### Run all tests

```bash
go test -v ./...
```

### Run only integration tests

```bash
go test -v -run "Test" .
```

### Run a specific test

```bash
go test -v -run TestUploadCreatesProjectAndVersion .
```

## Structure

```
integration_test.go     # all integration tests + helpers
├── setup()             # creates temp DB, storage, httptest.Server
├── makeZip()           # builds in-memory zip from filename→content map
├── uploadZip()         # POSTs a zip to /api/upload, returns parsed JSON
│
├── Phase 2 tests       # Upload, serving, validation
├── Phase 3 tests       # (add here)
└── ...
```

## Adding Tests for a New Phase

1. Open `integration_test.go`
2. Add new `TestXxx` functions below the last phase section
3. Mark the section with a comment: `// --- Phase N: Description ---`
4. Use the existing helpers (`setup`, `makeZip`, `uploadZip`) or add new ones at the top
5. Run `go test -v .` to verify

### Example: adding a Phase 3 test

```go
// --- Phase 3: Project List ---

func TestListProjectsReturnsUploadedProjects(t *testing.T) {
    env := setup(t)
    z := makeZip(t, map[string]string{"index.html": "x"})
    uploadZip(t, env.Server.URL, "proj-a", z)
    uploadZip(t, env.Server.URL, "proj-b", z)

    resp, _ := http.Get(env.Server.URL + "/api/projects")
    // assert status, parse JSON, check both projects present
}
```

## Helpers Reference

| Helper | Purpose |
|--------|---------|
| `setup(t)` | Returns `*testEnv` with `.Server`, `.DB`, `.Storage`, `.TmpDir`. Auto-cleans up. |
| `makeZip(t, files)` | Returns `[]byte` zip from `map[string]string` of filename→content. |
| `uploadZip(t, baseURL, name, zip)` | POSTs to `/api/upload`, returns `map[string]any` JSON response. Fails test on non-200. |
