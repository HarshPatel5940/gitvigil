# =============================================================================
# GitVigil Makefile
# =============================================================================

.PHONY: all build run test fmt vet clean dev \
        docker-build docker-up docker-down docker-logs docker-clean \
        help

# Go parameters
BINARY_NAME := gitvigil
MAIN_PATH := ./cmd/main.go
GO := go

# Docker parameters
DOCKER_IMAGE := gitvigil
DOCKER_TAG := latest

# =============================================================================
# Development
# =============================================================================

## build: Build the binary
build:
	@echo "Building $(BINARY_NAME)..."
	@$(GO) build -o $(BINARY_NAME) $(MAIN_PATH)
	@echo "Done!"

## run: Run the application locally
run:
	@$(GO) run $(MAIN_PATH)

## test: Run all tests
test:
	@echo "Running tests..."
	@$(GO) test -v ./...

## test-coverage: Run tests with coverage
test-coverage:
	@echo "Running tests with coverage..."
	@$(GO) test -coverprofile=coverage.out ./...
	@$(GO) tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report: coverage.html"

## fmt: Format code
fmt:
	@echo "Formatting code..."
	@$(GO) fmt ./...

## vet: Run go vet
vet:
	@echo "Running go vet..."
	@$(GO) vet ./...

## lint: Run linter (requires golangci-lint)
lint:
	@echo "Running linter..."
	@golangci-lint run ./...

## clean: Clean build artifacts
clean:
	@echo "Cleaning..."
	@rm -f $(BINARY_NAME)
	@rm -f coverage.out coverage.html
	@$(GO) clean
	@echo "Done!"

## dev: Format, vet, and build
dev: fmt vet build

## tidy: Tidy go modules
tidy:
	@$(GO) mod tidy

# =============================================================================
# Docker
# =============================================================================

## docker-build: Build Docker image
docker-build:
	@echo "Building Docker image..."
	@docker build -t $(DOCKER_IMAGE):$(DOCKER_TAG) .
	@echo "Done!"

## docker-up: Start services with Docker Compose
docker-up:
	@echo "Starting services..."
	@docker compose up -d
	@echo "Services started. Access at http://localhost:8080"

## docker-down: Stop Docker Compose services
docker-down:
	@echo "Stopping services..."
	@docker compose down

## docker-logs: View application logs
docker-logs:
	@docker compose logs -f app

## docker-logs-db: View database logs
docker-logs-db:
	@docker compose logs -f db

## docker-restart: Restart the application
docker-restart:
	@docker compose restart app

## docker-clean: Stop services and remove volumes
docker-clean:
	@echo "Stopping services and removing volumes..."
	@docker compose down -v
	@echo "Done!"

## docker-shell: Open shell in app container
docker-shell:
	@docker compose exec app sh

## docker-db-shell: Open psql in database container
docker-db-shell:
	@docker compose exec db psql -U gitvigil -d gitvigil

# =============================================================================
# Database
# =============================================================================

## db-migrate: Run database migrations (local)
db-migrate:
	@echo "Running migrations..."
	@$(GO) run $(MAIN_PATH) migrate

# =============================================================================
# Help
# =============================================================================

## help: Show this help message
help:
	@echo "GitVigil - Hackathon Monitoring Service"
	@echo ""
	@echo "Usage: make [target]"
	@echo ""
	@echo "Development:"
	@sed -n 's/^## //p' $(MAKEFILE_LIST) | grep -E '^[a-z]' | grep -v 'docker-' | column -t -s ':'
	@echo ""
	@echo "Docker:"
	@sed -n 's/^## //p' $(MAKEFILE_LIST) | grep -E '^docker-' | column -t -s ':'

# Default target
all: dev
