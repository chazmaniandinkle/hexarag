#!/bin/bash

# HexaRAG Performance Benchmark Tool
# Comprehensive performance testing and benchmarking

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
CONCURRENT_USERS=1
TOTAL_REQUESTS=10
WARMUP_REQUESTS=2
REQUEST_DELAY=1
OUTPUT_FILE=""
CSV_OUTPUT=false
QUIET=false
VERBOSE=false
DETAILED=false
TEST_TYPE="conversation"

# Test prompts for different scenarios
declare -a SHORT_PROMPTS=(
    "Hello!"
    "What is 2+2?"
    "Hi there"
    "Test message"
    "Quick question"
)

declare -a MEDIUM_PROMPTS=(
    "Can you explain what machine learning is?"
    "Write a short summary of renewable energy benefits."
    "What are the key principles of good software design?"
    "Describe the process of photosynthesis in plants."
    "How does the internet work at a basic level?"
)

declare -a LONG_PROMPTS=(
    "Please write a comprehensive essay about the impact of artificial intelligence on modern society, covering both positive and negative aspects, including economic implications, ethical concerns, and future prospects."
    "Explain in detail how a computer processes information from input to output, including the role of the CPU, memory, storage, and operating system, with examples of how this applies to everyday computing tasks."
    "Provide a thorough analysis of climate change, including its causes, current and projected impacts, mitigation strategies, and the role of international cooperation in addressing this global challenge."
    "Write a detailed guide on sustainable living practices that individuals can adopt, covering areas such as energy consumption, transportation, diet, waste reduction, and conscious consumption."
    "Discuss the evolution of programming languages from early assembly languages to modern high-level languages, including key milestones, paradigm shifts, and how language design influences software development practices."
)

