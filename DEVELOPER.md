# HexaRAG Developer Guide

Welcome to the HexaRAG development environment! This guide will help you get started with development, testing, and contributing to the project.

## Quick Start

### Prerequisites

- **Docker & Docker Compose** - Container orchestration
- **Go 1.21+** - Backend development
- **Git** - Version control
- **curl & jq** - API testing (recommended)
- **8GB+ disk space** - For AI models

### One-Command Setup

```bash
# Complete development environment setup
make dev-setup-full

# Or using the script directly
./scripts/dev-setup.sh
```

This will:
1. Check system dependencies
2. Install Go dependencies
3. Initialize the database
4. Download default AI models
5. Start all services
6. Run health checks

## Developer Scripts (Phase 3D)

HexaRAG includes a comprehensive suite of developer scripts to streamline common tasks:

### 🗣️ Interactive Chat Testing

```bash
# Basic chat testing
./scripts/test-chat.sh

# Test with specific model and API
./scripts/test-chat.sh -m llama3.2:3b -u http://localhost:8080

# Save conversation history
./scripts/test-chat.sh -s my-conversation.json

# Load test messages from file
./scripts/test-chat.sh -l test-messages.txt

# Makefile shortcuts
make test-chat
```

**Features:**
- Interactive CLI chat interface
- Model switching during conversation
- Conversation history saving/loading
- Extended knowledge toggle
- Verbose and quiet modes

### 📥 Model Management

```bash
# List available models
./scripts/pull-model.sh --list

# Download specific model
./scripts/pull-model.sh deepseek-r1:1.5b

# Download multiple models from file
./scripts/pull-model.sh --batch models.txt

# Check if model is available
./scripts/pull-model.sh --check llama3.2:3b

# Makefile shortcuts
make pull-model MODEL=deepseek-r1:1.5b
make models  # List models
```

**Popular Models:**
- `deepseek-r1:1.5b` - Fast reasoning (1.7GB)
- `llama3.2:3b` - Balanced general model (2.0GB)
- `qwen2.5:1.5b` - Compact multilingual (934MB)

### 🗃️ Database Management

```bash
# Reset database with backup
./scripts/reset-db.sh

# Reset without backup (faster)
./scripts/reset-db.sh --no-backup --force

# Restore from backup
./scripts/reset-db.sh --restore ./backups/hexarag_2024-01-15_14-30-45.db

# Load test data after reset
./scripts/reset-db.sh --test-data

# Makefile shortcuts
make reset-db
```

**Features:**
- Automatic backup creation
- Safe migration re-running
- Test data loading
- Backup restoration

### 🏥 Health Monitoring

```bash
# Basic health check
./scripts/health-check.sh

# Detailed health check
./scripts/health-check.sh --detailed --verbose

# Continuous monitoring
./scripts/health-check.sh --continuous --interval 60

# JSON output for monitoring
./scripts/health-check.sh --json

# Makefile shortcuts
make health
```

**Checks:**
- API server response
- Ollama service and models
- NATS messaging
- Database accessibility
- Docker containers
- System resources
- Network connectivity

### 📊 Performance Testing

```bash
# Basic benchmark
./scripts/benchmark.sh

# Load testing
./scripts/benchmark.sh --type load --concurrent 10 --requests 50

# Stress testing
./scripts/benchmark.sh --type stress --verbose

# Save results to CSV
./scripts/benchmark.sh --csv --output results.csv

# Makefile shortcuts
make benchmark
```

**Test Types:**
- **Conversation** - Full conversation flow testing
- **Load** - Increasing concurrent user load
- **Stress** - Find system breaking points
- **Response** - Response time analysis

## Development Workflow

### 1. Environment Setup

```bash
# Initial setup
git clone <repository>
cd hexarag
make dev-setup-full

# Verify setup
make health
```

### 2. Code Development

```bash
# Start development with auto-reload
make dev

# Or manual build and run
make build
make run

# Format and check code
make fmt
make lint
make vet
```

### 3. Testing

```bash
# Run unit tests
make test

# Test with coverage
make test-coverage

# Interactive API testing
make test-chat

# Performance testing
make benchmark
```

### 4. Database Changes

```bash
# Create migration file
# Add to internal/adapters/storage/sqlite/migrations/

# Run migrations
make migrate

# Reset database for testing
make reset-db
```

## Architecture Overview

HexaRAG follows hexagonal (ports and adapters) architecture:

```
├── cmd/                    # Application entry points
│   ├── server/            # Main server
│   └── migrate/           # Migration tool
├── internal/              # Private application code
│   ├── adapters/          # External interfaces
│   │   ├── api/          # HTTP API
│   │   ├── llm/          # LLM integrations
│   │   ├── messaging/    # NATS messaging
│   │   └── storage/      # Database
│   ├── domain/           # Business logic
│   │   ├── entities/     # Domain models
│   │   ├── ports/        # Interfaces
│   │   └── services/     # Business services
│   └── infrastructure/   # Cross-cutting concerns
├── web/                  # Frontend assets
├── scripts/              # Developer tools
└── deployments/         # Deployment configs
```

