#!/bin/bash

# HexaRAG System Health Check
# Comprehensive health verification for all system components

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
OLLAMA_URL="${OLLAMA_URL:-http://localhost:11434}"
NATS_URL="${NATS_URL:-http://localhost:8222}"
DB_CONTAINER="hexarag"
TIMEOUT=10
QUIET=false
VERBOSE=false
JSON_OUTPUT=false
CONTINUOUS=false
INTERVAL=30
DETAILED=false

# Health check results (using arrays instead of associative arrays for compatibility)
HEALTH_STATUS=()
HEALTH_DETAILS=()
OVERALL_HEALTHY=true

# Function to show usage
show_help() {
    cat << EOF
HexaRAG System Health Check

Usage: $0 [OPTIONS]

OPTIONS:
    -u, --api-url URL       HexaRAG API URL (default: $API_URL)
    -o, --ollama-url URL    Ollama API URL (default: $OLLAMA_URL)
    -n, --nats-url URL      NATS monitoring URL (default: $NATS_URL)
    -t, --timeout SECONDS   Request timeout (default: $TIMEOUT)
    -c, --continuous        Run continuously (default interval: ${INTERVAL}s)
    -i, --interval SECONDS  Continuous mode interval (default: $INTERVAL)
    -d, --detailed          Show detailed component information
    -j, --json              Output results in JSON format
    -q, --quiet             Quiet mode (errors only)
    -v, --verbose           Verbose mode (detailed output)
    -h, --help              Show this help message

CHECKS PERFORMED:
    🔗 API Server           HTTP health endpoint and response time
    🤖 Ollama Service       Model API and available models
    📡 NATS Messaging       Connection and statistics
    🗃️  Database            SQLite file and table accessibility
    🐳 Docker Containers    Container status and health
    💾 System Resources     Memory, disk, and CPU usage
    🌐 Network              DNS resolution and connectivity

EXAMPLES:
    # Basic health check
    $0

    # Detailed health check with verbose output
    $0 --detailed --verbose

    # Continuous monitoring every 60 seconds
    $0 --continuous --interval 60

    # JSON output for monitoring integration
    $0 --json

    # Quick check with custom timeout
    $0 --timeout 5 --quiet

EXIT CODES:
    0   All services healthy
    1   One or more services unhealthy
    2   Critical system error

EOF
}

# Function to print colored output
print_colored() {
    local color=$1
    local message=$2
    if [[ "$QUIET" != "true" && "$JSON_OUTPUT" != "true" ]]; then
        echo -e "${color}${message}${NC}"
    fi
}

# Function to print verbose output
print_verbose() {
    if [[ "$VERBOSE" == "true" && "$JSON_OUTPUT" != "true" ]]; then
        print_colored "$GRAY" "[VERBOSE] $1"
    fi
}

# Function to record health status
record_status() {
    local component="$1"
    local status="$2"
    local details="$3"
    
    # Store status in simple variables for compatibility
    case "$component" in
        "api") API_STATUS="$status"; API_DETAILS="$details" ;;
        "ollama") OLLAMA_STATUS="$status"; OLLAMA_DETAILS="$details" ;;
        "nats") NATS_STATUS="$status"; NATS_DETAILS="$details" ;;
        "database") DATABASE_STATUS="$status"; DATABASE_DETAILS="$details" ;;
        "docker") DOCKER_STATUS="$status"; DOCKER_DETAILS="$details" ;;
        "resources") RESOURCES_STATUS="$status"; RESOURCES_DETAILS="$details" ;;
        "network") NETWORK_STATUS="$status"; NETWORK_DETAILS="$details" ;;
    esac
    
    if [[ "$status" != "healthy" ]]; then
        OVERALL_HEALTHY=false
    fi
}

