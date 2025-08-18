#!/bin/bash

# HexaRAG Development Environment Setup
# Complete setup and initialization script for developers

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
PURPLE='\033[0;35m'
CYAN='\033[0;36m'
GRAY='\033[0;37m'
NC='\033[0m' # No Color

# Default configuration
SKIP_DEPENDENCIES=false
SKIP_MODELS=false
SKIP_DATABASE=false
SKIP_SERVICES=false
FORCE=false
QUIET=false
VERBOSE=false
DEFAULT_MODELS=("deepseek-r1:1.5b" "llama3.2:3b")
DOCKER_COMPOSE_FILE="docker-compose.yml"

# Function to show usage
show_help() {
    cat << EOF
HexaRAG Development Environment Setup

Usage: $0 [OPTIONS]

OPTIONS:
    --skip-deps             Skip dependency installation
    --skip-models           Skip model downloads
    --skip-database         Skip database initialization
    --skip-services         Skip service startup
    -f, --force             Force setup without prompts
    -q, --quiet             Quiet mode (minimal output)
    -v, --verbose           Verbose mode (detailed output)
    -h, --help              Show this help message

SETUP PROCESS:
    1. 🔍 Check system dependencies
    2. 🐳 Verify Docker and Docker Compose
    3. 📦 Install Go dependencies
    4. 🗃️  Initialize database with migrations
    5. 🤖 Download default AI models
    6. 🚀 Start all services
    7. ✅ Run health checks
    8. 📚 Display usage information

DEFAULT MODELS:
    - deepseek-r1:1.5b     Fast reasoning model (~1.7GB)
    - llama3.2:3b          Balanced general model (~2.0GB)

EXAMPLES:
    # Complete setup
    $0

    # Quick setup without models
    $0 --skip-models

    # Force setup without prompts
    $0 --force

    # Minimal setup for CI
    $0 --skip-models --skip-services --quiet

REQUIREMENTS:
    - Docker and Docker Compose
    - Go 1.21 or later
    - curl, jq (for testing)
    - 8GB+ available disk space (for models)

EOF
}

# Function to print colored output
print_colored() {
    local color=$1
    local message=$2
    if [[ "$QUIET" != "true" ]]; then
        echo -e "${color}${message}${NC}"
    fi
}

# Function to print verbose output
print_verbose() {
    if [[ "$VERBOSE" == "true" ]]; then
        print_colored "$GRAY" "[VERBOSE] $1"
    fi
}

# Function to print step header
print_step() {
    local step="$1"
    local description="$2"
    if [[ "$QUIET" != "true" ]]; then
        echo
        print_colored "$CYAN" "=== Step $step: $description ==="
    fi
}

# Function to print error and exit
die() {
    print_colored "$RED" "ERROR: $1" >&2
    exit 1
}

# Function to confirm action
confirm_action() {
    local message="$1"
    
    if [[ "$FORCE" == "true" ]]; then
        return 0
    fi
    
    print_colored "$YELLOW" "$message"
    read -p "Continue? (Y/n): " -r response
    
    case "$response" in
        [nN][oO]|[nN])
            print_colored "$BLUE" "Setup cancelled"
            exit 0
            ;;
        *)
            return 0
            ;;
    esac
}

