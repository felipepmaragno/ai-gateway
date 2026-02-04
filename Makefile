.PHONY: build run test test-race test-cover lint clean dev migrate-up migrate-down

# Build
build:
	go build -o bin/aigateway ./cmd/aigateway

# Run
run: build
	./bin/aigateway

# Development with hot reload (requires air)
dev:
	air

# Tests
test:
	go test -v ./...

test-race:
	go test -race -v ./...

test-cover:
	go test -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report: coverage.html"

# Lint (requires golangci-lint)
lint:
	golangci-lint run

# Database migrations
migrate-up:
	go run ./cmd/aigateway migrate up

migrate-down:
	go run ./cmd/aigateway migrate down

# Clean
clean:
	rm -rf bin/
	rm -f coverage.out coverage.html

# Docker
docker-build:
	docker build -t aigateway:latest .

docker-up:
	docker compose up -d

docker-down:
	docker compose down

# Help
help:
	@echo "Available targets:"
	@echo "  build        - Build the binary"
	@echo "  run          - Build and run"
	@echo "  dev          - Run with hot reload"
	@echo "  test         - Run tests"
	@echo "  test-race    - Run tests with race detector"
	@echo "  test-cover   - Run tests with coverage"
	@echo "  lint         - Run linter"
	@echo "  migrate-up   - Run database migrations"
	@echo "  migrate-down - Rollback database migrations"
	@echo "  clean        - Remove build artifacts"
	@echo "  docker-build - Build Docker image"
	@echo "  docker-up    - Start Docker Compose"
	@echo "  docker-down  - Stop Docker Compose"