# Function to check dependencies
check_dependencies() {
    local missing_deps=()
    
    command -v curl >/dev/null 2>&1 || missing_deps+=("curl")
    command -v jq >/dev/null 2>&1 || missing_deps+=("jq")
    command -v docker >/dev/null 2>&1 || missing_deps+=("docker")
    
    if [[ ${#missing_deps[@]} -gt 0 ]]; then
        print_colored "$RED" "ERROR: Missing required dependencies: ${missing_deps[*]}"
        exit 2
    fi
}

# Function to check API health
check_api_health() {
    local component="api"
    print_verbose "Checking API health at $API_URL"
    
    local start_time
    start_time=$(date +%s%3N)
    
    local response
    local http_code
    
    if response=$(curl -s --max-time "$TIMEOUT" -w "%{http_code}" "$API_URL/health" 2>/dev/null); then
        http_code="${response: -3}"
        response="${response%???}"
        
        local end_time
        end_time=$(date +%s%3N)
        local response_time=$((end_time - start_time))
        
        if [[ "$http_code" == "200" ]]; then
            local api_status
            if api_status=$(echo "$response" | jq -r '.status' 2>/dev/null); then
                if [[ "$api_status" == "ok" ]]; then
                    record_status "$component" "healthy" "Response time: ${response_time}ms"
                    print_colored "$GREEN" "✓ API Server is healthy (${response_time}ms)"
                    
                    if [[ "$DETAILED" == "true" ]]; then
                        local service_name
                        local timestamp
                        service_name=$(echo "$response" | jq -r '.service // "unknown"' 2>/dev/null)
                        timestamp=$(echo "$response" | jq -r '.timestamp // "unknown"' 2>/dev/null)
                        print_colored "$GRAY" "  Service: $service_name, Timestamp: $timestamp"
                    fi
                    return 0
                fi
            fi
        fi
    fi
    
    record_status "$component" "unhealthy" "Failed to connect or invalid response"
    print_colored "$RED" "✗ API Server is not responding"
    return 1
}

# Function to check Ollama service
check_ollama_health() {
    local component="ollama"
    print_verbose "Checking Ollama service at $OLLAMA_URL"
    
    local response
    local http_code
    
    if response=$(curl -s --max-time "$TIMEOUT" -w "%{http_code}" "$OLLAMA_URL/api/tags" 2>/dev/null); then
        http_code="${response: -3}"
        response="${response%???}"
        
        if [[ "$http_code" == "200" ]]; then
            local model_count
            if model_count=$(echo "$response" | jq -r '.models | length' 2>/dev/null); then
                record_status "$component" "healthy" "Available models: $model_count"
                print_colored "$GREEN" "✓ Ollama Service is healthy ($model_count models available)"
                
                if [[ "$DETAILED" == "true" ]]; then
                    local models
                    if models=$(echo "$response" | jq -r '.models[]?.name // empty' 2>/dev/null); then
                        print_colored "$GRAY" "  Models:"
                        echo "$models" | while read -r model; do
                            [[ -n "$model" ]] && print_colored "$GRAY" "    • $model"
                        done
                    fi
                fi
                return 0
            fi
        fi
    fi
    
    record_status "$component" "unhealthy" "Service not responding or no models available"
    print_colored "$RED" "✗ Ollama Service is not responding"
    return 1
}

# Function to check NATS messaging
check_nats_health() {
    local component="nats"
    print_verbose "Checking NATS service at $NATS_URL"
    
    local response
    local http_code
    
    if response=$(curl -s --max-time "$TIMEOUT" -w "%{http_code}" "$NATS_URL/varz" 2>/dev/null); then
        http_code="${response: -3}"
        response="${response%???}"
        
        if [[ "$http_code" == "200" ]]; then
            local connections
            local messages
            if connections=$(echo "$response" | jq -r '.connections // 0' 2>/dev/null) && \
               messages=$(echo "$response" | jq -r '.in_msgs // 0' 2>/dev/null); then
                
                record_status "$component" "healthy" "Connections: $connections, Messages: $messages"
                print_colored "$GREEN" "✓ NATS Messaging is healthy"
                
                if [[ "$DETAILED" == "true" ]]; then
                    local uptime
                    local version
                    uptime=$(echo "$response" | jq -r '.uptime // "unknown"' 2>/dev/null)
                    version=$(echo "$response" | jq -r '.version // "unknown"' 2>/dev/null)
                    print_colored "$GRAY" "  Connections: $connections, Messages: $messages"
                    print_colored "$GRAY" "  Version: $version, Uptime: $uptime"
                fi
                return 0
            fi
        fi
    fi
    
    record_status "$component" "unhealthy" "Service not responding"
    print_colored "$RED" "✗ NATS Messaging is not responding"
    return 1
}

# Function to check database
check_database_health() {
    local component="database"
    print_verbose "Checking database in container: $DB_CONTAINER"
    
    # Check if container exists and is running
    if ! docker ps --format "table {{.Names}}" | grep -q "^${DB_CONTAINER}$"; then
        record_status "$component" "unhealthy" "Container not running"
        print_colored "$RED" "✗ Database container '$DB_CONTAINER' is not running"
        return 1
    fi
    
    # Check if database file exists
    if ! docker exec "$DB_CONTAINER" test -f /data/hexarag.db 2>/dev/null; then
        record_status "$component" "unhealthy" "Database file not found"
        print_colored "$RED" "✗ Database file not found"
        return 1
    fi
    
    # Check if database is accessible
    local table_count
    if table_count=$(docker exec "$DB_CONTAINER" sqlite3 /data/hexarag.db ".tables" 2>/dev/null | wc -w); then
        record_status "$component" "healthy" "Tables: $table_count"
        print_colored "$GREEN" "✓ Database is healthy ($table_count tables)"
        
        if [[ "$DETAILED" == "true" ]]; then
            local db_size
            if db_size=$(docker exec "$DB_CONTAINER" stat -c%s /data/hexarag.db 2>/dev/null); then
                local size_mb=$((db_size / 1024 / 1024))
                print_colored "$GRAY" "  Size: ${size_mb}MB, Tables: $table_count"
            fi
        fi
        return 0
    fi
    
    record_status "$component" "unhealthy" "Database not accessible"
    print_colored "$RED" "✗ Database is not accessible"
    return 1
}

# Function to check Docker containers
check_docker_health() {
    local component="docker"
    print_verbose "Checking Docker containers"
    
    # Check if Docker is running
    if ! docker info >/dev/null 2>&1; then
        record_status "$component" "unhealthy" "Docker daemon not running"
        print_colored "$RED" "✗ Docker daemon is not running"
        return 1
    fi
    
    # Get container status
    local containers
    containers=$(docker ps -a --format "table {{.Names}}\t{{.Status}}" | tail -n +2)
    
    if [[ -z "$containers" ]]; then
        record_status "$component" "warning" "No containers found"
        print_colored "$YELLOW" "⚠ No Docker containers found"
        return 1
    fi
    
    local running_count=0
    local total_count=0
    
    while IFS=$'\t' read -r name status; do
        total_count=$((total_count + 1))
        if [[ "$status" =~ ^Up ]]; then
            running_count=$((running_count + 1))
        fi
        
        if [[ "$DETAILED" == "true" ]]; then
            if [[ "$status" =~ ^Up ]]; then
                print_colored "$GRAY" "  ✓ $name: $status"
            else
                print_colored "$GRAY" "  ✗ $name: $status"
            fi
        fi
    done <<< "$containers"
    
    if [[ $running_count -eq $total_count ]]; then
        record_status "$component" "healthy" "All containers running ($running_count/$total_count)"
        print_colored "$GREEN" "✓ Docker containers are healthy ($running_count/$total_count running)"
        return 0
    else
        record_status "$component" "warning" "Some containers not running ($running_count/$total_count)"
        print_colored "$YELLOW" "⚠ Some Docker containers not running ($running_count/$total_count)"
        return 1
    fi
}

# Function to check system resources
check_system_resources() {
    local component="resources"
    print_verbose "Checking system resources"
    
    # Check disk space
    local disk_usage
    if disk_usage=$(df -h . | tail -1 | awk '{print $5}' | sed 's/%//'); then
        local disk_warning=false
        if [[ $disk_usage -gt 90 ]]; then
            disk_warning=true
        fi
        
        # Check memory usage (if available)
        local mem_info=""
        if command -v free >/dev/null 2>&1; then
            local mem_usage
            mem_usage=$(free | grep Mem | awk '{printf "%.1f", $3/$2 * 100.0}')
            mem_info=", Memory: ${mem_usage}%"
        elif [[ -f /proc/meminfo ]]; then
            local mem_total
            local mem_available
            mem_total=$(grep MemTotal /proc/meminfo | awk '{print $2}')
            mem_available=$(grep MemAvailable /proc/meminfo | awk '{print $2}')
            if [[ -n "$mem_total" && -n "$mem_available" ]]; then
                local mem_usage
                mem_usage=$(awk "BEGIN {printf \"%.1f\", (1 - $mem_available/$mem_total) * 100}")
                mem_info=", Memory: ${mem_usage}%"
            fi
        fi
        
        if [[ "$disk_warning" == "true" ]]; then
            record_status "$component" "warning" "Disk usage: ${disk_usage}%${mem_info}"
            print_colored "$YELLOW" "⚠ System resources warning (Disk: ${disk_usage}%${mem_info})"
        else
            record_status "$component" "healthy" "Disk usage: ${disk_usage}%${mem_info}"
            print_colored "$GREEN" "✓ System resources are healthy (Disk: ${disk_usage}%${mem_info})"
        fi
        
        return 0
    fi
    
    record_status "$component" "unknown" "Cannot determine resource usage"
    print_colored "$GRAY" "? System resource usage unknown"
    return 1
}

# Function to check network connectivity
check_network_health() {
    local component="network"
    print_verbose "Checking network connectivity"
    
    # Test DNS resolution
    if ! nslookup google.com >/dev/null 2>&1 && ! dig google.com >/dev/null 2>&1; then
        record_status "$component" "unhealthy" "DNS resolution failed"
        print_colored "$RED" "✗ Network connectivity issues (DNS resolution failed)"
        return 1
    fi
    
    # Test outbound connectivity (if curl is available)
    if command -v curl >/dev/null 2>&1; then
        if curl -s --max-time 5 --head http://google.com >/dev/null 2>&1; then
            record_status "$component" "healthy" "DNS and HTTP connectivity working"
            print_colored "$GREEN" "✓ Network connectivity is healthy"
            return 0
        fi
    fi
    
    record_status "$component" "warning" "DNS working, HTTP connectivity unknown"
    print_colored "$YELLOW" "⚠ Network partially healthy (DNS working, HTTP unknown)"
    return 1
}

# Function to output JSON results
output_json() {
    local timestamp
    timestamp=$(date -u +"%Y-%m-%dT%H:%M:%SZ")
    
    echo "{"
    echo "  \"timestamp\": \"$timestamp\","
    echo "  \"overall_healthy\": $([[ "$OVERALL_HEALTHY" == "true" ]] && echo "true" || echo "false"),"
    echo "  \"components\": {"
    
    local components=("api" "ollama" "nats" "database" "docker" "resources" "network")
    local first=true
    
    for component in "${components[@]}"; do
        local status=""
        local details=""
        
        case "$component" in
            "api") status="$API_STATUS"; details="$API_DETAILS" ;;
            "ollama") status="$OLLAMA_STATUS"; details="$OLLAMA_DETAILS" ;;
            "nats") status="$NATS_STATUS"; details="$NATS_DETAILS" ;;
            "database") status="$DATABASE_STATUS"; details="$DATABASE_DETAILS" ;;
            "docker") status="$DOCKER_STATUS"; details="$DOCKER_DETAILS" ;;
            "resources") status="$RESOURCES_STATUS"; details="$RESOURCES_DETAILS" ;;
            "network") status="$NETWORK_STATUS"; details="$NETWORK_DETAILS" ;;
        esac
        
        if [[ -n "$status" ]]; then
            if [[ "$first" == "false" ]]; then
                echo ","
            fi
            first=false
            
            echo -n "    \"$component\": {"
            echo -n "\"status\": \"$status\""
            if [[ -n "$details" ]]; then
                echo -n ", \"details\": \"$details\""
            fi
            echo -n "}"
        fi
    done
    
    echo ""
    echo "  }"
    echo "}"
}

