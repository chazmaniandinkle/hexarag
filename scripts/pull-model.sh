#!/bin/bash

# HexaRAG Model Download Utility
# Tool for downloading and managing AI models via the HexaRAG API

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
API_URL="${HEXARAG_API_URL:-http://localhost:8080}"
QUIET=false
VERBOSE=false
LIST_ONLY=false
CHECK_ONLY=false
BATCH_FILE=""
TIMEOUT=300  # 5 minutes default timeout

# Function to show usage
show_help() {
    cat << EOF
HexaRAG Model Download Utility

Usage: $0 [OPTIONS] [MODEL_NAME]

OPTIONS:
    -u, --url URL           API base URL (default: $API_URL)
    -l, --list              List available models and exit
    -c, --check MODEL       Check if model is available (don't download)
    -b, --batch FILE        Download models from file (one per line)
    -t, --timeout SECONDS   Download timeout in seconds (default: 300)
    -q, --quiet             Quiet mode (minimal output)
    -v, --verbose           Verbose mode (detailed output)
    -h, --help              Show this help message

ENVIRONMENT VARIABLES:
    HEXARAG_API_URL         Default API URL

EXAMPLES:
    # List all available models
    $0 --list

    # Download a specific model
    $0 llama3.2:3b

    # Check if model is available
    $0 --check deepseek-r1:8b

    # Download models from a file
    $0 --batch models.txt

    # Download with custom timeout
    $0 --timeout 600 llama3.2:7b

POPULAR MODELS:
    llama3.2:1b             Lightweight model (1.3GB)
    llama3.2:3b             Balanced model (2.0GB)
    deepseek-r1:1.5b        Fast reasoning model (1.7GB)
    deepseek-r1:8b          Advanced reasoning model (8.9GB)
    qwen2.5:0.5b            Compact model (394MB)
    qwen2.5:1.5b            Small model (934MB)
    qwen2.5:3b              Medium model (1.9GB)
    qwen2.5:7b              Large model (4.4GB)

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

# Function to print error and exit
die() {
    print_colored "$RED" "ERROR: $1" >&2
    exit 1
}

# Function to check dependencies
check_dependencies() {
    local missing_deps=()
    
    command -v curl >/dev/null 2>&1 || missing_deps+=("curl")
    command -v jq >/dev/null 2>&1 || missing_deps+=("jq")
    
    if [[ ${#missing_deps[@]} -gt 0 ]]; then
        die "Missing required dependencies: ${missing_deps[*]}. Please install them first."
    fi
}

# Function to check API health
check_api_health() {
    print_verbose "Checking API health at $API_URL"
    
    local response
    if ! response=$(curl -s --max-time 5 "$API_URL/health" 2>/dev/null); then
        die "Cannot connect to API at $API_URL. Is the server running?"
    fi
    
    local status
    if ! status=$(echo "$response" | jq -r '.status' 2>/dev/null); then
        die "Invalid response from API health endpoint"
    fi
    
    if [[ "$status" != "ok" ]]; then
        die "API is not healthy. Status: $status"
    fi
    
    print_verbose "API health check passed"
}

# Function to list available models
list_models() {
    print_verbose "Fetching available models from API"
    
    local response
    if ! response=$(curl -s "$API_URL/api/v1/models" 2>/dev/null); then
        die "Failed to fetch models from API"
    fi
    
    local models
    if ! models=$(echo "$response" | jq -r '.models[]? // empty' 2>/dev/null); then
        print_colored "$YELLOW" "No models currently available"
        return 0
    fi
    
    print_colored "$CYAN" "Available Models:"
    echo "=================="
    
    echo "$models" | jq -r '
        "Name: " + (.name // "unknown") +
        "\nSize: " + ((.size // 0) | tonumber | . / 1024 / 1024 / 1024 | floor | tostring) + "GB" +
        "\nModified: " + (.modified_at // "unknown") +
        "\n" + ("-" * 40)
    ' 2>/dev/null || echo "$models" | while read -r model; do
        echo "• $model"
    done
    
    echo
    print_colored "$GRAY" "Use '$0 MODEL_NAME' to download a specific model"
}

# Function to check if model exists
check_model_exists() {
    local model_name="$1"
    
    print_verbose "Checking if model '$model_name' is available"
    
    local response
    if ! response=$(curl -s "$API_URL/api/v1/models" 2>/dev/null); then
        die "Failed to check model availability"
    fi
    
    local model_exists
    if model_exists=$(echo "$response" | jq -r --arg name "$model_name" '.models[]? | select(.name == $name) | .name' 2>/dev/null); then
        if [[ -n "$model_exists" ]]; then
            print_colored "$GREEN" "✓ Model '$model_name' is already available"
            return 0
        fi
    fi
    
    print_colored "$YELLOW" "✗ Model '$model_name' is not currently available"
    return 1
}

# Function to pull a model with progress tracking
pull_model() {
    local model_name="$1"
    
    print_colored "$BLUE" "📥 Starting download of model: $model_name"
    print_verbose "Sending pull request to API"
    
    # Start the pull request
    local pull_response
    if ! pull_response=$(curl -s -X POST "$API_URL/api/v1/models/pull" \
        -H "Content-Type: application/json" \
        -d "{\"model\": \"$model_name\"}" 2>/dev/null); then
        die "Failed to start model download"
    fi
    
    local pull_status
    if ! pull_status=$(echo "$pull_response" | jq -r '.status' 2>/dev/null); then
        die "Failed to parse pull response"
    fi
    
    if [[ "$pull_status" != "pulling" ]]; then
        local error_msg
        error_msg=$(echo "$pull_response" | jq -r '.error // "Unknown error"' 2>/dev/null)
        die "Failed to start download: $error_msg"
    fi
    
    print_colored "$BLUE" "Download started. Monitoring progress..."
    
    # Monitor progress by checking model status
    local start_time
    start_time=$(date +%s)
    local last_status=""
    
    while true; do
        local current_time
        current_time=$(date +%s)
        local elapsed=$((current_time - start_time))
        
        if [[ $elapsed -gt $TIMEOUT ]]; then
            print_colored "$RED" "Download timeout after ${TIMEOUT} seconds"
            return 1
        fi
        
        # Check if model is now available
        if check_model_exists "$model_name" >/dev/null 2>&1; then
            print_colored "$GREEN" "✅ Model '$model_name' downloaded successfully!"
            local total_time=$((elapsed))
            print_colored "$CYAN" "Total download time: ${total_time}s"
            return 0
        fi
        
        # Show progress indicator
        local progress_indicator
        case $((elapsed % 4)) in
            0) progress_indicator="⠋" ;;
            1) progress_indicator="⠙" ;;
            2) progress_indicator="⠹" ;;
            3) progress_indicator="⠸" ;;
        esac
        
        if [[ "$QUIET" != "true" ]]; then
            printf "\r${BLUE}${progress_indicator} Downloading... (${elapsed}s elapsed)${NC}"
        fi
        
        sleep 2
    done
}

# Function to verify model integrity (basic check)
verify_model() {
    local model_name="$1"
    
    print_verbose "Verifying model integrity: $model_name"
    
    local response
    if ! response=$(curl -s "$API_URL/api/v1/models/$model_name" 2>/dev/null); then
        print_colored "$YELLOW" "⚠ Cannot verify model integrity (API endpoint unavailable)"
        return 1
    fi
    
    local model_info
    if model_info=$(echo "$response" | jq -r '.name' 2>/dev/null); then
        if [[ "$model_info" == "$model_name" ]]; then
            print_colored "$GREEN" "✓ Model verification passed"
            return 0
        fi
    fi
    
    print_colored "$RED" "✗ Model verification failed"
    return 1
}

# Function to download models from batch file
batch_download() {
    local batch_file="$1"
    
    if [[ ! -f "$batch_file" ]]; then
        die "Batch file not found: $batch_file"
    fi
    
    local total_models=0
    local successful_downloads=0
    local failed_downloads=0
    
    print_colored "$BLUE" "📦 Starting batch download from: $batch_file"
    
    while IFS= read -r line; do
        # Skip empty lines and comments
        [[ -z "$line" || "$line" =~ ^[[:space:]]*# ]] && continue
        
        local model_name
        model_name=$(echo "$line" | xargs)  # Trim whitespace
        
        print_colored "$CYAN" "\n--- Downloading model: $model_name ---"
        total_models=$((total_models + 1))
        
        # Check if model already exists
        if check_model_exists "$model_name" >/dev/null 2>&1; then
            print_colored "$YELLOW" "Model '$model_name' already exists, skipping"
            successful_downloads=$((successful_downloads + 1))
            continue
        fi
        
        # Download the model
        if pull_model "$model_name"; then
            if verify_model "$model_name"; then
                successful_downloads=$((successful_downloads + 1))
            else
                print_colored "$YELLOW" "Model downloaded but verification failed"
                successful_downloads=$((successful_downloads + 1))
            fi
        else
            print_colored "$RED" "Failed to download: $model_name"
            failed_downloads=$((failed_downloads + 1))
        fi
        
    done < "$batch_file"
    
    echo
    print_colored "$CYAN" "📊 Batch Download Summary:"
    print_colored "$CYAN" "========================="
    echo "Total models: $total_models"
    print_colored "$GREEN" "Successful: $successful_downloads"
    
    if [[ $failed_downloads -gt 0 ]]; then
        print_colored "$RED" "Failed: $failed_downloads"
    fi
    
    local success_rate
    if [[ $total_models -gt 0 ]]; then
        success_rate=$((successful_downloads * 100 / total_models))
        print_colored "$CYAN" "Success rate: ${success_rate}%"
    fi
}

# Function to suggest similar models
suggest_models() {
    local query="$1"
    
    print_colored "$CYAN" "🔍 Model suggestions for '$query':"
    
    # Common model patterns
    local suggestions=()
    
    case "$query" in
        *llama*|*"3"*)
            suggestions+=("llama3.2:1b" "llama3.2:3b" "llama3.2:7b")
            ;;
        *deepseek*|*reasoning*)
            suggestions+=("deepseek-r1:1.5b" "deepseek-r1:8b" "deepseek-r1:14b")
            ;;
        *qwen*)
            suggestions+=("qwen2.5:0.5b" "qwen2.5:1.5b" "qwen2.5:3b" "qwen2.5:7b")
            ;;
        *small*|*light*)
            suggestions+=("llama3.2:1b" "qwen2.5:0.5b" "deepseek-r1:1.5b")
            ;;
        *large*|*big*)
            suggestions+=("llama3.2:7b" "deepseek-r1:8b" "qwen2.5:7b")
            ;;
        *)
            suggestions+=("llama3.2:3b" "deepseek-r1:1.5b" "qwen2.5:1.5b")
            ;;
    esac
    
    for model in "${suggestions[@]}"; do
        echo "  • $model"
    done
    
    echo
    print_colored "$GRAY" "Use '$0 --list' to see all available models"
}

# Parse command line arguments
while [[ $# -gt 0 ]]; do
    case $1 in
        -u|--url)
            API_URL="$2"
            shift 2
            ;;
        -l|--list)
            LIST_ONLY=true
            shift
            ;;
        -c|--check)
            CHECK_ONLY=true
            MODEL_TO_CHECK="$2"
            shift 2
            ;;
        -b|--batch)
            BATCH_FILE="$2"
            shift 2
            ;;
        -t|--timeout)
            TIMEOUT="$2"
            shift 2
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
        -*)
            die "Unknown option: $1. Use -h for help."
            ;;
        *)
            MODEL_NAME="$1"
            shift
            ;;
    esac
