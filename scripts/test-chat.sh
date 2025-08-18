#!/bin/bash

# HexaRAG Chat Testing Tool
# Interactive CLI tool for testing conversations with the HexaRAG API

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
MODEL="${HEXARAG_MODEL:-deepseek-r1:8b}"
EXTENDED_KNOWLEDGE=false
QUIET=false
VERBOSE=false
SAVE_HISTORY=false
HISTORY_FILE=""
LOAD_FILE=""

# Function to show usage
show_help() {
    cat << EOF
HexaRAG Chat Testing Tool

Usage: $0 [OPTIONS]

OPTIONS:
    -u, --url URL           API base URL (default: $API_URL)
    -m, --model MODEL       Model to use (default: $MODEL)
    -e, --extended          Use extended knowledge
    -q, --quiet             Quiet mode (minimal output)
    -v, --verbose           Verbose mode (detailed output)
    -s, --save FILE         Save conversation history to file
    -l, --load FILE         Load messages from file
    -h, --help              Show this help message

ENVIRONMENT VARIABLES:
    HEXARAG_API_URL         Default API URL
    HEXARAG_MODEL           Default model name

EXAMPLES:
    # Basic interactive chat
    $0

    # Use different model and API
    $0 -u http://localhost:8080 -m llama3.2:3b

    # Save conversation history
    $0 -s conversation.json

    # Load test messages from file
    $0 -l test-messages.txt

    # Quiet mode for scripting
    $0 -q -l automated-test.txt

INTERACTIVE COMMANDS:
    exit, quit, q           Exit the chat
    help, h                 Show help
    models                  List available models
    switch MODEL            Switch to different model
    clear                   Clear conversation history
    save FILE               Save current conversation
    extended on/off         Toggle extended knowledge
    info                    Show conversation info

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

# Function to check if dependencies are available
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

# Function to create a new conversation
create_conversation() {
    local title="$1"
    print_verbose "Creating new conversation: $title"
    
    local payload
    payload=$(jq -n --arg title "$title" '{title: $title, system_prompt_id: "default"}')
    
    local response
    if ! response=$(curl -s -X POST "$API_URL/api/v1/conversations" \
        -H "Content-Type: application/json" \
        -d "$payload" 2>/dev/null); then
        die "Failed to create conversation"
    fi
    
    local conv_id
    if ! conv_id=$(echo "$response" | jq -r '.id' 2>/dev/null); then
        die "Failed to parse conversation ID from response"
    fi
    
    if [[ "$conv_id" == "null" || -z "$conv_id" ]]; then
        die "Invalid conversation ID received"
    fi
    
    print_verbose "Created conversation with ID: $conv_id"
    echo "$conv_id"
}

# Function to send a message
send_message() {
    local conv_id="$1"
    local message="$2"
    
    print_verbose "Sending message to conversation $conv_id"
    
    local payload
    payload=$(jq -n \
        --arg content "$message" \
        --argjson extended "$EXTENDED_KNOWLEDGE" \
        '{content: $content, use_extended_knowledge: $extended}')
    
    local response
    if ! response=$(curl -s -X POST "$API_URL/api/v1/conversations/$conv_id/messages" \
        -H "Content-Type: application/json" \
        -d "$payload" 2>/dev/null); then
        die "Failed to send message"
    fi
    
    local status
    if ! status=$(echo "$response" | jq -r '.status' 2>/dev/null); then
        print_colored "$RED" "Failed to parse response status"
        return 1
    fi
    
    print_verbose "Message sent, status: $status"
    return 0
}

# Function to get conversation messages
get_messages() {
    local conv_id="$1"
    
    local response
    if ! response=$(curl -s "$API_URL/api/v1/conversations/$conv_id/messages" 2>/dev/null); then
        die "Failed to get messages"
    fi
    
    echo "$response"
}

# Function to list available models
list_models() {
    print_verbose "Fetching available models"
    
    local response
    if ! response=$(curl -s "$API_URL/api/v1/models" 2>/dev/null); then
        print_colored "$RED" "Failed to fetch models"
        return 1
    fi
    
    local models
    if ! models=$(echo "$response" | jq -r '.models[]?.name // empty' 2>/dev/null); then
        print_colored "$RED" "Failed to parse models"
        return 1
    fi
    
    if [[ -z "$models" ]]; then
        print_colored "$YELLOW" "No models available"
        return 1
    fi
    
    print_colored "$CYAN" "Available models:"
    echo "$models" | while read -r model; do
        if [[ "$model" == "$MODEL" ]]; then
            print_colored "$GREEN" "  • $model (current)"
        else
            echo "  • $model"
        fi
    done
}

# Function to switch model
switch_model() {
    local new_model="$1"
    local conv_id="$2"
    
    print_verbose "Switching to model: $new_model"
    
    local payload
    payload=$(jq -n \
        --arg model "$new_model" \
        --arg conv_id "$conv_id" \
        '{model: $model, conversation_id: $conv_id}')
    
    local response
    if ! response=$(curl -s -X PUT "$API_URL/api/v1/models/current" \
        -H "Content-Type: application/json" \
        -d "$payload" 2>/dev/null); then
        print_colored "$RED" "Failed to switch model"
        return 1
    fi
    
    local status
    if ! status=$(echo "$response" | jq -r '.status' 2>/dev/null); then
        print_colored "$RED" "Failed to parse switch response"
        return 1
    fi
    
    if [[ "$status" == "switched" ]]; then
        MODEL="$new_model"
        print_colored "$GREEN" "Switched to model: $new_model"
        return 0
    else
        print_colored "$RED" "Failed to switch model: $status"
        return 1
    fi
}

# Function to save conversation history
save_conversation() {
    local conv_id="$1"
    local filename="$2"
    
    print_verbose "Saving conversation to: $filename"
    
    local messages
    if ! messages=$(get_messages "$conv_id"); then
        print_colored "$RED" "Failed to get messages for saving"
        return 1
    fi
    
    echo "$messages" > "$filename"
    print_colored "$GREEN" "Conversation saved to: $filename"
}

# Function to load messages from file
load_messages_from_file() {
    local filename="$1"
    local conv_id="$2"
    
    if [[ ! -f "$filename" ]]; then
        die "File not found: $filename"
    fi
    
    print_colored "$BLUE" "Loading messages from: $filename"
    
    while IFS= read -r line; do
        [[ -z "$line" || "$line" =~ ^[[:space:]]*# ]] && continue
        
        print_colored "$PURPLE" "You: $line"
        if send_message "$conv_id" "$line"; then
            sleep 2  # Give server time to process
        else
            print_colored "$RED" "Failed to send message: $line"
        fi
    done < "$filename"
}

# Function to show conversation info
show_conversation_info() {
    local conv_id="$1"
    
    print_colored "$CYAN" "Conversation Information:"
    echo "  ID: $conv_id"
    echo "  Model: $MODEL"
    echo "  Extended Knowledge: $EXTENDED_KNOWLEDGE"
    echo "  API URL: $API_URL"
}

# Function to handle interactive commands
handle_command() {
    local input="$1"
    local conv_id="$2"
    
    case "$input" in
        "exit"|"quit"|"q")
            return 1
            ;;
        "help"|"h")
            cat << EOF

Interactive Commands:
  exit, quit, q           Exit the chat
  help, h                 Show this help
  models                  List available models
  switch MODEL            Switch to different model
  clear                   Clear screen
  extended on/off         Toggle extended knowledge
  info                    Show conversation info
  save FILE               Save conversation to file

EOF
            ;;
        "models")
            list_models
            ;;
        "clear")
            clear
            ;;
        "info")
            show_conversation_info "$conv_id"
            ;;
        extended\ on)
            EXTENDED_KNOWLEDGE=true
            print_colored "$GREEN" "Extended knowledge enabled"
            ;;
        extended\ off)
            EXTENDED_KNOWLEDGE=false
            print_colored "$GREEN" "Extended knowledge disabled"
            ;;
        switch\ *)
            local new_model="${input#switch }"
            switch_model "$new_model" "$conv_id"
            ;;
        save\ *)
            local filename="${input#save }"
            save_conversation "$conv_id" "$filename"
            ;;
        *)
            return 0  # Not a command, treat as regular message
            ;;
    esac
    
    return 2  # Command handled, don't send as message
}