# Function to run all health checks
run_health_checks() {
    if [[ "$JSON_OUTPUT" != "true" ]]; then
        print_colored "$CYAN" "🏥 HexaRAG System Health Check"
        print_colored "$CYAN" "============================="
        echo
    fi
    
    # Reset status
    OVERALL_HEALTHY=true
    HEALTH_STATUS=()
    HEALTH_DETAILS=()
    
    # Run all checks
    check_api_health || true
    check_ollama_health || true
    check_nats_health || true
    check_database_health || true
    check_docker_health || true
    check_system_resources || true
    check_network_health || true
    
    if [[ "$JSON_OUTPUT" == "true" ]]; then
        output_json
    else
        echo
        if [[ "$OVERALL_HEALTHY" == "true" ]]; then
            print_colored "$GREEN" "🎉 All systems are healthy!"
        else
            print_colored "$RED" "⚠️  Some systems need attention"
        fi
        
        print_colored "$GRAY" "Check completed at $(date)"
    fi
}

# Function to run continuous monitoring
run_continuous() {
    print_colored "$BLUE" "Starting continuous monitoring (interval: ${INTERVAL}s)"
    print_colored "$GRAY" "Press Ctrl+C to stop"
    echo
    
    while true; do
        run_health_checks
        
        if [[ "$JSON_OUTPUT" != "true" ]]; then
            echo
            print_colored "$GRAY" "Waiting ${INTERVAL} seconds..."
            echo "$(printf '%*s' 50 '' | tr ' ' '=')"
        fi
        
        sleep "$INTERVAL"
    done
}