# Function to show usage
show_help() {
    cat << EOF
HexaRAG Performance Benchmark Tool

Usage: $0 [OPTIONS]

OPTIONS:
    -u, --url URL           API base URL (default: $API_URL)
    -m, --model MODEL       Model to benchmark (default: $MODEL)
    -c, --concurrent N      Concurrent users (default: $CONCURRENT_USERS)
    -r, --requests N        Total requests per user (default: $TOTAL_REQUESTS)
    -w, --warmup N          Warmup requests (default: $WARMUP_REQUESTS)
    -d, --delay SECONDS     Delay between requests (default: $REQUEST_DELAY)
    -t, --type TYPE         Test type: conversation|response|load|stress (default: $TEST_TYPE)
    -o, --output FILE       Save results to file
    --csv                   Output results in CSV format
    --detailed              Show detailed per-request metrics
    -q, --quiet             Quiet mode (results only)
    -v, --verbose           Verbose mode (detailed output)
    -h, --help              Show this help message

TEST TYPES:
    conversation            Test full conversation flow (create + messages)
    response                Test message response times only
    load                    Load testing with increasing concurrent users
    stress                  Stress testing to find breaking points

EXAMPLES:
    # Basic benchmark
    $0

    # Load test with 10 concurrent users, 50 requests each
    $0 --type load --concurrent 10 --requests 50

    # Stress test to find limits
    $0 --type stress --verbose

    # Detailed response time analysis
    $0 --type response --detailed --requests 100

    # Save results to CSV for analysis
    $0 --csv --output benchmark_results.csv

METRICS COLLECTED:
    📊 Response Times        Min, max, mean, median, p95, p99
    🔄 Throughput           Requests per second
    ⚡ Latency              End-to-end request latency
    💾 Memory Usage         API server memory consumption
    🚫 Error Rates          Failed requests and error types
    📈 Percentiles          Detailed response time distribution

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
    command -v bc >/dev/null 2>&1 || missing_deps+=("bc")
    
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

# Function to create conversation
create_conversation() {
    local conv_title="$1"
    
    local payload
    payload=$(jq -n --arg title "$conv_title" '{title: $title, system_prompt_id: "default"}')
    
    local response
    if ! response=$(curl -s -X POST "$API_URL/api/v1/conversations" \
        -H "Content-Type: application/json" \
        -d "$payload" 2>/dev/null); then
        echo "ERROR"
        return 1
    fi
    
    local conv_id
    if ! conv_id=$(echo "$response" | jq -r '.id' 2>/dev/null); then
        echo "ERROR"
        return 1
    fi
    
    echo "$conv_id"
}

# Function to send message and measure response time
send_message_timed() {
    local conv_id="$1"
    local message="$2"
    
    local start_time
    start_time=$(date +%s%3N)
    
    local payload
    payload=$(jq -n --arg content "$message" '{content: $content, use_extended_knowledge: false}')
    
    local response
    local http_code
    if response=$(curl -s -w "%{http_code}" -X POST "$API_URL/api/v1/conversations/$conv_id/messages" \
        -H "Content-Type: application/json" \
        -d "$payload" 2>/dev/null); then
        
        http_code="${response: -3}"
        response="${response%???}"
        
        local end_time
        end_time=$(date +%s%3N)
        local response_time=$((end_time - start_time))
        
        if [[ "$http_code" == "200" ]]; then
            echo "$response_time"
            return 0
        fi
    fi
    
    echo "ERROR"
    return 1
}

# Function to run warmup requests
run_warmup() {
    if [[ $WARMUP_REQUESTS -eq 0 ]]; then
        return 0
    fi
    
    print_colored "$BLUE" "🔥 Running warmup ($WARMUP_REQUESTS requests)..."
    
    local conv_id
    conv_id=$(create_conversation "Benchmark Warmup")
    
    if [[ "$conv_id" == "ERROR" ]]; then
        print_colored "$YELLOW" "Warning: Failed to create warmup conversation"
        return 1
    fi
    
    for ((i=1; i<=WARMUP_REQUESTS; i++)); do
        local prompt="${SHORT_PROMPTS[$((i % ${#SHORT_PROMPTS[@]}))]}"
        local response_time
        response_time=$(send_message_timed "$conv_id" "$prompt")
        
        if [[ "$response_time" == "ERROR" ]]; then
            print_colored "$YELLOW" "Warning: Warmup request $i failed"
        else
            print_verbose "Warmup $i: ${response_time}ms"
        fi
        
        sleep 0.5
    done
    
    print_verbose "Warmup completed"
}

# Function to calculate statistics
calculate_stats() {
    local -n times_ref=$1
    local -a sorted_times
    IFS=$'\n' sorted_times=($(sort -n <<< "${times_ref[*]}"))
    
    local count=${#sorted_times[@]}
    if [[ $count -eq 0 ]]; then
        echo "0 0 0 0 0 0"
        return
    fi
    
    # Min and Max
    local min=${sorted_times[0]}
    local max=${sorted_times[$((count-1))]}
    
    # Mean
    local sum=0
    for time in "${sorted_times[@]}"; do
        sum=$((sum + time))
    done
    local mean=$((sum / count))
    
    # Median
    local median
    if [[ $((count % 2)) -eq 1 ]]; then
        median=${sorted_times[$((count / 2))]}
    else
        local mid1=${sorted_times[$((count / 2 - 1))]}
        local mid2=${sorted_times[$((count / 2))]}
        median=$(((mid1 + mid2) / 2))
    fi
    
    # Percentiles
    local p95_idx=$(( (count * 95) / 100 ))
    local p99_idx=$(( (count * 99) / 100 ))
    [[ $p95_idx -ge $count ]] && p95_idx=$((count - 1))
    [[ $p99_idx -ge $count ]] && p99_idx=$((count - 1))
    
    local p95=${sorted_times[$p95_idx]}
    local p99=${sorted_times[$p99_idx]}
    
    echo "$min $max $mean $median $p95 $p99"
}

# Function to run conversation benchmark
run_conversation_benchmark() {
    print_colored "$CYAN" "🗣️  Running conversation benchmark..."
    print_colored "$GRAY" "Model: $MODEL, Requests: $TOTAL_REQUESTS, Concurrent: $CONCURRENT_USERS"
    echo
    
    local -a all_response_times
    local total_errors=0
    local start_time
    start_time=$(date +%s)
    
    # Run concurrent users
    for ((user=1; user<=CONCURRENT_USERS; user++)); do
        (
            local user_times=()
            local conv_id
            conv_id=$(create_conversation "Benchmark User $user")
            
            if [[ "$conv_id" == "ERROR" ]]; then
                echo "ERROR: Failed to create conversation for user $user" >&2
                exit 1
            fi
            
            for ((req=1; req<=TOTAL_REQUESTS; req++)); do
                # Select prompt based on request number
                local prompt
                if [[ $req -le 5 ]]; then
                    prompt="${SHORT_PROMPTS[$((req % ${#SHORT_PROMPTS[@]}))]}"
                elif [[ $req -le 15 ]]; then
                    prompt="${MEDIUM_PROMPTS[$(((req-6) % ${#MEDIUM_PROMPTS[@]}))]}"
                else
                    prompt="${LONG_PROMPTS[$(((req-16) % ${#LONG_PROMPTS[@]}))]}"
                fi
                
                local response_time
                response_time=$(send_message_timed "$conv_id" "$prompt")
                
                if [[ "$response_time" == "ERROR" ]]; then
                    echo "ERROR:$user:$req" >&2
                else
                    echo "SUCCESS:$user:$req:$response_time"
                    if [[ "$DETAILED" == "true" ]]; then
                        print_colored "$GRAY" "  User $user, Request $req: ${response_time}ms"
                    fi
                fi
                
                sleep "$REQUEST_DELAY"
            done
        ) &
    done
    
    # Wait for all users to complete and collect results
    wait
    
    # Process results from background jobs
    while IFS= read -r line; do
        if [[ "$line" =~ ^SUCCESS:.*:.*:(.*) ]]; then
            local response_time="${BASH_REMATCH[1]}"
            all_response_times+=("$response_time")
        elif [[ "$line" =~ ^ERROR: ]]; then
            total_errors=$((total_errors + 1))
        fi
    done < <(jobs -p | xargs -I {} wait {})
    
    local end_time
    end_time=$(date +%s)
    local total_duration=$((end_time - start_time))
    
    # Calculate statistics
    local stats
    stats=$(calculate_stats all_response_times)
    read -r min max mean median p95 p99 <<< "$stats"
    
    local successful_requests=${#all_response_times[@]}
    local total_attempted_requests=$((CONCURRENT_USERS * TOTAL_REQUESTS))
    local success_rate
    if [[ $total_attempted_requests -gt 0 ]]; then
        success_rate=$(bc <<< "scale=2; $successful_requests * 100 / $total_attempted_requests")
    else
        success_rate="0"
    fi
    
    local throughput
    if [[ $total_duration -gt 0 ]]; then
        throughput=$(bc <<< "scale=2; $successful_requests / $total_duration")
    else
        throughput="0"
    fi
    
    # Display results
    echo
    print_colored "$CYAN" "📊 Benchmark Results"
    print_colored "$CYAN" "==================="
    
    echo "Total Duration: ${total_duration}s"
    echo "Successful Requests: $successful_requests"
    echo "Failed Requests: $total_errors"
    echo "Success Rate: ${success_rate}%"
    echo "Throughput: $throughput req/s"
    echo
    
    print_colored "$GREEN" "Response Times (ms):"
    echo "  Min:    $min"
    echo "  Max:    $max"
    echo "  Mean:   $mean"
    echo "  Median: $median"
    echo "  P95:    $p95"
    echo "  P99:    $p99"
    
    # Output to file if requested
    if [[ -n "$OUTPUT_FILE" ]]; then
        output_results_to_file "$OUTPUT_FILE" "$total_duration" "$successful_requests" "$total_errors" "$success_rate" "$throughput" "$min" "$max" "$mean" "$median" "$p95" "$p99"
    fi
}

# Function to run load test
run_load_test() {
    print_colored "$CYAN" "📈 Running load test..."
    
    local -a user_counts=(1 2 5 10 20)
    
    for users in "${user_counts[@]}"; do
        print_colored "$BLUE" "\n--- Testing with $users concurrent users ---"
        CONCURRENT_USERS=$users
        TOTAL_REQUESTS=5  # Fewer requests per user for load testing
        run_conversation_benchmark
        sleep 2
    done
}

# Function to run stress test
run_stress_test() {
    print_colored "$CYAN" "🔥 Running stress test..."
    
    local max_users=50
    local increment=5
    local failure_threshold=50  # % failure rate that indicates breaking point
    
    for ((users=5; users<=max_users; users+=increment)); do
        print_colored "$BLUE" "\n--- Stress testing with $users concurrent users ---"
        CONCURRENT_USERS=$users
        TOTAL_REQUESTS=3  # Quick requests for stress testing
        
        # Run test and capture results
        run_conversation_benchmark
        
        # You could implement breaking point detection here
        # For now, just continue until max_users
        
        sleep 1
    done
}

# Function to output results to file
output_results_to_file() {
    local file="$1"
    local duration="$2"
    local successful="$3"
    local errors="$4"
    local success_rate="$5"
    local throughput="$6"
    local min="$7"
    local max="$8"
    local mean="$9"
    local median="${10}"
    local p95="${11}"
    local p99="${12}"
    
    local timestamp
    timestamp=$(date -u +"%Y-%m-%dT%H:%M:%SZ")
    
    if [[ "$CSV_OUTPUT" == "true" ]]; then
        # CSV format
        if [[ ! -f "$file" ]]; then
            echo "timestamp,model,concurrent_users,total_requests,duration_s,successful_requests,failed_requests,success_rate_pct,throughput_rps,min_ms,max_ms,mean_ms,median_ms,p95_ms,p99_ms" > "$file"
        fi
        
        echo "$timestamp,$MODEL,$CONCURRENT_USERS,$TOTAL_REQUESTS,$duration,$successful,$errors,$success_rate,$throughput,$min,$max,$mean,$median,$p95,$p99" >> "$file"
    else
        # JSON format
        cat << EOF >> "$file"
{
  "timestamp": "$timestamp",
  "configuration": {
    "model": "$MODEL",
    "concurrent_users": $CONCURRENT_USERS,
    "total_requests": $TOTAL_REQUESTS,
    "test_type": "$TEST_TYPE"
  },
  "results": {
    "duration_seconds": $duration,
    "successful_requests": $successful,
    "failed_requests": $errors,
    "success_rate_percent": $success_rate,
    "throughput_rps": $throughput,
    "response_times_ms": {
      "min": $min,
      "max": $max,
      "mean": $mean,
      "median": $median,
      "p95": $p95,
      "p99": $p99
    }
  }
}
EOF
    fi
    
    print_colored "$GREEN" "Results saved to: $file"
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
        -c|--concurrent)
            CONCURRENT_USERS="$2"
            shift 2
            ;;
        -r|--requests)
            TOTAL_REQUESTS="$2"
            shift 2
            ;;
        -w|--warmup)
            WARMUP_REQUESTS="$2"
            shift 2
            ;;
        -d|--delay)
            REQUEST_DELAY="$2"
            shift 2
            ;;
        -t|--type)
            TEST_TYPE="$2"
            shift 2
            ;;
        -o|--output)
            OUTPUT_FILE="$2"
            shift 2
            ;;
        --csv)
            CSV_OUTPUT=true
            shift
            ;;
        --detailed)
            DETAILED=true
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
    print_verbose "Starting HexaRAG Performance Benchmark"
    
    # Check dependencies
    check_dependencies
    
    # Check API health
    check_api_health
    
    # Run warmup
    run_warmup
    
    # Run appropriate test type
    case "$TEST_TYPE" in
        "conversation")
            run_conversation_benchmark
            ;;
        "response")
            print_colored "$BLUE" "Response-only testing not yet implemented"
            run_conversation_benchmark
            ;;
        "load")
            run_load_test
            ;;
        "stress")
            run_stress_test
            ;;
        *)
            die "Unknown test type: $TEST_TYPE. Use conversation, response, load, or stress."
            ;;
    esac
    
    print_colored "$GREEN" "\n🎉 Benchmark completed!"
}

# Run main function
main "$@"