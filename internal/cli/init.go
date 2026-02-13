package cli

import (
	"fmt"
	"os"
	"path/filepath"
)

const designGuidelinesFile = "DESIGN_GUIDELINES.md"

const designGuidelinesContent = `# Design Guidelines for Design Reviewer

Use these rules when generating UI/UX design mockups as HTML+CSS. These designs will be uploaded to a review tool that renders them in a sandboxed iframe.

## Rendering Constraints

### No JavaScript
The viewer uses a sandboxed iframe (` + "`" + `sandbox="allow-same-origin"` + "`" + `). **Scripts will not execute.** All visual output must be achieved with pure HTML and CSS only.

- No ` + "`" + `<script>` + "`" + ` tags
- No inline ` + "`" + `onclick` + "`" + ` or event handlers
- No JS-dependent libraries (e.g., Alpine.js, HTMX)

### Self-Contained — No External Resources
Everything must be local. The reviewer serves files from the uploaded directory only.

**Do not use:**
- Google Fonts via ` + "`" + `<link>` + "`" + ` URL
- Tailwind/Bootstrap CDN
- External image URLs
- Any ` + "`" + `https://` + "`" + ` references in ` + "`" + `<link>` + "`" + `, ` + "`" + `<script>` + "`" + `, or ` + "`" + `<img>` + "`" + ` tags

**Instead:**
- Download fonts and reference them locally: ` + "`" + `./fonts/Inter.woff2` + "`" + `
- Include CSS files in the directory: ` + "`" + `./styles/main.css` + "`" + `
- Place images locally: ` + "`" + `./images/hero.png` + "`" + `

### File Structure
` + "```" + `
my-design/
├── index.html          # Main page (required)
├── about.html          # Additional pages (optional)
├── styles/
│   └── main.css
├── images/
│   └── logo.png
└── fonts/
    └── Inter.woff2
` + "```" + `

- Each screen or page should be a **separate HTML file**
- The reviewer shows a tab for each ` + "`" + `.html` + "`" + ` file
- Do not use ` + "`" + `<a>` + "`" + ` links for page navigation — the reviewer handles it via tabs
- ` + "`" + `index.html` + "`" + ` is loaded by default

### Design for 1440px Width
The viewer displays designs at desktop width (1440px) by default. Design accordingly.

### CSS Features That Work
- CSS Grid and Flexbox
- Custom properties (` + "`" + `--var` + "`" + `)
- Transitions and animations (` + "`" + `@keyframes` + "`" + `)
- Media queries (though desktop is the default view)
- ` + "`" + `:hover` + "`" + `, ` + "`" + `:focus` + "`" + `, ` + "`" + `:nth-child` + "`" + ` and other CSS pseudo-classes
- ` + "`" + `calc()` + "`" + `, ` + "`" + `clamp()` + "`" + `, ` + "`" + `min()` + "`" + `, ` + "`" + `max()` + "`" + `

### What Won't Work
- Anything requiring JavaScript (click handlers, toggles, modals, dynamic content)
- External resource loading
- ` + "`" + `<iframe>` + "`" + ` within the design
- ` + "`" + `<form>` + "`" + ` submissions

## Tips for Best Results
- Use semantic HTML (` + "`" + `<header>` + "`" + `, ` + "`" + `<nav>` + "`" + `, ` + "`" + `<main>` + "`" + `, ` + "`" + `<section>` + "`" + `, etc.)
- Keep the design visually clear — reviewers will drop pin annotations on it
- Use placeholder content that looks realistic (names, text, images)
- If showing multiple states (e.g., empty state, filled state), make them separate HTML files
`

// Init creates a DESIGN_GUIDELINES.md file in the given directory.
func Init(dir string) error {
	path := filepath.Join(dir, designGuidelinesFile)
	if _, err := os.Stat(path); err == nil {
		fmt.Println("DESIGN_GUIDELINES.md already exists, skipping.")
		return nil
	}
	if err := os.WriteFile(path, []byte(designGuidelinesContent), 0644); err != nil {
		return fmt.Errorf("writing %s: %w", designGuidelinesFile, err)
	}
	fmt.Println("Created DESIGN_GUIDELINES.md — include this file in your project so AI tools follow the rendering constraints.")
	return nil
}