# Parse command line arguments
while [[ $# -gt 0 ]]; do
    case $1 in
        -u|--api-url)
            API_URL="$2"
            shift 2
            ;;
        -o|--ollama-url)
            OLLAMA_URL="$2"
            shift 2
            ;;
        -n|--nats-url)
            NATS_URL="$2"
            shift 2
            ;;
        -t|--timeout)
            TIMEOUT="$2"
            shift 2
            ;;
        -c|--continuous)
            CONTINUOUS=true
            shift
            ;;
        -i|--interval)
            INTERVAL="$2"
            shift 2
            ;;
        -d|--detailed)
            DETAILED=true
            shift
            ;;
        -j|--json)
            JSON_OUTPUT=true
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
            echo "Unknown option: $1. Use -h for help." >&2
            exit 2
            ;;
    esac
done

# Main execution
main() {
    print_verbose "Starting HexaRAG System Health Check"
    
    # Check dependencies
    check_dependencies
    
    # Run health checks
    if [[ "$CONTINUOUS" == "true" ]]; then
        run_continuous
    else
        run_health_checks
    fi
    
    # Exit with appropriate code
    if [[ "$OVERALL_HEALTHY" == "true" ]]; then
        exit 0
    else
        exit 1
    fi
}

# Handle interrupts gracefully
trap 'print_colored "$BLUE" "\nHealth check interrupted"; exit 0' INT TERM

# Run main function
main "$@"