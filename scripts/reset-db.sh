#!/bin/bash

# HexaRAG Database Reset Tool
# Tool for resetting the database with backup and restore capabilities

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
DOCKER_COMPOSE_FILE="docker-compose.yml"
DB_CONTAINER="hexarag"
DB_PATH="/data/hexarag.db"
BACKUP_DIR="./backups"
MIGRATIONS_DIR="./internal/adapters/storage/sqlite/migrations"
TEST_DATA_FILE="./test-data.sql"
FORCE=false
QUIET=false
VERBOSE=false
CREATE_BACKUP=true
LOAD_TEST_DATA=false
RESTORE_FILE=""

# Function to show usage
show_help() {
    cat << EOF
HexaRAG Database Reset Tool

Usage: $0 [OPTIONS]

OPTIONS:
    -f, --force             Skip confirmation prompts
    -q, --quiet             Quiet mode (minimal output)
    -v, --verbose           Verbose mode (detailed output)
    --no-backup             Don't create backup before reset
    --test-data             Load test data after reset
    --restore FILE          Restore from specific backup file
    -c, --container NAME    Docker container name (default: $DB_CONTAINER)
    -d, --db-path PATH      Database path in container (default: $DB_PATH)
    --backup-dir DIR        Backup directory (default: $BACKUP_DIR)
    -h, --help              Show this help message

EXAMPLES:
    # Basic database reset with backup
    $0

    # Force reset without confirmation
    $0 --force

    # Reset and load test data
    $0 --test-data

    # Reset without creating backup
    $0 --no-backup --force

    # Restore from specific backup
    $0 --restore ./backups/hexarag_2024-01-15_14-30-45.db

    # Quiet mode for scripts
    $0 --force --quiet

OPERATIONS:
    1. Create timestamped backup (unless --no-backup)
    2. Stop the application
    3. Clear database tables
    4. Re-run migrations
    5. Load test data (if --test-data)
    6. Restart application

BACKUP FILES:
    Backups are stored in: $BACKUP_DIR/
    Format: hexarag_YYYY-MM-DD_HH-MM-SS.db

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
    
    command -v docker >/dev/null 2>&1 || missing_deps+=("docker")
    command -v docker-compose >/dev/null 2>&1 || missing_deps+=("docker-compose")
    
    if [[ ${#missing_deps[@]} -gt 0 ]]; then
        die "Missing required dependencies: ${missing_deps[*]}. Please install them first."
    fi
}

# Function to check if Docker container exists and is running
check_container() {
    print_verbose "Checking Docker container: $DB_CONTAINER"
    
    if ! docker ps -a --format "table {{.Names}}" | grep -q "^${DB_CONTAINER}$"; then
        die "Docker container '$DB_CONTAINER' not found. Is docker-compose running?"
    fi
    
    local container_status
    container_status=$(docker inspect --format='{{.State.Status}}' "$DB_CONTAINER" 2>/dev/null)
    
    if [[ "$container_status" != "running" ]]; then
        print_colored "$YELLOW" "Container '$DB_CONTAINER' is not running (status: $container_status)"
        print_colored "$BLUE" "Starting container..."
        
        if ! docker-compose -f "$DOCKER_COMPOSE_FILE" up -d "$DB_CONTAINER"; then
            die "Failed to start container"
        fi
        
        sleep 2  # Give container time to start
    fi
    
    print_verbose "Container check passed"
}

# Function to create database backup
create_backup() {
    local backup_file="$1"
    
    print_colored "$BLUE" "📦 Creating database backup..."
    print_verbose "Backup file: $backup_file"
    
    # Create backup directory if it doesn't exist
    mkdir -p "$BACKUP_DIR"
    
    # Copy database file from container
    if ! docker cp "${DB_CONTAINER}:${DB_PATH}" "$backup_file"; then
        die "Failed to create backup. Database file may not exist."
    fi
    
    # Verify backup file was created and is not empty
    if [[ ! -f "$backup_file" ]]; then
        die "Backup file was not created"
    fi
    
    local backup_size
    backup_size=$(stat -f%z "$backup_file" 2>/dev/null || stat -c%s "$backup_file" 2>/dev/null || echo "0")
    
    if [[ "$backup_size" -eq 0 ]]; then
        die "Backup file is empty"
    fi
    
    print_colored "$GREEN" "✓ Backup created: $backup_file (${backup_size} bytes)"
}

# Function to restore database from backup
restore_database() {
    local backup_file="$1"
    
    if [[ ! -f "$backup_file" ]]; then
        die "Backup file not found: $backup_file"
    fi
    
    print_colored "$BLUE" "📥 Restoring database from backup..."
    print_verbose "Restoring from: $backup_file"
    
    # Copy backup file to container
    if ! docker cp "$backup_file" "${DB_CONTAINER}:${DB_PATH}"; then
        die "Failed to restore database from backup"
    fi
    
    # Verify restore by checking if file exists in container
    if ! docker exec "$DB_CONTAINER" test -f "$DB_PATH"; then
        die "Database file not found after restore"
    fi
    
    print_colored "$GREEN" "✓ Database restored from backup"
}

# Function to clear database tables
clear_database() {
    print_colored "$BLUE" "🗑️  Clearing database tables..."
    
    # Get list of all tables
    local tables
    if ! tables=$(docker exec "$DB_CONTAINER" sqlite3 "$DB_PATH" ".tables" 2>/dev/null); then
        die "Failed to get database tables"
    fi
    
    if [[ -z "$tables" ]]; then
        print_colored "$YELLOW" "No tables found in database"
        return 0
    fi
    
    print_verbose "Found tables: $tables"
    
    # Drop all tables except schema_migrations
    for table in $tables; do
        if [[ "$table" != "schema_migrations" ]]; then
            print_verbose "Dropping table: $table"
            if ! docker exec "$DB_CONTAINER" sqlite3 "$DB_PATH" "DROP TABLE IF EXISTS $table;" 2>/dev/null; then
                print_colored "$YELLOW" "Warning: Failed to drop table $table"
            fi
        fi
    done
    
    # Clear data from remaining tables
    if docker exec "$DB_CONTAINER" sqlite3 "$DB_PATH" "DELETE FROM schema_migrations WHERE version > 1;" 2>/dev/null; then
        print_verbose "Cleared migration history"
    fi
    
    print_colored "$GREEN" "✓ Database tables cleared"
}

# Function to run migrations
run_migrations() {
    print_colored "$BLUE" "🔄 Running database migrations..."
    
    if [[ ! -d "$MIGRATIONS_DIR" ]]; then
        print_colored "$YELLOW" "Migration directory not found: $MIGRATIONS_DIR"
        print_colored "$YELLOW" "Skipping migration step"
        return 0
    fi
    
    # Get list of migration files
    local migration_files
    migration_files=$(find "$MIGRATIONS_DIR" -name "*.sql" | sort)
    
    if [[ -z "$migration_files" ]]; then
        print_colored "$YELLOW" "No migration files found"
        return 0
    fi
    
    print_verbose "Found migration files:"
    echo "$migration_files" | while read -r file; do
        print_verbose "  • $(basename "$file")"
    done
    
    # Run each migration
    echo "$migration_files" | while read -r migration_file; do
        local migration_name
        migration_name=$(basename "$migration_file" .sql)
        
        print_verbose "Running migration: $migration_name"
        
        # Copy migration file to container and execute
        if ! docker cp "$migration_file" "${DB_CONTAINER}:/tmp/migration.sql"; then
            print_colored "$RED" "Failed to copy migration file: $migration_name"
            continue
        fi
        
        if ! docker exec "$DB_CONTAINER" sqlite3 "$DB_PATH" ".read /tmp/migration.sql"; then
            print_colored "$RED" "Failed to run migration: $migration_name"
            continue
        fi
        
        # Clean up
        docker exec "$DB_CONTAINER" rm -f /tmp/migration.sql 2>/dev/null || true
    done
    
    print_colored "$GREEN" "✓ Database migrations completed"
}

# Function to load test data
load_test_data() {
    if [[ ! -f "$TEST_DATA_FILE" ]]; then
        print_colored "$YELLOW" "Test data file not found: $TEST_DATA_FILE"
        print_colored "$YELLOW" "Skipping test data loading"
        return 0
    fi
    
    print_colored "$BLUE" "📊 Loading test data..."
    print_verbose "Test data file: $TEST_DATA_FILE"
    
    # Copy test data file to container and execute
    if ! docker cp "$TEST_DATA_FILE" "${DB_CONTAINER}:/tmp/test-data.sql"; then
        die "Failed to copy test data file"
    fi
    
    if ! docker exec "$DB_CONTAINER" sqlite3 "$DB_PATH" ".read /tmp/test-data.sql"; then
        die "Failed to load test data"
    fi
    
    # Clean up
    docker exec "$DB_CONTAINER" rm -f /tmp/test-data.sql 2>/dev/null || true
    
    print_colored "$GREEN" "✓ Test data loaded"
}

# Function to restart application
restart_application() {
    print_colored "$BLUE" "🔄 Restarting application..."
    
    if ! docker-compose -f "$DOCKER_COMPOSE_FILE" restart; then
        print_colored "$YELLOW" "Warning: Failed to restart application via docker-compose"
        print_colored "$YELLOW" "You may need to restart manually"
        return 1
    fi
    
    # Wait for services to come up
    sleep 3
    
    print_colored "$GREEN" "✓ Application restarted"
}

# Function to verify database integrity
verify_database() {
    print_colored "$BLUE" "🔍 Verifying database integrity..."
    
    # Check if database file exists
    if ! docker exec "$DB_CONTAINER" test -f "$DB_PATH"; then
        die "Database file not found after reset"
    fi
    
    # Check if database is accessible
    if ! docker exec "$DB_CONTAINER" sqlite3 "$DB_PATH" ".tables" >/dev/null 2>&1; then
        die "Database is not accessible or corrupted"
    fi
    
    # Get table count
    local table_count
    table_count=$(docker exec "$DB_CONTAINER" sqlite3 "$DB_PATH" ".tables" 2>/dev/null | wc -w)
    
    print_verbose "Database contains $table_count tables"
    
    print_colored "$GREEN" "✓ Database integrity verified"
}

# Function to list available backups
list_backups() {
    print_colored "$CYAN" "📋 Available backups in $BACKUP_DIR:"
    
    if [[ ! -d "$BACKUP_DIR" ]]; then
        print_colored "$YELLOW" "Backup directory does not exist"
        return 0
    fi
    
    local backups
    backups=$(find "$BACKUP_DIR" -name "hexarag_*.db" -type f | sort -r)
    
    if [[ -z "$backups" ]]; then
        print_colored "$YELLOW" "No backup files found"
        return 0
    fi
    
    echo "$backups" | while read -r backup; do
        local filename
        local size
        local date
        
        filename=$(basename "$backup")
        size=$(stat -f%z "$backup" 2>/dev/null || stat -c%s "$backup" 2>/dev/null || echo "0")
        date=$(stat -f%Sm -t "%Y-%m-%d %H:%M:%S" "$backup" 2>/dev/null || stat -c%y "$backup" 2>/dev/null | cut -d. -f1)
        
        printf "  • %-30s %10s bytes  %s\n" "$filename" "$size" "$date"
    done
}

# Function to confirm action
confirm_action() {
    local message="$1"
    
    if [[ "$FORCE" == "true" ]]; then
        return 0
    fi
    
    print_colored "$YELLOW" "$message"
    read -p "Are you sure? (y/N): " -r response
    
    case "$response" in
        [yY][eE][sS]|[yY])
            return 0
            ;;
        *)
            print_colored "$BLUE" "Operation cancelled"
            exit 0
            ;;
    esac
}

# Parse command line arguments
while [[ $# -gt 0 ]]; do
    case $1 in
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
        --no-backup)
            CREATE_BACKUP=false
            shift
            ;;
        --test-data)
            LOAD_TEST_DATA=true
            shift
            ;;
        --restore)
            RESTORE_FILE="$2"
            shift 2
            ;;
        -c|--container)
            DB_CONTAINER="$2"
            shift 2
            ;;
        -d|--db-path)
            DB_PATH="$2"
            shift 2
            ;;
        --backup-dir)
            BACKUP_DIR="$2"
            shift 2
            ;;
        --list-backups)
            list_backups
            exit 0
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
    print_verbose "Starting HexaRAG Database Reset Tool"
    
    # Check dependencies
    check_dependencies
    
    # Check container
    check_container
    
    # Handle restore mode
    if [[ -n "$RESTORE_FILE" ]]; then
        confirm_action "This will restore the database from backup: $RESTORE_FILE"
        restore_database "$RESTORE_FILE"
        restart_application
        verify_database
        print_colored "$GREEN" "🎉 Database restored successfully!"
        exit 0
    fi
    
    # Regular reset mode
    print_colored "$CYAN" "🗃️  HexaRAG Database Reset"
    print_colored "$CYAN" "========================"
    
    if [[ "$CREATE_BACKUP" == "true" ]]; then
        confirm_action "This will reset the database and create a backup."
    else
        confirm_action "This will reset the database WITHOUT creating a backup."
    fi
    
    # Create backup if requested
    if [[ "$CREATE_BACKUP" == "true" ]]; then
        local timestamp
        timestamp=$(date +"%Y-%m-%d_%H-%M-%S")
        local backup_file="$BACKUP_DIR/hexarag_${timestamp}.db"
        
        create_backup "$backup_file"
    fi
    
    # Reset database
    clear_database
    run_migrations
    
    # Load test data if requested
    if [[ "$LOAD_TEST_DATA" == "true" ]]; then
        load_test_data
    fi
    
    # Restart application
    restart_application
    
    # Verify database
    verify_database
    
    print_colored "$GREEN" "🎉 Database reset completed successfully!"
    
    if [[ "$CREATE_BACKUP" == "true" ]]; then
        print_colored "$BLUE" "💾 Backup created in: $BACKUP_DIR"
    fi
    
    if [[ "$LOAD_TEST_DATA" == "true" ]]; then
        print_colored "$BLUE" "📊 Test data loaded"
    fi
}

# Run main function
main "$@"