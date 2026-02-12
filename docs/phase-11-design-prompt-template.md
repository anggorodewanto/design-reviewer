# Phase 11: Design Prompt Template (`init` Command)

## Goal
Add a `design-reviewer init` CLI command that generates a `DESIGN_GUIDELINES.md` file — a prompt template users drop into their project so AI coding tools generate designs compatible with the review app.

## Prerequisites
Phase 9 complete — CLI tool exists with `login`, `logout`, `push` commands.

## What to Build

### 1. `init` Command (`internal/cli/init.go`)

```
design-reviewer init [directory]
```

- If `directory` is omitted, use current working directory
- Creates `DESIGN_GUIDELINES.md` in the target directory
- If file already exists, print "DESIGN_GUIDELINES.md already exists, skipping." and exit 0
- On success, print "Created DESIGN_GUIDELINES.md — include this file in your project so AI tools follow the rendering constraints."

### 2. Template Content

Embed the following as a Go string constant (or `embed.FS`). The file should be written as-is:

```markdown
# Design Guidelines for Design Reviewer

Use these rules when generating UI/UX design mockups as HTML+CSS. These designs will be uploaded to a review tool that renders them in a sandboxed iframe.

## Rendering Constraints

### No JavaScript
The viewer uses a sandboxed iframe (`sandbox="allow-same-origin"`). **Scripts will not execute.** All visual output must be achieved with pure HTML and CSS only.

- No `<script>` tags
- No inline `onclick` or event handlers
- No JS-dependent libraries (e.g., Alpine.js, HTMX)

### Self-Contained — No External Resources
Everything must be local. The reviewer serves files from the uploaded directory only.

**Do not use:**
- Google Fonts via `<link>` URL
- Tailwind/Bootstrap CDN
- External image URLs
- Any `https://` references in `<link>`, `<script>`, or `<img>` tags

**Instead:**
- Download fonts and reference them locally: `./fonts/Inter.woff2`
- Include CSS files in the directory: `./styles/main.css`
- Place images locally: `./images/hero.png`

### File Structure
```
my-design/
├── index.html          # Main page (required)
├── about.html          # Additional pages (optional)
├── styles/
│   └── main.css
├── images/
│   └── logo.png
└── fonts/
    └── Inter.woff2
```

- Each screen or page should be a **separate HTML file**
- The reviewer shows a tab for each `.html` file
- Do not use `<a>` links for page navigation — the reviewer handles it via tabs
- `index.html` is loaded by default

### Design for 1440px Width
The viewer displays designs at desktop width (1440px) by default. Design accordingly.

### CSS Features That Work
- CSS Grid and Flexbox
- Custom properties (`--var`)
- Transitions and animations (`@keyframes`)
- Media queries (though desktop is the default view)
- `:hover`, `:focus`, `:nth-child` and other CSS pseudo-classes
- `calc()`, `clamp()`, `min()`, `max()`

### What Won't Work
- Anything requiring JavaScript (click handlers, toggles, modals, dynamic content)
- External resource loading
- `<iframe>` within the design
- `<form>` submissions

## Tips for Best Results
- Use semantic HTML (`<header>`, `<nav>`, `<main>`, `<section>`, etc.)
- Keep the design visually clear — reviewers will drop pin annotations on it
- Use placeholder content that looks realistic (names, text, images)
- If showing multiple states (e.g., empty state, filled state), make them separate HTML files
```

### 3. Register the Command

Add `init` to the CLI's subcommand routing in `cmd/cli/main.go`.

## Verification
- `./design-reviewer init` creates `DESIGN_GUIDELINES.md` in current directory
- `./design-reviewer init ./my-project` creates it in `./my-project/`
- Running again when file exists prints skip message, doesn't overwrite
- The generated file is valid markdown and accurately describes the rendering constraints
