# Phase 10: Dockerfile + Fly.io Deployment

## Goal
Create the Dockerfile and Fly.io configuration for production deployment with persistent storage.

## Prerequisites
Phases 1-9 complete — the full application works locally.

## What to Build

### 1. Dockerfile

Multi-stage build:

```dockerfile
# Stage 1: Build
FROM golang:1.22-alpine AS builder
RUN apk add --no-cache gcc musl-dev
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=1 go build -o server ./cmd/server
RUN CGO_ENABLED=1 go build -o design-reviewer ./cmd/cli

# Stage 2: Runtime
FROM alpine:3.19
RUN apk add --no-cache ca-certificates
WORKDIR /app
COPY --from=builder /app/server .
COPY --from=builder /app/design-reviewer .
COPY web/ ./web/
EXPOSE 8080
CMD ["./server", "--port", "8080", "--db", "/data/design-reviewer.db", "--uploads", "/data/uploads"]
```

Notes:
- `CGO_ENABLED=1` is required for go-sqlite3
- `gcc` and `musl-dev` needed for CGO compilation in Alpine
- Copy `web/` directory for templates and static assets
- Data stored in `/data/` which will be a persistent volume

### 2. Fly.io Configuration (`fly.toml`)

```toml
app = "design-reviewer"
primary_region = "sin"  # Singapore, adjust as needed

[build]

[http_service]
  internal_port = 8080
  force_https = true
  auto_stop_machines = true
  auto_start_machines = true
  min_machines_running = 0

[mounts]
  source = "data"
  destination = "/data"

[env]
  BASE_URL = "https://design-reviewer.fly.dev"
```

### 3. Environment Secrets

Document the secrets that need to be set via `fly secrets set`:
```bash
fly secrets set GOOGLE_CLIENT_ID=your-client-id
fly secrets set GOOGLE_CLIENT_SECRET=your-client-secret
fly secrets set SESSION_SECRET=random-32-char-string
```

### 4. Deployment Script (`scripts/deploy.sh`)

```bash
#!/bin/bash
set -e

echo "Deploying design-reviewer to Fly.io..."

# Create volume if first deploy
fly volumes list | grep -q "data" || fly volumes create data --region sin --size 5

# Deploy
fly deploy

echo "Deployed! URL: https://design-reviewer.fly.dev"
```

### 5. `.dockerignore`

```
.git
*.md
phases/
data/
```

### 6. README Update

Create or update `README.md` with:
- Project description (one paragraph)
- Local development setup:
  - Prerequisites: Go 1.22+, GCC (for SQLite)
  - Set up Google OAuth credentials
  - Environment variables needed
  - `go run ./cmd/server` to start server
  - `go build -o design-reviewer ./cmd/cli` to build CLI
- Deployment:
  - Install `flyctl`
  - `fly launch` for first deploy
  - Set secrets
  - `fly deploy` for subsequent deploys

## Verification
- `docker build -t design-reviewer .` succeeds
- `docker run -p 8080:8080 -v $(pwd)/data:/data -e GOOGLE_CLIENT_ID=... -e GOOGLE_CLIENT_SECRET=... -e SESSION_SECRET=... -e BASE_URL=http://localhost:8080 design-reviewer` runs the app
- App works correctly in Docker (upload, view, comment)
- `fly deploy` deploys successfully (if Fly.io account is set up)
- Persistent volume retains data across deploys
