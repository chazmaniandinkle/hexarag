#!/bin/bash

# HexaRAG Ollama Setup Script
# Detects host system configuration and sets up optimal Ollama integration

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Functions
log_info() { echo -e "${BLUE}[INFO]${NC} $1"; }
log_success() { echo -e "${GREEN}[SUCCESS]${NC} $1"; }
log_warning() { echo -e "${YELLOW}[WARNING]${NC} $1"; }
log_error() { echo -e "${RED}[ERROR]${NC} $1"; }

# Detect OS and set default Ollama path
detect_ollama_path() {
    if [[ "$OSTYPE" == "darwin"* ]]; then
        echo "$HOME/.ollama"
    elif [[ "$OSTYPE" == "linux"* ]]; then
        echo "$HOME/.ollama"
    elif [[ "$OSTYPE" == "msys" ]] || [[ "$OSTYPE" == "cygwin" ]]; then
        echo "$USERPROFILE/.ollama"
    else
        echo "$HOME/.ollama"
    fi
}

# Check if local Ollama is running
check_local_ollama() {
    if curl -s http://localhost:11434/api/tags >/dev/null 2>&1; then
        return 0
    else
        return 1
    fi
}

# Get available models from Ollama
get_ollama_models() {
    if check_local_ollama; then
        curl -s http://localhost:11434/api/tags | jq -r '.models[]?.name // empty' 2>/dev/null || echo ""
    else
        echo ""
    fi
}

# Check if Docker is running
check_docker() {
    if docker info >/dev/null 2>&1; then
        return 0
    else
        return 1
    fi
}

# Main setup function
main() {
    echo "🚀 HexaRAG Ollama Setup"
    echo "======================="
    echo ""

    # Check prerequisites
    log_info "Checking system prerequisites..."
    
    if ! command -v docker &> /dev/null; then
        log_error "Docker is not installed. Please install Docker first."
        exit 1
    fi

    if ! check_docker; then
        log_error "Docker is not running. Please start Docker first."
        exit 1
    fi

    # Detect Ollama path
    OLLAMA_PATH=$(detect_ollama_path)
    log_info "Detected Ollama path: $OLLAMA_PATH"

    # Check if local Ollama is running
    if check_local_ollama; then
        log_success "Local Ollama detected and running"
        
        # Get available models
        MODELS=$(get_ollama_models)
        if [[ -n "$MODELS" ]]; then
            log_info "Available models:"
            echo "$MODELS" | while read -r model; do
                echo "  - $model"
            done
            echo ""
            
            # Check for deepseek-r1:8b
            if echo "$MODELS" | grep -q "deepseek-r1:8b"; then
                log_success "Found deepseek-r1:8b model"
                DEFAULT_MODEL="deepseek-r1:8b"
            else
                log_warning "deepseek-r1:8b model not found"
                DEFAULT_MODEL=$(echo "$MODELS" | head -n1)
                if [[ -n "$DEFAULT_MODEL" ]]; then
                    log_info "Will use: $DEFAULT_MODEL"
                fi
            fi
        else
            log_warning "No models found in local Ollama"
            DEFAULT_MODEL="deepseek-r1:8b"
        fi

        # Ask user about configuration mode
        echo "Select Ollama configuration mode:"
        echo "1) Managed (recommended) - Use containerized Ollama with your existing models"
        echo "2) External - Connect to your running Ollama instance"
        echo ""
        read -p "Choose option (1-2) [1]: " CHOICE
        CHOICE=${CHOICE:-1}

        case $CHOICE in
            1)
                OLLAMA_MODE="managed"
                OLLAMA_URL="http://ollama:11434"
                log_info "Using managed mode - container will share your model storage"
                ;;
            2)
                OLLAMA_MODE="external"
                OLLAMA_URL="http://host.docker.internal:11434"
                log_info "Using external mode - will connect to your local Ollama"
                ;;
            *)
                log_error "Invalid choice"
                exit 1
                ;;
        esac
    else
        log_info "No local Ollama detected, using managed mode"
        OLLAMA_MODE="managed"
        OLLAMA_URL="http://ollama:11434"
        DEFAULT_MODEL="deepseek-r1:8b"
    fi

    # Check if host Ollama path exists and is readable
    if [[ "$OLLAMA_MODE" == "managed" ]]; then
        if [[ -d "$OLLAMA_PATH" ]]; then
            log_success "Host Ollama directory found: $OLLAMA_PATH"
        else
            log_warning "Host Ollama directory not found: $OLLAMA_PATH"
            log_info "Container will start with empty model storage"
        fi
    fi

    # Create .env file
    log_info "Creating .env configuration..."
    
    cat > .env << EOF
# HexaRAG Configuration - Generated $(date)
# Ollama Configuration
OLLAMA_MODE=$OLLAMA_MODE
OLLAMA_MODELS_PATH=$OLLAMA_PATH
HEXARAG_LLM_BASE_URL=$OLLAMA_URL/v1
HEXARAG_LLM_MODEL=$DEFAULT_MODEL

# Docker Compose Profiles
COMPOSE_PROFILES=full-stack

# Application defaults
HEXARAG_SERVER_PORT=8080
HEXARAG_LOGGING_LEVEL=info
EOF

    log_success ".env file created"

    # Set appropriate Docker Compose profiles
    if [[ "$OLLAMA_MODE" == "external" ]]; then
        export COMPOSE_PROFILES="minimal"
        log_info "Set COMPOSE_PROFILES=minimal (no Ollama container)"
    else
        export COMPOSE_PROFILES="full-stack"
        log_info "Set COMPOSE_PROFILES=full-stack (with Ollama container)"
    fi

    # Start the services
    echo ""
    log_info "Starting HexaRAG services..."
    
    cd deployments/docker
    
    if [[ "$OLLAMA_MODE" == "external" ]]; then
        # Don't start Ollama container for external mode
        docker-compose up -d nats hexarag
    else
        # Start all services including Ollama
        docker-compose --profile full-stack up -d
    fi

    # Wait a bit for services to start
    sleep 5

    # Test the setup
    log_info "Testing setup..."
    
    # Check health endpoint
    if curl -s http://localhost:8080/health >/dev/null 2>&1; then
        log_success "HexaRAG API is responding"
    else
        log_warning "HexaRAG API not yet ready (may take a moment)"
    fi

    # Final instructions
    echo ""
    log_success "Setup complete!"
    echo ""
    echo "🎉 HexaRAG is now running with $OLLAMA_MODE Ollama mode"
    echo ""
    echo "Next steps:"
    echo "  • API available at: http://localhost:8080"
    echo "  • Health check:     http://localhost:8080/health"
    echo "  • Chat UI:          http://localhost:8080 (coming soon)"
    echo ""
    
    if [[ "$OLLAMA_MODE" == "managed" && "$DEFAULT_MODEL" == "deepseek-r1:8b" && ! $(echo "$MODELS" | grep -q "deepseek-r1:8b") ]]; then
        echo "To pull the deepseek-r1:8b model:"
        echo "  docker-compose exec ollama ollama pull deepseek-r1:8b"
        echo ""
    fi
    
    echo "To stop services:"
    echo "  cd deployments/docker && docker-compose down"
    echo ""
    echo "View logs:"
    echo "  cd deployments/docker && docker-compose logs -f hexarag"
}

# Run main function
main "$@"