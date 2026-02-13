# Stage 1: Build
FROM golang:1.25-alpine AS builder
RUN apk add --no-cache gcc musl-dev
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=1 go build -o server ./cmd/server
RUN CGO_ENABLED=1 go build -o design-reviewer ./cmd/cli

# Stage 2: Runtime
FROM alpine:3.21
RUN apk add --no-cache ca-certificates
WORKDIR /app
COPY --from=builder /app/server .
COPY --from=builder /app/design-reviewer .
COPY web/ ./web/
EXPOSE 8080
CMD ["./server", "--port", "8080", "--db", "/data/design-reviewer.db", "--uploads", "/data/uploads"]
