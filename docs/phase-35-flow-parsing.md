# Phase 35: Flow Definition & Parsing

## Goal
Add backend support for defining user flows between HTML pages. Designers declare flows via a `flow.yaml` file and/or `data-dr-link` HTML attributes. The server parses both sources and exposes a merged flow graph via API.

## Prerequisites
Phases 1-6 complete — viewer with multi-page tabs and version history works.

## What to Build

### 1. YAML Flow Definition (`flow.yaml`)

Designers place a `flow.yaml` in their mockup directory alongside HTML files. It is uploaded as part of the zip — no CLI changes needed.

**Schema:**
```yaml
# flow.yaml
title: "Onboarding Flow"
flows:
  index.html:
    - target: login.html
      label: "Sign In"
    - target: signup.html
      label: "Register"
  login.html:
    - target: dashboard.html
      label: "Success"
    - target: forgot-password.html
      label: "Forgot Password"
```

- Keys under `flows` are source page filenames (relative to project root).
- Each entry has a `target` (destination page filename) and an optional `label`.
- Pages that exist in the project but are not mentioned appear as disconnected nodes.
- References to nonexistent files are flagged as `missing` in the graph.

### 2. HTML Jump Points (`data-dr-link`)

Designers can annotate elements in their HTML to declare flow connections inline:

```html
<a href="#" data-dr-link="login.html">Sign In</a>
<button data-dr-link="dashboard.html">Continue</button>
```

- Any element with a `data-dr-link` attribute declares an edge from the current page to the target page.
- The element's text content is used as the edge label.
- These edges are merged with YAML-defined edges (duplicates deduplicated by source+target pair, YAML takes precedence).

### 3. Flow Parser (`internal/flow/flow.go`)

**`ParseFlowYAML(r io.Reader) (*FlowDef, error)`**
- Parses and validates `flow.yaml` content.

**`ExtractHTMLLinks(filename string, r io.Reader) ([]Edge, error)`**
- Tokenizes HTML using `golang.org/x/net/html`.
- Finds elements with `data-dr-link` attribute.
- Returns edges with the element's text content as label.

**`BuildGraph(pages []string, yamlDef *FlowDef, htmlEdges map[string][]Edge) *Graph`**
- Merges YAML edges and HTML-extracted edges.
- Creates a node for every HTML page in the project.
- Flags edges pointing to nonexistent pages.

```go
type FlowDef struct {
    Title string            `yaml:"title"`
    Flows map[string][]Edge `yaml:"flows"`
}

type Edge struct {
    Target string `yaml:"target"`
    Label  string `yaml:"label"`
}

type Graph struct {
    Title string      `json:"title"`
    Nodes []Node      `json:"nodes"`
    Edges []GraphEdge `json:"edges"`
}

type Node struct {
    ID      string `json:"id"`      // filename
    Label   string `json:"label"`   // from YAML or filename
    Missing bool   `json:"missing"` // true if file doesn't exist
}

type GraphEdge struct {
    Source string `json:"source"`
    Target string `json:"target"`
    Label  string `json:"label"`
    Origin string `json:"origin"` // "yaml" or "html"
}
```

### 4. Flow API Endpoint (`internal/api/flow.go`)

**`GET /api/versions/{id}/flow`**
- Reads `flow.yaml` from the version's storage directory (missing file is not an error).
- Scans all HTML files for `data-dr-link` attributes.
- Merges both sources via `BuildGraph`.
- Returns JSON:

```json
{
  "title": "Onboarding Flow",
  "nodes": [
    {"id": "index.html", "label": "index.html", "missing": false},
    {"id": "login.html", "label": "login.html", "missing": false},
    {"id": "orphan.html", "label": "orphan.html", "missing": true}
  ],
  "edges": [
    {"source": "index.html", "target": "login.html", "label": "Sign In", "origin": "yaml"},
    {"source": "login.html", "target": "dashboard.html", "label": "Continue", "origin": "html"}
  ]
}
```

### 5. Register Route

Add to `Handler.RegisterRoutes()`:
- `GET /api/versions/{id}/flow` — behind `versionAccess` middleware.

## Files to Change

| File | Change |
|------|--------|
| `internal/flow/flow.go` | New — YAML parser, HTML link extractor, graph builder |
| `internal/api/flow.go` | New — flow API handler |
| `internal/api/api.go` | Register flow route |
| `go.mod` | Add `golang.org/x/net` and `gopkg.in/yaml.v3` dependencies |

## Edge Cases

- **No `flow.yaml` and no `data-dr-link` tags**: returns all pages as disconnected nodes with no edges.
- **Broken references**: YAML or `data-dr-link` pointing to nonexistent files → node with `missing: true`.
- **Circular flows**: valid — graph is a general directed graph, not a DAG.
- **Subdirectory pages**: paths like `pages/about.html` work as-is — node IDs are relative paths.
- **Duplicate edges**: same source+target from both YAML and HTML → keep YAML version.
- **Malformed `flow.yaml`**: return 400 with validation error.

## Verification

- Upload a project with `flow.yaml` → `GET /api/versions/{id}/flow` returns correct nodes and edges.
- Upload a project with `data-dr-link` in HTML → edges with `"origin": "html"` appear in response.
- Upload a project with both → edges merged, duplicates deduplicated.
- Upload a project with neither → all pages returned as disconnected nodes, empty edges array.
- Reference a nonexistent page → node appears with `"missing": true`.
- Malformed YAML → 400 response.
