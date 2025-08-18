# HexaRAG Makefile

.PHONY: build run test clean migrate docker-build docker-run deps fmt lint vet

# Build the application
build:
	@echo "Building HexaRAG..."
	go build -o bin/hexarag ./cmd/server
	go build -o bin/migrate ./cmd/migrate

# Run the application
run:
	@echo "Starting HexaRAG server..."
	go run ./cmd/server

# Run database migrations
migrate:
	@echo "Running database migrations..."
	go run ./cmd/migrate

# Run tests
test:
	@echo "Running tests..."
	go test -v ./...

# Run tests with coverage
test-coverage:
	@echo "Running tests with coverage..."
	go test -v -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html

# Clean build artifacts
clean:
	@echo "Cleaning..."
	rm -rf bin/
	rm -f coverage.out coverage.html

# Install dependencies
deps:
	@echo "Installing dependencies..."
	go mod download
	go mod tidy

# Format code
fmt:
	@echo "Formatting code..."
	go fmt ./...

# Lint code
lint:
	@echo "Linting code..."
	golangci-lint run

# Vet code
vet:
	@echo "Vetting code..."
	go vet ./...

# Build Docker image
docker-build:
	@echo "Building Docker image..."
	docker build -t hexarag:latest .

# Run with Docker Compose
docker-run:
	@echo "Starting services with Docker Compose..."
	docker-compose up --build

# Stop Docker Compose services
docker-stop:
	@echo "Stopping Docker Compose services..."
	docker-compose down

# Development setup
dev-setup: deps migrate
	@echo "Development environment ready!"

# Run development server with auto-reload (requires air: go install github.com/cosmtrek/air@latest)
dev:
	@echo "Starting development server with auto-reload..."
	air

# Check if dependencies are available
check-deps:
	@echo "Checking dependencies..."
	@which go >/dev/null || (echo "Go is not installed" && exit 1)
	@echo "✓ Go is installed"
	@go version

# Help
help:
	@echo "Available targets:"
	@echo "  build        - Build the application"
	@echo "  run          - Run the application"
	@echo "  migrate      - Run database migrations"
	@echo "  test         - Run tests"
	@echo "  test-coverage - Run tests with coverage report"
	@echo "  clean        - Clean build artifacts"
	@echo "  deps         - Install dependencies"
	@echo "  fmt          - Format code"
	@echo "  lint         - Lint code"
	@echo "  vet          - Vet code"
	@echo "  docker-build - Build Docker image"
	@echo "  docker-run   - Run with Docker Compose"
	@echo "  docker-stop  - Stop Docker Compose services"
	@echo "  dev-setup    - Setup development environment"
	@echo "  dev          - Run development server with auto-reload"
	@echo "  check-deps   - Check if dependencies are available"
	@echo "  help         - Show this help message"