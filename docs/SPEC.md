# Design Reviewer — Product Spec

## Overview

Internal tool for reviewing UI/UX design mockups built as HTML+CSS. Designers create mockups locally using AI coding tools (Claude Code, Cursor, etc.), then push them to a collaborative web app where PMs, engineers, and other designers can view and annotate.

Two components:
1. **CLI** (`design-reviewer`) — pushes local HTML+CSS projects to the review server
2. **Web App** — serves uploaded designs with collaborative pin-comment annotations

---

## CLI

### Commands

```
design-reviewer push <directory> [--name <project-name>]
```
- Bundles the directory (HTML, CSS, JS, images, fonts — must be self-contained, no external CDN references)
- Uploads as a zip to the server API
- If `--name` matches an existing project, creates a new version
- If new name, creates a new project
- Returns the review URL

```
design-reviewer list
```
- Lists all projects with their current status and review URLs

```
design-reviewer login
```
- Opens browser to server's Google OAuth flow
- CLI starts a temporary local HTTP server on `localhost:9876`
- After Google auth, server redirects to `localhost:9876/callback` with an API token
- Token stored in `~/.design-reviewer.yaml`

```
design-reviewer logout
```
- Removes stored token

### Configuration

- Server URL and auth token stored in `~/.design-reviewer.yaml`
- `--server` flag overrides server URL
- All commands except `login` require a valid token

---

## Web App

### Auth
- Google OAuth SSO (company Google Workspace)
- User identified by Google email and display name
- No roles — all users have equal permissions
- Session stored as HTTP-only cookie

### Project List (Home Page)
- Shows all design projects
- Each project shows: name, current status, version count, last updated, link to review
- Status badges: **Draft** → **In Review** → **Approved** → **Handed Off**
- Status is manually changed by any user

### Design Viewer
- Renders uploaded HTML+CSS in a sandboxed iframe
- Multi-page navigation if the project contains multiple HTML files
- Viewport toggle: desktop (1440px) / tablet (768px) / mobile (375px)

### Annotation System
- Click anywhere on the rendered design to drop a pin
- Pin stored as `{x%, y%, page, comment, author, timestamp}`
  - Percentage-based coordinates relative to the iframe content for viewport independence
- Threaded replies on each pin
- Resolve / unresolve comments
- Filter: All / Open / Resolved
- Pins visually shown as numbered markers on the design

### Version History
- Each `push` creates a new version
- Version list shown in sidebar
- Switching versions shows that version's design
- **Unresolved comments carry over** to the new version
- Resolved comments stay on the version where they were resolved

---

## Tech Stack

| Component | Choice |
|-----------|--------|
| Backend | Go (stdlib `net/http` + minimal dependencies) |
| Frontend | Vanilla HTML/CSS/JS (no framework) |
| Database | SQLite |
| Hosting | Fly.io (persistent volume for SQLite + uploaded files) |
| CLI | Go (single binary, same repo) |

### Project Structure

```
design-reviewer/
├── SPEC.md
├── cmd/
│   ├── server/         # Web app backend
│   │   └── main.go
│   └── cli/            # CLI tool
│       └── main.go
├── internal/
│   ├── api/            # HTTP handlers
│   ├── db/             # SQLite queries
│   ├── storage/        # File storage (uploaded designs)
│   └── auth/           # Google OAuth
├── web/
│   ├── static/         # CSS, JS for the web app
│   └── templates/      # HTML templates
├── go.mod
└── Dockerfile
```

---

## Data Model

### projects
| Column | Type | Description |
|--------|------|-------------|
| id | TEXT (UUID) | Primary key |
| name | TEXT | Unique project name |
| status | TEXT | draft / in_review / approved / handed_off |
| created_at | DATETIME | |
| updated_at | DATETIME | |

### versions
| Column | Type | Description |
|--------|------|-------------|
| id | TEXT (UUID) | Primary key |
| project_id | TEXT | FK → projects |
| version_num | INTEGER | Auto-incrementing per project |
| storage_path | TEXT | Path to uploaded files on disk |
| created_at | DATETIME | |

