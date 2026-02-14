# Phase 36: Flow Graph Visualization

## Goal
Add an interactive flow graph tab to the design viewer. Renders the flow data from the Phase 35 API as a directed graph using Cytoscape.js. Clicking a node navigates to that page in the viewer.

## Prerequisites
Phase 35 complete — `GET /api/versions/{id}/flow` returns the merged flow graph JSON.

## What to Build

### 1. Cytoscape Vendor Files (`web/static/vendor/`)

- Add `cytoscape.min.js` and `cytoscape-dagre.min.js`.
- Include in `viewer.html` via `<script>` tags (only loaded on the viewer page).

### 2. Graph Rendering (`web/static/flow.js`)

Fetch `/api/versions/{id}/flow` and render with Cytoscape.js:

- **Nodes**: rounded rectangles labeled with the page filename.
- **Edges**: directed arrows with optional labels.
- **YAML edges**: solid lines. **HTML-extracted edges**: dashed lines.
- **Missing-file nodes**: red dashed border.
- **Active node**: highlighted (blue border) when a page is currently being viewed.
- **Click node** → switch the viewer iframe to that page and activate its page tab.
- **Zoom/pan**: enabled (built into Cytoscape).

**Layout:**
- Dagre layout (top-to-bottom) when edges exist.
- Grid layout when all nodes are disconnected.

### 3. Viewer Integration (`web/templates/viewer.html`)

- Add a "Flow" tab to the existing page tab bar.
- Add a `<div id="flow-graph">` container, hidden by default.
- When "Flow" tab is active: hide iframe, show graph container, initialize Cytoscape.
- When a page tab is active: hide graph, show iframe.

### 4. Tab Switching (`web/static/viewer.js`)

- Extend existing tab switching logic to handle the Flow tab.
- Lazy-load: only fetch flow data and initialize Cytoscape on first Flow tab activation.

### 5. Styles (`web/static/style.css`)

- `#flow-graph`: fills the viewer content area, `display: none` by default.
- Node styles: light background, rounded corners, border color by state.
- Active node: blue border/shadow.
- Missing node: red dashed border.
- Edge labels: small font along the edge.

## Files to Change

| File | Change |
|------|--------|
| `web/static/vendor/cytoscape.min.js` | New — Cytoscape.js library |
| `web/static/vendor/cytoscape-dagre.min.js` | New — dagre layout plugin |
| `web/static/flow.js` | New — graph rendering and interaction |
| `web/static/style.css` | Graph container and node/edge styles |
| `web/templates/viewer.html` | Flow tab, graph container div, vendor script tags |
| `web/static/viewer.js` | Tab switching logic for flow tab |

## Edge Cases

- **No edges at all**: grid layout with all nodes evenly spaced — serves as a page inventory.
- **Large projects (50+ pages)**: Cytoscape zoom/pan handles this; dagre layout scales fine.
- **Circular flows**: dagre handles cycles — nodes may appear at the same level.
- **Flow tab on project with no pages**: empty graph container with a "No pages" message.
- **Window resize**: Cytoscape refit on container resize.

## Verification

- Open a project with flow data → "Flow" tab visible in tab bar.
- Click "Flow" tab → graph renders with correct nodes and edges.
- YAML edges are solid, HTML-extracted edges are dashed.
- Missing-file nodes have red dashed border.
- Click a node → viewer switches to that page, page tab activates.
- Switch to a page tab → graph hides, iframe shows.
- Switch back to Flow tab → graph still rendered (no re-fetch).
- Project with no `flow.yaml` → all nodes disconnected in grid layout.
- Zoom and pan work in the graph.