## API Reference

### Base URL
```
http://localhost:8080/api/v1
```

### Key Endpoints

#### Conversations
```bash
# List conversations
GET /conversations

# Create conversation
POST /conversations
{
  "title": "My Chat",
  "system_prompt_id": "default"
}

# Send message
POST /conversations/{id}/messages
{
  "content": "Hello!",
  "use_extended_knowledge": false
}
```

#### Models
```bash
# List available models
GET /models

# Pull model
POST /models/pull
{
  "model": "deepseek-r1:1.5b"
}

# Switch model
PUT /models/current
{
  "model": "llama3.2:3b",
  "conversation_id": "conv-123"
}
```

#### Health
```bash
# System health
GET /health

# Model status
GET /models/status

# Inference status
GET /inference/status
```

## Configuration

### Environment Variables

```bash
# API Configuration
HEXARAG_API_URL=http://localhost:8080
HEXARAG_MODEL=deepseek-r1:8b

# Ollama Configuration
OLLAMA_URL=http://localhost:11434

# Database
DATABASE_PATH=/data/hexarag.db

# Messaging
NATS_URL=nats://localhost:4222
```

### Docker Compose Services

```yaml
services:
  hexarag:        # Main application
  hexarag-db:     # SQLite database
  ollama:         # AI model server
  nats:           # Message broker
```

## Troubleshooting

### Common Issues

#### API Not Responding
```bash
# Check service status
docker-compose ps

# Check health
make health

# View logs
docker-compose logs hexarag
```

#### Models Not Available
```bash
# Check Ollama
curl http://localhost:11434/api/tags

# Download model
make pull-model MODEL=deepseek-r1:1.5b

# Check model status
make models
```

#### Database Issues
```bash
# Reset database
make reset-db

# Check database file
docker exec hexarag ls -la /data/
```

#### Performance Issues
```bash
# Run benchmark
make benchmark

# Check system resources
./scripts/health-check.sh --detailed
```

### Debug Mode

```bash
# Enable verbose logging
export DEBUG=true

# Run with debug info
go run -race ./cmd/server
```

## Contributing

### Code Style

- Follow Go conventions (`gofmt`, `golint`)
- Write tests for new features
- Update documentation
- Use meaningful commit messages

### Pull Request Process

1. Create feature branch
2. Implement changes with tests
3. Run quality checks: `make fmt lint vet test`
4. Update documentation
5. Submit pull request

### Testing Guidelines

- Unit tests for business logic
- Integration tests for API endpoints
- Performance tests for critical paths
- Use table-driven tests where appropriate

## Monitoring and Observability

### Health Checks

```bash
# Manual health check
make health

# Automated monitoring
./scripts/health-check.sh --continuous --json > health.log
```

### Performance Monitoring

```bash
# Continuous benchmarking
./scripts/benchmark.sh --type load --output performance.csv

# Resource monitoring
docker stats
```

### Logging

- Application logs: `docker-compose logs hexarag`
- Ollama logs: `docker-compose logs ollama`
- NATS logs: `docker-compose logs nats`

## Advanced Development

### Custom Models

```bash
# Pull custom model
./scripts/pull-model.sh custom-model:latest

# Test custom model
./scripts/test-chat.sh -m custom-model:latest
```

### API Extensions

1. Add endpoint to `internal/adapters/api/http/handlers.go`
2. Implement business logic in `internal/domain/services/`
3. Add tests
4. Update API documentation

### Database Schema Changes

1. Create migration in `internal/adapters/storage/sqlite/migrations/`
2. Test migration: `make reset-db`
3. Update entities in `internal/domain/entities/`

## Performance Optimization

### Database

- Index frequently queried columns
- Use prepared statements
- Optimize N+1 queries

### API

- Implement response caching
- Use compression middleware
- Optimize JSON serialization

### Models

- Use appropriate model sizes
- Implement model warming
- Monitor memory usage

## Security Considerations

- Validate all inputs
- Sanitize user content
- Use HTTPS in production
- Implement rate limiting
- Regular security updates

## Production Deployment

### Docker

```bash
# Build production image
make docker-build

# Deploy with compose
docker-compose -f docker-compose.prod.yml up -d
```

### Environment Setup

```bash
# Production setup script
./scripts/dev-setup.sh --skip-models --quiet
```

### Monitoring

```bash
# Health check endpoint for load balancers
curl http://localhost:8080/health

# Metrics collection
./scripts/health-check.sh --json --continuous --interval 30
```

## Resources

- [Go Documentation](https://golang.org/doc/)
- [Docker Compose Reference](https://docs.docker.com/compose/)
- [Ollama Documentation](https://ollama.ai/docs)
- [NATS Documentation](https://docs.nats.io/)

## Support

- Check existing issues in the repository
- Use the developer scripts for debugging
- Run comprehensive health checks
- Review logs for error details

---

*This guide is updated as part of Phase 3D: Developer Scripts Suite. For the latest information, check the repository documentation.*