# Function to run interactive chat
run_interactive_chat() {
    local conv_id="$1"
    
    print_colored "$GREEN" "🚀 HexaRAG Chat Session Started"
    print_colored "$BLUE" "Conversation ID: $conv_id"
    print_colored "$BLUE" "Model: $MODEL"
    print_colored "$GRAY" "Type 'help' for commands, 'exit' to quit"
    echo
    
    while true; do
        echo -n "You: "
        read -r input
        
        [[ -z "$input" ]] && continue
        
        # Handle special commands
        if handle_command "$input" "$conv_id"; then
            case $? in
                1) break ;;  # Exit command
                2) continue ;;  # Command handled
            esac
        fi
        
        # Send regular message
        if send_message "$conv_id" "$input"; then
            print_colored "$CYAN" "Assistant: Message sent successfully (check WebSocket for response)"
        else
            print_colored "$RED" "Failed to send message"
        fi
        echo
    done
    
    # Save history if requested
    if [[ "$SAVE_HISTORY" == "true" && -n "$HISTORY_FILE" ]]; then
        save_conversation "$conv_id" "$HISTORY_FILE"
    fi
    
    print_colored "$GREEN" "Chat session ended"
}

# Parse command line arguments
while [[ $# -gt 0 ]]; do
    case $1 in
        -u|--url)
            API_URL="$2"
            shift 2
            ;;
        -m|--model)
            MODEL="$2"
            shift 2
            ;;
        -e|--extended)
            EXTENDED_KNOWLEDGE=true
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
        -s|--save)
            SAVE_HISTORY=true
            HISTORY_FILE="$2"
            shift 2
            ;;
        -l|--load)
            LOAD_FILE="$2"
            shift 2
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
    print_verbose "Starting HexaRAG Chat Testing Tool"
    
    # Check dependencies
    check_dependencies
    
    # Check API health
    check_api_health
    
    # Create conversation
    local conv_id
    conv_id=$(create_conversation "CLI Test Chat - $(date)")
    
    # Load messages from file if specified
    if [[ -n "$LOAD_FILE" ]]; then
        load_messages_from_file "$LOAD_FILE" "$conv_id"
    fi
    
    # Run interactive chat (unless in quiet mode with load file)
    if [[ "$QUIET" != "true" || -z "$LOAD_FILE" ]]; then
        run_interactive_chat "$conv_id"
    fi
}

# Run main function
main "$@"