### comments
| Column | Type | Description |
|--------|------|-------------|
| id | TEXT (UUID) | Primary key |
| version_id | TEXT | FK → versions (version where comment was created) |
| page | TEXT | HTML filename (e.g., index.html) |
| x_percent | REAL | X position as percentage |
| y_percent | REAL | Y position as percentage |
| author_name | TEXT | From Google profile |
| author_email | TEXT | From Google profile |
| body | TEXT | Comment text |
| resolved | BOOLEAN | Default false |
| created_at | DATETIME | |

### replies
| Column | Type | Description |
|--------|------|-------------|
| id | TEXT (UUID) | Primary key |
| comment_id | TEXT | FK → comments |
| author_name | TEXT | |
| author_email | TEXT | |
| body | TEXT | |
| created_at | DATETIME | |

---

## API Endpoints

### CLI-facing
- `POST /api/upload` — upload zip, create project/version
- `GET /api/projects` — list all projects

### Web App
- `GET /` — project list page
- `GET /projects/:id` — design viewer + annotations
- `PATCH /api/projects/:id/status` — update project status
- `GET /api/projects/:id/versions` — list versions
- `GET /api/versions/:id/comments` — get comments for a version (includes carried-over unresolved)
- `POST /api/versions/:id/comments` — create comment
- `POST /api/comments/:id/replies` — add reply
- `PATCH /api/comments/:id/resolve` — toggle resolve
- `GET /designs/:version_id/*filepath` — serve uploaded static files

### Auth
- `GET /auth/google/login` — redirect to Google OAuth
- `GET /auth/google/callback` — handle OAuth callback (web)
- `GET /auth/google/cli-login?port=9876` — initiate OAuth for CLI, redirects back to CLI's localhost after auth
- `POST /api/auth/token` — exchange OAuth code for API token (CLI flow)

---

## Comment Carry-Over Logic

When a new version is pushed:
1. All **unresolved** comments from the previous version become visible on the new version
2. Their coordinates are preserved (percentage-based, so they roughly map to the same area)
3. Once resolved on any version, they stop carrying forward
4. Resolved comments remain viewable on the version where they were resolved

---

## MVP Scope (Build This First)

1. CLI `push` command — zip and upload a directory
2. Server receives and stores uploads, serves static files in iframe
3. Pin comments with author name, threaded replies, resolve/unresolve
4. Project list with status workflow
5. Version history with comment carry-over
6. Google OAuth login (web + CLI)
7. CLI `login` / `logout` commands

### Deferred
- Side-by-side version comparison
- Viewport toggle (start with desktop only, add later)
- Notifications (email/Slack when new comments)
- CLI `list` command (nice-to-have, not critical for MVP)

---

## Design Prompt Template

A reusable prompt template shipped with the CLI (`design-reviewer init`) that users copy into their project directory. It instructs AI coding tools (Claude Code, Cursor, etc.) to generate designs compatible with the review app's rendering environment.

### Command

```
design-reviewer init [directory]
```
- Creates a `DESIGN_GUIDELINES.md` file in the target directory (default: current directory)
- If file already exists, skip with a message

### Template Content

The `DESIGN_GUIDELINES.md` must communicate these rendering constraints:

1. **No JavaScript execution** — the viewer renders HTML in a sandboxed iframe (`sandbox="allow-same-origin"`). Scripts will not run. All visual output must be pure HTML+CSS.
2. **Self-contained assets** — no external CDN references. No Google Fonts via URL, no Tailwind CDN, no external images. All fonts, images, and stylesheets must be local files in the project directory.
3. **Local file structure** — CSS in `.css` files or `<style>` tags. Images and fonts as relative paths (e.g., `./images/logo.png`, `./fonts/Inter.woff2`).
4. **Multi-page = multiple HTML files** — each screen/page should be a separate `.html` file. The viewer shows tabs for each HTML file. Do not rely on `<a>` links for navigation between pages.
5. **Desktop-first layout** — design for 1440px viewport width. The viewer displays at desktop width by default.
6. **No interactive states that require JS** — hover effects via CSS are fine. Anything needing click handlers, toggles, modals, or dynamic content will not work.
7. **Standard HTML5 + CSS3** — use well-supported features. CSS Grid, Flexbox, custom properties, transitions, and animations all work.
8. **Annotation-friendly** — reviewers will click on the design to drop pin comments. Avoid elements that might interfere with click detection (though the annotation overlay handles this).
