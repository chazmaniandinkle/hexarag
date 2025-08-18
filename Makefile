# HexaRAG Makefile

.PHONY: build run test clean migrate docker-build docker-run deps fmt lint vet setup chat test-model models test-chat pull-model reset-db health benchmark dev-setup-full

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

# Interactive setup with Ollama detection
setup:
	@echo "🚀 Running HexaRAG setup..."
	@chmod +x scripts/setup-ollama.sh
	@./scripts/setup-ollama.sh

# Launch chat UI in browser
chat:
	@echo "🗣️ Opening chat interface..."
	@open http://localhost:8080 2>/dev/null || xdg-open http://localhost:8080 2>/dev/null || echo "Please open http://localhost:8080 in your browser"

# Test current model configuration
test-model:
	@echo "🤖 Testing model connectivity..."
	@curl -s http://localhost:8080/api/v1/conversations | jq -r 'if .conversations then "✓ API responding" else "✗ API error" end' || echo "✗ API not available"

# List and manage models
models:
	@echo "📋 Available models:"
	@docker-compose -f deployments/docker/docker-compose.yml exec ollama ollama list 2>/dev/null || echo "Ollama container not running"

# Developer Scripts (Phase 3D)

# Interactive chat testing
test-chat:
	@echo "💬 Starting interactive chat test..."
	@chmod +x scripts/test-chat.sh
	@./scripts/test-chat.sh

# Pull specific model
pull-model:
	@echo "📥 Downloading model..."
	@chmod +x scripts/pull-model.sh
	@./scripts/pull-model.sh $(MODEL)

# Reset database with backup
reset-db:
	@echo "🗑️ Resetting database..."
	@chmod +x scripts/reset-db.sh
	@./scripts/reset-db.sh

# Comprehensive health check
health:
	@echo "🏥 Running system health check..."
	@chmod +x scripts/health-check.sh
	@./scripts/health-check.sh

# Performance benchmark
benchmark:
	@echo "📊 Running performance benchmark..."
	@chmod +x scripts/benchmark.sh
	@./scripts/benchmark.sh

# Complete development environment setup
dev-setup-full:
	@echo "🚀 Setting up complete development environment..."
	@chmod +x scripts/dev-setup.sh
	@./scripts/dev-setup.sh

# Help
help:
	@echo "HexaRAG Makefile Commands:"
	@echo ""
	@echo "🚀 Getting Started:"
	@echo "  setup          - Interactive setup with Ollama detection"
	@echo "  chat           - Launch chat interface in browser"
	@echo "  test-model     - Test current model configuration"
	@echo ""
	@echo "🛠️ Development:"
	@echo "  build          - Build the application"
	@echo "  run            - Run the application locally"
	@echo "  migrate        - Run database migrations"
	@echo "  dev-setup      - Setup development environment"
	@echo "  dev            - Run development server with auto-reload"
	@echo ""
	@echo "🧪 Testing & Quality:"
	@echo "  test           - Run tests"
	@echo "  test-coverage  - Run tests with coverage report"
	@echo "  fmt            - Format code"
	@echo "  lint           - Lint code"
	@echo "  vet            - Vet code"
	@echo ""
	@echo "🐳 Docker:"
	@echo "  docker-build   - Build Docker image"
	@echo "  docker-run     - Run with Docker Compose"
	@echo "  docker-stop    - Stop Docker Compose services"
	@echo ""
	@echo "🔧 Utilities:"
	@echo "  models         - List available models"
	@echo "  reset-db       - Reset database for testing"
	@echo "  clean          - Clean build artifacts"
	@echo "  deps           - Install dependencies"
	@echo "  check-deps     - Check if dependencies are available"
	@echo ""
	@echo "🛠️ Developer Scripts (Phase 3D):"
	@echo "  test-chat      - Interactive chat testing tool"
	@echo "  pull-model     - Download AI model (use MODEL=name)"
	@echo "  health         - Comprehensive system health check"
	@echo "  benchmark      - Performance testing and benchmarking"
	@echo "  dev-setup-full - Complete development environment setup"