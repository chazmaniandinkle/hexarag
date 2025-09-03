# HexaRAG

A hexagonal architecture-based Retrieval-Augmented Generation (RAG) conversational AI system designed for maximum **portability**, **testability**, and **scalability**. Built with Go, the system can run entirely locally with minimal dependencies or deploy to full serverless cloud environments without changing the core application logic.

## ✨ Features

- **🏗️ Hexagonal Architecture**: Clean separation between business logic and infrastructure
- **🔄 Multi-Environment Deployment**: Same codebase runs locally (Docker + SQLite + NATS) or in cloud (AWS Lambda + SQS + OpenSearch)
- **🤖 OpenAI-Compatible**: Works with Ollama, LM Studio, OpenAI, and other compatible APIs
- **🔧 Tool Support**: MCP (Model Context Protocol) compatible tool system
- **💬 Real-time Communication**: WebSocket support for live conversation updates
- **📊 Context Management**: Intelligent conversation context construction with token management
- **🗃️ Flexible Storage**: SQLite for local development, designed for easy swap to production databases
- **⚡ Event-Driven**: NATS-based messaging for scalable service communication
- **🎯 Type-Safe**: Full Go type safety with comprehensive interfaces

## 🚀 Quick Start

### Prerequisites

- Go 1.21+
- Make (optional, for convenience commands)

### Local Development

1. **Clone and setup:**
```bash
git clone https://github.com/yourusername/hexarag.git
cd hexarag
make deps
```

2. **Run database migrations:**
```bash
make migrate
```

3. **Start the server:**
```bash
make run
```

The server will start on `http://localhost:8080` with:
- REST API at `/api/v1/*`
- WebSocket endpoint at `/ws`
- Web interface at `/` (when implemented)

### Docker Development

```bash
make docker-run
```

This starts all services including NATS server and Ollama (when Docker Compose is implemented).

## 🏛️ Architecture

HexaRAG follows strict hexagonal architecture principles:

### Core Domain (The Hexagon)
- **Entities**: `Message`, `Conversation`, `SystemPrompt`, `ToolCall`
- **Services**: `ContextConstructor`, `InferenceEngine`
- **Ports**: Interfaces defining contracts with external systems

### Adapters (The Outside)
- **Storage**: SQLite adapter (swappable with PostgreSQL, etc.)
- **Messaging**: NATS adapter (swappable with SQS, Redis, etc.)
- **LLM**: OpenAI-compatible adapter (works with Ollama, LM Studio, OpenAI)
- **Tools**: MCP time server (extensible to any MCP-compatible tools)
- **API**: HTTP/WebSocket adapters

## 📁 Project Structure

```
hexarag/
├── cmd/
│   ├── server/          # Main application entry point
│   └── migrate/         # Database migration utility
├── internal/
│   ├── domain/          # Core business logic (the hexagon)
│   │   ├── entities/    # Domain entities
│   │   ├── services/    # Business services
│   │   └── ports/       # Interface contracts
│   └── adapters/        # Infrastructure implementations
│       ├── storage/     # Database adapters
│       ├── messaging/   # Event bus adapters
│       ├── llm/         # Language model adapters
│       ├── tools/       # Tool execution adapters
│       └── api/         # HTTP/WebSocket adapters
├── pkg/                 # Shared utilities
├── deployments/         # Deployment configurations
├── web/                 # Frontend assets
└── docs/                # Documentation
```

## ⚙️ Configuration

Configuration is managed via YAML files and environment variables:

```yaml
# deployments/config/config.yaml
server:
  port: 8080
  host: "0.0.0.0"

llm:
  provider: "openai-compatible"
  base_url: "http://localhost:11434/v1"  # Ollama
  model: "llama2"
  max_tokens: 4096
  temperature: 0.7

nats:
  url: "nats://localhost:4222"
  jetstream:
    enabled: true
    retention_days: 7

database:
  path: "./data/hexarag.db"

tools:
  mcp_time_server:
    enabled: true
    timezones: ["UTC", "America/New_York", "Europe/London"]
```

Override with environment variables:
```bash
export HEXARAG_LLM_BASE_URL="http://localhost:1234/v1"  # LM Studio
export HEXARAG_LLM_MODEL="llama-3.2-3b"
```

## 🔌 API Reference

### REST API

**Conversations:**
- `GET /api/v1/conversations` - List conversations
- `POST /api/v1/conversations` - Create conversation
- `GET /api/v1/conversations/{id}` - Get conversation
- `PUT /api/v1/conversations/{id}` - Update conversation
- `DELETE /api/v1/conversations/{id}` - Delete conversation

