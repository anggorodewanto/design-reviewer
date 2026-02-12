# Phase 4: Design Viewer Page

## Goal
Build the design viewer page that renders uploaded HTML/CSS designs in a sandboxed iframe with multi-page navigation.

## Prerequisites
Phases 1-3 complete — DB, storage, upload API, project list page, static file serving all work.

## What to Build

### 1. Design Viewer Page Handler (`internal/api/viewer.go`)

`GET /projects/{id}`
- Looks up project by ID
- Gets the latest version (default) or a specific version if `?version={version_id}` query param is present
- Gets list of HTML files in the version (`storage.ListHTMLFiles`)
- Renders the viewer template with:
  - Project info (name, status)
  - Current version info
  - List of HTML pages
  - Default page: `index.html` if it exists, otherwise first HTML file

### 2. Viewer Template (`web/templates/viewer.html`)

Layout:
```
┌─────────────────────────────────────────────┐
│  Project Name          Status Badge         │
├──────────┬──────────────────────────────────┤
│ Sidebar  │  ┌────────────────────────────┐  │
│          │  │  Page tabs (if multi-page) │  │
│ (empty   │  ├────────────────────────────┤  │
│  for now │  │                            │  │
│  — used  │  │     Sandboxed iframe       │  │
│  later   │  │                            │  │
│  for     │  │                            │  │
│  versions│  │                            │  │
│  )       │  │                            │  │
│          │  └────────────────────────────┘  │
└──────────┴──────────────────────────────────┘
```

- Header bar with project name and status
- Left sidebar (empty placeholder — Phase 6 adds version list here)
- Main area:
  - Page tabs if multiple HTML files exist (tab per HTML file, clicking switches iframe src)
  - Sandboxed iframe loading the design

### 3. Iframe Setup

The iframe should:
- `src` points to `/designs/{version_id}/{page}` (e.g., `/designs/abc123/index.html`)
- `sandbox="allow-same-origin"` — allows CSS to work but blocks scripts
- Width: 100% of the main content area
- Height: fill available vertical space (use CSS `calc()` or flexbox)
- No border

### 4. Multi-Page Navigation (JavaScript)

In `web/static/viewer.js`:
- Page tabs are rendered from the template (list of HTML filenames)
- Clicking a tab updates the iframe `src` to the corresponding page
- Active tab is visually highlighted
- URL doesn't need to change (no need for routing, just swap iframe src)

### 5. Viewer Styles

Add to `web/static/style.css`:
- Viewer layout (sidebar + main area using CSS grid or flexbox)
- Sidebar: fixed width ~250px, light background
- Page tabs: horizontal tab bar above iframe
- Active tab styling
- Iframe: fills remaining space, no border

### 6. Register Route

Add to `Handler.RegisterRoutes()`:
- `GET /projects/{id}` → viewer page handler

## Verification
- Upload a project with multiple HTML files
- Navigate to `/projects/{id}` — see the design rendered in the iframe
- Page tabs appear and switching tabs loads different HTML files
- Iframe is sandboxed (scripts in uploaded designs don't execute)
- Sidebar placeholder is visible but empty