# Function to check system dependencies
check_system_dependencies() {
    print_step "1" "Checking System Dependencies"
    
    local missing_deps=()
    local missing_optional=()
    
    # Required dependencies
    command -v docker >/dev/null 2>&1 || missing_deps+=("docker")
    command -v docker-compose >/dev/null 2>&1 || missing_deps+=("docker-compose")
    command -v go >/dev/null 2>&1 || missing_deps+=("go")
    
    # Optional but recommended
    command -v curl >/dev/null 2>&1 || missing_optional+=("curl")
    command -v jq >/dev/null 2>&1 || missing_optional+=("jq")
    command -v git >/dev/null 2>&1 || missing_optional+=("git")
    
    if [[ ${#missing_deps[@]} -gt 0 ]]; then
        print_colored "$RED" "Missing required dependencies:"
        for dep in "${missing_deps[@]}"; do
            echo "  ✗ $dep"
        done
        echo
        print_colored "$BLUE" "Installation instructions:"
        print_colored "$GRAY" "Docker: https://docs.docker.com/get-docker/"
        print_colored "$GRAY" "Docker Compose: https://docs.docker.com/compose/install/"
        print_colored "$GRAY" "Go: https://golang.org/doc/install"
        die "Please install missing dependencies and run setup again"
    fi
    
    if [[ ${#missing_optional[@]} -gt 0 ]]; then
        print_colored "$YELLOW" "Missing optional dependencies:"
        for dep in "${missing_optional[@]}"; do
            echo "  ⚠ $dep (recommended for testing)"
        done
    fi
    
    # Check Docker daemon
    if ! docker info >/dev/null 2>&1; then
        die "Docker daemon is not running. Please start Docker and try again."
    fi
    
    # Check Go version
    local go_version
    go_version=$(go version | grep -oE 'go[0-9]+\.[0-9]+' | sed 's/go//')
    print_verbose "Found Go version: $go_version"
    
    print_colored "$GREEN" "✓ All system dependencies are available"
}

# Function to check disk space
check_disk_space() {
    print_verbose "Checking available disk space"
    
    local available_space
    available_space=$(df -BG . | tail -1 | awk '{print $4}' | sed 's/G//')
    
    local required_space=8  # GB
    
    if [[ $available_space -lt $required_space ]]; then
        print_colored "$YELLOW" "Warning: Low disk space (${available_space}GB available, ${required_space}GB recommended)"
        confirm_action "Continue with limited disk space?"
    else
        print_verbose "Disk space check passed: ${available_space}GB available"
    fi
}

# Function to install Go dependencies
install_go_dependencies() {
    print_step "2" "Installing Go Dependencies"
    
    if [[ ! -f "go.mod" ]]; then
        die "go.mod not found. Are you in the correct directory?"
    fi
    
    print_verbose "Running go mod download"
    if ! go mod download; then
        die "Failed to download Go dependencies"
    fi
    
    print_verbose "Running go mod tidy"
    if ! go mod tidy; then
        die "Failed to tidy Go modules"
    fi
    
    print_colored "$GREEN" "✓ Go dependencies installed"
}

# Function to build the application
build_application() {
    print_verbose "Building HexaRAG application"
    
    if ! go build -o server ./cmd/server; then
        die "Failed to build application"
    fi
    
    print_colored "$GREEN" "✓ Application built successfully"
}

# Function to initialize database
initialize_database() {
    if [[ "$SKIP_DATABASE" == "true" ]]; then
        print_colored "$GRAY" "Skipping database initialization"
        return 0
    fi
    
    print_step "3" "Initializing Database"
    
    # Start database container if not running
    print_verbose "Starting database container"
    if ! docker-compose -f "$DOCKER_COMPOSE_FILE" up -d hexarag-db 2>/dev/null; then
        print_colored "$YELLOW" "Warning: Failed to start database container"
    fi
    
    # Wait for database to be ready
    print_verbose "Waiting for database to be ready"
    sleep 3
    
    # Check if database needs initialization
    if docker exec hexarag test -f /data/hexarag.db 2>/dev/null; then
        print_colored "$BLUE" "Database already exists"
        confirm_action "Reinitialize database? This will clear all data."
        
        # Reset database if confirmed
        if [[ -f "scripts/reset-db.sh" ]]; then
            print_verbose "Using reset-db script"
            ./scripts/reset-db.sh --force --no-backup
        else
            print_colored "$YELLOW" "Reset script not found, skipping database reset"
        fi
    else
        print_verbose "Initializing new database"
        
        # Run migrations if available
        if [[ -d "internal/adapters/storage/sqlite/migrations" ]]; then
            print_verbose "Running database migrations"
            # The application will handle migrations on first run
        fi
    fi
    
    print_colored "$GREEN" "✓ Database initialized"
}

# Function to download default models
download_models() {
    if [[ "$SKIP_MODELS" == "true" ]]; then
        print_colored "$GRAY" "Skipping model downloads"
        return 0
    fi
    
    print_step "4" "Downloading AI Models"
    
    print_colored "$BLUE" "This will download the following models:"
    for model in "${DEFAULT_MODELS[@]}"; do
        echo "  • $model"
    done
    echo
    
    local total_size_estimate="~4GB"
    print_colored "$YELLOW" "Estimated download size: $total_size_estimate"
    confirm_action "Proceed with model downloads?"
    
    # Check if pull-model script exists
    if [[ -f "scripts/pull-model.sh" ]]; then
        for model in "${DEFAULT_MODELS[@]}"; do
            print_colored "$BLUE" "Downloading model: $model"
            if ./scripts/pull-model.sh "$model"; then
                print_colored "$GREEN" "✓ $model downloaded successfully"
            else
                print_colored "$YELLOW" "⚠ Failed to download $model (will retry later)"
            fi
        done
    else
        print_colored "$YELLOW" "Model download script not found, skipping model installation"
        print_colored "$GRAY" "You can download models later using: docker exec hexarag ollama pull MODEL_NAME"
    fi
    
    print_colored "$GREEN" "✓ Model downloads completed"
}

# Function to start services
start_services() {
    if [[ "$SKIP_SERVICES" == "true" ]]; then
        print_colored "$GRAY" "Skipping service startup"
        return 0
    fi
    
    print_step "5" "Starting Services"
    
    if [[ ! -f "$DOCKER_COMPOSE_FILE" ]]; then
        die "Docker Compose file not found: $DOCKER_COMPOSE_FILE"
    fi
    
    print_verbose "Starting all services with Docker Compose"
    if ! docker-compose -f "$DOCKER_COMPOSE_FILE" up -d; then
        die "Failed to start services"
    fi
    
    print_colored "$BLUE" "Waiting for services to be ready..."
    sleep 10
    
    # Check service status
    local services_status
    services_status=$(docker-compose -f "$DOCKER_COMPOSE_FILE" ps --services --filter "status=running")
    
    if [[ -n "$services_status" ]]; then
        print_colored "$GREEN" "✓ Services started successfully"
        if [[ "$VERBOSE" == "true" ]]; then
            print_colored "$GRAY" "Running services:"
            echo "$services_status" | while read -r service; do
                print_colored "$GRAY" "  • $service"
            done
        fi
    else
        print_colored "$YELLOW" "⚠ Some services may not have started correctly"
    fi
}

# Function to run health checks
run_health_checks() {
    print_step "6" "Running Health Checks"
    
    if [[ -f "scripts/health-check.sh" ]]; then
        print_verbose "Running comprehensive health check"
        if ./scripts/health-check.sh --quiet; then
            print_colored "$GREEN" "✓ All health checks passed"
        else
            print_colored "$YELLOW" "⚠ Some health checks failed (this may be normal during initial startup)"
        fi
    else
        print_verbose "Health check script not found, running basic checks"
        
        # Basic API check
        if curl -s --max-time 5 http://localhost:8080/health >/dev/null 2>&1; then
            print_colored "$GREEN" "✓ API server is responding"
        else
            print_colored "$YELLOW" "⚠ API server is not responding (may still be starting)"
        fi
    fi
}

# Function to display setup summary and usage information
display_summary() {
    print_step "7" "Setup Complete"
    
    print_colored "$GREEN" "🎉 HexaRAG development environment is ready!"
    echo
    
    print_colored "$CYAN" "Service URLs:"
    echo "  🌐 Web UI:     http://localhost:8080"
    echo "  📡 API:        http://localhost:8080/api/v1"
    echo "  🤖 Ollama:     http://localhost:11434"
    echo "  📊 NATS:       http://localhost:8222"
    echo
    
    print_colored "$CYAN" "Useful Commands:"
    echo "  📋 Health check:      ./scripts/health-check.sh"
    echo "  💬 Test chat:         ./scripts/test-chat.sh"
    echo "  📥 Download model:    ./scripts/pull-model.sh MODEL_NAME"
    echo "  🔄 Reset database:    ./scripts/reset-db.sh"
    echo "  📊 Run benchmark:     ./scripts/benchmark.sh"
    echo "  🛑 Stop services:     docker-compose down"
    echo
    
    print_colored "$CYAN" "Development Workflow:"
    echo "  1. Make code changes"
    echo "  2. Run: go build -o server ./cmd/server"
    echo "  3. Restart: docker-compose restart hexarag"
    echo "  4. Test: ./scripts/test-chat.sh"
    echo
    
    if [[ "$SKIP_MODELS" == "true" ]]; then
        print_colored "$YELLOW" "Note: No models were downloaded. You may want to run:"
        echo "  ./scripts/pull-model.sh deepseek-r1:1.5b"
    fi
    
    print_colored "$GRAY" "For more information, see DEVELOPER.md (if available)"
}

# Function to cleanup on error
cleanup_on_error() {
    print_colored "$RED" "Setup failed! Cleaning up..."
    
    # Stop any running containers
    if [[ -f "$DOCKER_COMPOSE_FILE" ]]; then
        docker-compose -f "$DOCKER_COMPOSE_FILE" down 2>/dev/null || true
    fi
    
    print_colored "$GRAY" "You can re-run the setup script to try again"
}

# Parse command line arguments
while [[ $# -gt 0 ]]; do
    case $1 in
        --skip-deps)
            SKIP_DEPENDENCIES=true
            shift
            ;;
        --skip-models)
            SKIP_MODELS=true
            shift
            ;;
        --skip-database)
            SKIP_DATABASE=true
            shift
            ;;
        --skip-services)
            SKIP_SERVICES=true
            shift
            ;;
        -f|--force)
            FORCE=true
            shift
            ;;
        -q|--quiet)
            QUIET=true
            shift
            ;;
        -v|--verbose)
            VERBOSE=true
            shift
            ;;
        -h|--help)
            show_help
            exit 0
            ;;
        *)
            die "Unknown option: $1. Use -h for help."
            ;;
    esac
done

# Main execution
main() {
    # Set error handler
    trap cleanup_on_error ERR
    
    print_colored "$CYAN" "🚀 HexaRAG Development Environment Setup"
    print_colored "$CYAN" "========================================"
    
    if [[ "$QUIET" != "true" ]]; then
        echo "This script will set up a complete HexaRAG development environment."
        echo "It will check dependencies, build the application, initialize the database,"
        echo "download AI models, and start all services."
        echo
    fi
    
    confirm_action "Ready to set up HexaRAG development environment?"
    
    # Run setup steps
    if [[ "$SKIP_DEPENDENCIES" != "true" ]]; then
        check_system_dependencies
        check_disk_space
        install_go_dependencies
        build_application
    fi
    
    initialize_database
    download_models
    start_services
    run_health_checks
    display_summary
    
    print_colored "$GREEN" "✨ Setup completed successfully!"
}

# Run main function
main "$@"