**Messages:**
- `GET /api/v1/conversations/{id}/messages` - Get messages
- `POST /api/v1/conversations/{id}/messages` - Send message

**System Prompts:**
- `GET /api/v1/system-prompts` - List system prompts
- `POST /api/v1/system-prompts` - Create system prompt
- `GET /api/v1/system-prompts/{id}` - Get system prompt
- `PUT /api/v1/system-prompts/{id}` - Update system prompt
- `DELETE /api/v1/system-prompts/{id}` - Delete system prompt

### WebSocket API

Connect to `/ws?conversation_id={id}` for real-time updates:

```javascript
const ws = new WebSocket('ws://localhost:8080/ws?conversation_id=conv123');

ws.onmessage = (event) => {
  const message = JSON.parse(event.data);
  console.log('Received:', message.type, message.data);
};
```

## 🛠️ Development

### Available Commands

```bash
make build          # Build binaries
make run            # Run development server
make test           # Run tests
make test-coverage  # Run tests with coverage
make migrate        # Run database migrations
make fmt            # Format code
make lint           # Lint code (requires golangci-lint)
make docker-build   # Build Docker image
make docker-run     # Run with Docker Compose
make dev-setup      # Setup development environment
make help           # Show all commands
```

### Adding New Adapters

1. **Define the port interface** in `internal/domain/ports/`
2. **Implement the adapter** in `internal/adapters/`
3. **Wire it up** in `cmd/server/main.go`

Example: Adding PostgreSQL support:
```go
// internal/adapters/storage/postgres/postgres_adapter.go
type Adapter struct { /* ... */ }

func (a *Adapter) SaveMessage(ctx context.Context, msg *entities.Message) error {
    // PostgreSQL implementation
}
```

### Testing

The hexagonal architecture makes testing straightforward:

```go
// Test business logic with mocks
mockStorage := &MockStoragePort{}
contextConstructor := services.NewContextConstructor(mockStorage, ...)

// Test adapters with real dependencies
sqliteAdapter := sqlite.NewAdapter(":memory:", "")
```

## 🚀 Deployment

### Local Development
- **Storage**: SQLite file database
- **Messaging**: Local NATS server
- **LLM**: Ollama or LM Studio
- **Tools**: Built-in MCP time server

### Production Options

**Self-Hosted (Kubernetes):**
- **Storage**: PostgreSQL
- **Messaging**: NATS cluster
- **LLM**: Self-hosted models or API services
- **Deployment**: Helm charts (planned)

**Cloud-Native (AWS Serverless):**
- **Storage**: Amazon RDS or DynamoDB
- **Messaging**: Amazon SQS + EventBridge
- **LLM**: Amazon Bedrock or OpenAI
- **Deployment**: AWS SAM (planned)

## 🗺️ Roadmap

### Phase 1: Foundation ✅
- [x] Hexagonal architecture core
- [x] Local development stack
- [x] Basic conversation management
- [x] OpenAI-compatible LLM integration
- [x] MCP tool system
- [x] Real-time WebSocket communication

### Phase 2: Enhanced RAG (Planned)
- [ ] Vector storage integration (ChromaDB/OpenSearch)
- [ ] Semantic search capabilities
- [ ] Document ingestion pipeline
- [ ] Advanced context construction

### Phase 3: Cloud Deployment (Planned)
- [ ] AWS Lambda adapters
- [ ] Kubernetes deployment
- [ ] Production monitoring
- [ ] Multi-tenancy support

### Phase 4: Production Features (Planned)
- [ ] Authentication and authorization
- [ ] Rate limiting and quotas
- [ ] Advanced tool ecosystem
- [ ] Analytics and insights

## 🧪 Testing

```bash
# Run all tests
make test

# Run with coverage
make test-coverage

# Test specific package
go test ./internal/domain/services -v
```

## 📚 Documentation

- [Architecture Guide](docs/ARCHITECTURE.md) - Detailed hexagonal architecture explanation
- [Deployment Guide](docs/DEPLOYMENT.md) - Production deployment strategies
- [API Documentation](docs/API.md) - Complete API reference
- [Contributing Guide](CONTRIBUTING.md) - How to contribute

## 🤝 Contributing

1. Fork the repository
2. Create a feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -m 'Add amazing feature'`)
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request

## 📄 License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

## 🙏 Acknowledgments

- Inspired by hexagonal architecture principles
- Built for the Model Context Protocol (MCP) ecosystem
- Designed for the modern AI application landscape