done

# Main execution
main() {
    print_verbose "Starting HexaRAG Model Download Utility"
    
    # Check dependencies
    check_dependencies
    
    # Check API health
    check_api_health
    
    # Handle list mode
    if [[ "$LIST_ONLY" == "true" ]]; then
        list_models
        exit 0
    fi
    
    # Handle check mode
    if [[ "$CHECK_ONLY" == "true" ]]; then
        if [[ -z "$MODEL_TO_CHECK" ]]; then
            die "Model name required for check mode. Use -c MODEL_NAME"
        fi
        
        if check_model_exists "$MODEL_TO_CHECK"; then
            exit 0
        else
            suggest_models "$MODEL_TO_CHECK"
            exit 1
        fi
    fi
    
    # Handle batch mode
    if [[ -n "$BATCH_FILE" ]]; then
        batch_download "$BATCH_FILE"
        exit 0
    fi
    
    # Single model download
    if [[ -z "$MODEL_NAME" ]]; then
        die "Model name required. Use -h for help or -l to list available models."
    fi
    
    # Check if model already exists
    if check_model_exists "$MODEL_NAME"; then
        print_colored "$YELLOW" "Model already available. Use -v for verification."
        if [[ "$VERBOSE" == "true" ]]; then
            verify_model "$MODEL_NAME"
        fi
        exit 0
    fi
    
    # Download the model
    if pull_model "$MODEL_NAME"; then
        verify_model "$MODEL_NAME"
        print_colored "$GREEN" "🎉 Model '$MODEL_NAME' is ready to use!"
    else
        print_colored "$RED" "Failed to download model: $MODEL_NAME"
        suggest_models "$MODEL_NAME"
        exit 1
    fi
}

# Run main function
main "$@"