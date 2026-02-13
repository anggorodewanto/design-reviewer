# Design Reviewer

Internal tool for reviewing UI/UX design mockups built as HTML+CSS. Designers push mockups via CLI to a collaborative web app where teams can view and annotate designs with pin comments.

## Prerequisites

- Go 1.22+ with GCC (for SQLite CGO)
- Google OAuth credentials (for authentication)

## Environment Variables

| Variable | Description |
|----------|-------------|
| `GOOGLE_CLIENT_ID` | Google OAuth client ID |
| `GOOGLE_CLIENT_SECRET` | Google OAuth client secret |
| `SESSION_SECRET` | Random 32+ character string for session encryption |
| `BASE_URL` | Public URL of the server (e.g., `http://localhost:8080`) |

## Local Development

```bash
# Start the server
go run ./cmd/server --port 8080 --db ./data/design-reviewer.db --uploads ./data/uploads

# Build the CLI
go build -o design-reviewer ./cmd/cli
```

## Deployment (Fly.io)

1. Install [flyctl](https://fly.io/docs/hands-on/install-flyctl/)
2. Run `fly launch` for first-time setup
3. Set secrets:
   ```bash
   fly secrets set GOOGLE_CLIENT_ID=your-client-id
   fly secrets set GOOGLE_CLIENT_SECRET=your-client-secret
   fly secrets set SESSION_SECRET=random-32-char-string
   ```
4. Deploy: `fly deploy` (or `bash scripts/deploy.sh` which also creates the volume)
