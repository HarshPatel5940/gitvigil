# =============================================================================
# GitVigil Dockerfile
# Multi-stage build for minimal, secure production image
# =============================================================================

# -----------------------------------------------------------------------------
# Build Stage
# -----------------------------------------------------------------------------
FROM golang:1.23-bookworm AS builder

WORKDIR /app

# Download dependencies first (better caching)
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build the binary
# - CGO_ENABLED=0: Static binary, no C dependencies
# - -ldflags="-s -w": Strip debug info for smaller binary
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags="-s -w" \
    -o /gitvigil \
    ./cmd/main.go

# -----------------------------------------------------------------------------
# Runtime Stage
# -----------------------------------------------------------------------------
FROM debian:bookworm-slim

# Install runtime dependencies
RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates \
    && rm -rf /var/lib/apt/lists/*

# Create non-root user
RUN useradd -r -u 1001 appuser

WORKDIR /app

# Copy binary from builder
COPY --from=builder /gitvigil .

# Copy migrations
COPY --from=builder /app/internal/database/migrations ./internal/database/migrations

# Use non-root user
USER appuser

# Expose default port
EXPOSE 8080

# Health check
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:8080/health || exit 1

# Run the application
ENTRYPOINT ["/app/gitvigil"]
