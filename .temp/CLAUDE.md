# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

GitVigil is a hackathon monitoring GitHub App that detects commit backdating, monitors force-pushes, validates licenses, tracks activity streaks, and provides reviewer insights.

**Module:** `github.com/harshpatel5940/gitvigil`

## Development Commands

```bash
# Build
go build -o gitvigil ./cmd/main.go

# Run (requires PostgreSQL and .env configured)
go run ./cmd/main.go

# Test
go test ./...

# Format
go fmt ./...

# Vet
go vet ./...
```

## Architecture

```
cmd/main.go                    # HTTP server, config, graceful shutdown
internal/
├── config/config.go           # Load .env, validate settings
├── database/
│   ├── database.go            # pgx connection pool
│   ├── migrate.go             # Embedded migration runner
│   └── migrations/            # SQL schema files
├── github/
│   └── app.go                 # JWT auth, installation tokens
├── webhook/
│   ├── handler.go             # /webhook endpoint, push event processing
│   └── signature.go           # HMAC-SHA256 verification
├── detection/
│   └── detector.go            # Backdate, license, streak detection
├── analysis/
│   ├── commits.go             # Conventional Commits parsing
│   ├── distribution.go        # Contribution distribution (Gini coefficient)
│   └── volume.go              # Daily builder vs deadline dumper
├── scorecard/
│   └── handler.go             # /scorecard API endpoint
├── auth/
│   └── handler.go             # /auth/github/callback OAuth
├── api/
│   ├── handler.go             # Base API with JSON helpers
│   ├── repositories.go        # Repository list/get endpoints
│   ├── installations.go       # Installation list/get endpoints
│   └── stats.go               # System statistics endpoint
└── models/
    ├── repository.go          # Repository CRUD
    ├── commit.go              # Commit stats
    ├── alert.go               # Alert types and CRUD
    ├── contributor.go         # Contributor stats
    └── installation.go        # Installation CRUD
```

## API Endpoints

- `POST /webhook` - Receives GitHub push/installation events
- `GET /scorecard?repo=owner/name` - Returns repository analysis JSON
- `GET /auth/github/callback` - OAuth callback
- `GET /health` - Health check

**Management API (v1):**
- `GET /api/v1/repositories` - List all monitored repositories
- `GET /api/v1/repositories/:id` - Get single repository
- `GET /api/v1/installations` - List all GitHub App installations
- `GET /api/v1/installations/:id` - Get installation details
- `GET /api/v1/installations/:id/repositories` - List repos for installation
- `GET /api/v1/stats` - System-wide statistics

## Configuration

Environment variables (`.env`):
- `GITHUB_APP_ID`, `GITHUB_APP_CLIENT_ID`, `GITHUB_APP_CLIENT_SECRET`
- `GITHUB_WEBHOOK_SECRET` - For webhook signature verification
- `GITHUB_PRIVATE_KEY_PATH` - Path to RSA private key PEM
- `DATABASE_URL` - PostgreSQL connection string
- `PORT`, `BASE_URL`
- `BACKDATE_SUSPICIOUS_HOURS` (default: 24), `BACKDATE_CRITICAL_HOURS` (default: 72)
- `STREAK_INACTIVITY_HOURS` (default: 72)

## Self-Hosting with Docker

```bash
# Quick start
cp .env.example .env
# Edit .env with your GitHub App credentials

# Start services
make docker-up

# View logs
make docker-logs

# Stop services
make docker-down
```

## Detection Logic

**Backdate Detection**: Compares webhook receive time vs commit `author.date`. Flags commits with >24h difference as suspicious, >72h as critical.

**Force Push**: Detects `forced: true` on push events.

**Streak Tracking**: Marks repos as "at_risk" after 72 hours of inactivity.

**Conventional Commits**: Validates `feat:`, `fix:`, `docs:`, etc. prefixes.

**Contribution Patterns**: Identifies "deadline_dumper" (>50% commits in last 20% of time) vs "daily_builder" (consistent activity).
