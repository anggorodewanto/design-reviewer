# Phase 30: Dockerfile Runs as Root

**Severity:** 🟡 Medium

## Problem

The runtime container has no `USER` directive and runs as root.

## Fix

Add a non-root user in the Dockerfile:

```dockerfile
FROM alpine:3.21
RUN apk add --no-cache ca-certificates && adduser -D -u 1001 appuser
WORKDIR /app
COPY --from=builder /app/server .
COPY --from=builder /app/design-reviewer .
COPY web/ ./web/
USER appuser
EXPOSE 8080
CMD ["./server", "--port", "8080", "--db", "/data/design-reviewer.db", "--uploads", "/data/uploads"]
```

## Files to Change

- `Dockerfile`

## Acceptance Criteria

- Container runs as non-root user `appuser`
- Application still starts and functions correctly
- Data directory permissions allow the non-root user to read/write
