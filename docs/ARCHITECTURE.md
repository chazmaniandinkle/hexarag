# HexaRAG Architecture Guide

## Overview

HexaRAG implements a strict **Hexagonal Architecture** (also known as Ports and Adapters pattern) to achieve maximum portability, testability, and maintainability. This architectural pattern ensures that the core business logic remains independent of external systems like databases, messaging systems, and deployment environments.

## Core Principles

### 1. Dependency Inversion
The core domain depends only on abstractions (interfaces), never on concrete implementations. All external dependencies are injected through ports.

### 2. Separation of Concerns
- **Inside (The Hexagon)**: Pure business logic with no infrastructure knowledge
- **Outside (The Adapters)**: Infrastructure implementations that adapt external systems to our domain interfaces

### 3. Testability
Business logic can be tested in complete isolation using mock implementations of ports.

### 4. Portability
The same core logic runs unchanged across different environments by swapping adapter implementations.

## Architecture Layers

```
┌─────────────────────────────────────────────────────────────┐
│                     External Systems                        │
│  ┌─────────┐  ┌─────────┐  ┌─────────┐  ┌─────────┐       │
│  │ SQLite  │  │  NATS   │  │ Ollama  │  │  HTTP   │       │
│  └─────────┘  └─────────┘  └─────────┘  └─────────┘       │
└─────────────────────────────────────────────────────────────┘
                              │
┌─────────────────────────────────────────────────────────────┐
│                      Adapters Layer                         │
│  ┌─────────┐  ┌─────────┐  ┌─────────┐  ┌─────────┐       │
│  │Storage  │  │Messaging│  │   LLM   │  │   API   │       │
│  │Adapter  │  │ Adapter │  │ Adapter │  │ Adapter │       │
│  └─────────┘  └─────────┘  └─────────┘  └─────────┘       │
└─────────────────────────────────────────────────────────────┘
                              │
┌─────────────────────────────────────────────────────────────┐
│                      Ports Layer                            │
│  ┌─────────┐  ┌─────────┐  ┌─────────┐  ┌─────────┐       │
│  │Storage  │  │Messaging│  │   LLM   │  │  Tool   │       │
│  │  Port   │  │  Port   │  │  Port   │  │  Port   │       │
│  └─────────┘  └─────────┘  └─────────┘  └─────────┘       │
└─────────────────────────────────────────────────────────────┘
                              │
┌─────────────────────────────────────────────────────────────┐
│                   Domain Layer (Core)                       │
│  ┌─────────────────┐           ┌─────────────────┐          │
│  │    Services     │           │    Entities     │          │
│  │                 │           │                 │          │
│  │ Context         │◄──────────│ Message         │          │
│  │ Constructor     │           │ Conversation    │          │
│  │                 │           │ SystemPrompt    │          │
│  │ Inference       │           │ ToolCall        │          │
│  │ Engine          │           │                 │          │
│  └─────────────────┘           └─────────────────┘          │
└─────────────────────────────────────────────────────────────┘
```

## Domain Layer (The Hexagon)

### Entities
Pure domain objects with no external dependencies:

- **Message**: Represents a single conversation message
- **Conversation**: Manages conversation state and message ordering
- **SystemPrompt**: Reusable system prompts with templates
- **ToolCall**: Function calls made by the LLM with results

### Services
Business logic orchestration:

- **ContextConstructor**: Builds optimal context for LLM inference
- **InferenceEngine**: Orchestrates LLM calls and tool execution

## Ports Layer (Interfaces)

### StoragePort
Defines all persistence operations:
```go
type StoragePort interface {
    SaveMessage(ctx context.Context, message *entities.Message) error
    GetMessages(ctx context.Context, conversationID string, limit int) ([]*entities.Message, error)
    SaveConversation(ctx context.Context, conversation *entities.Conversation) error
    // ... other storage operations
}
```

### MessagingPort  
Defines event bus operations:
```go
type MessagingPort interface {
    Publish(ctx context.Context, subject string, data []byte) error
    Subscribe(ctx context.Context, subject string, handler MessageHandler) error
    // ... other messaging operations
}
```

### LLMPort
Defines language model operations:
```go
type LLMPort interface {
    Complete(ctx context.Context, request *CompletionRequest) (*CompletionResponse, error)
    CompleteStream(ctx context.Context, request *CompletionRequest, handler StreamHandler) error
    // ... other LLM operations
}
```

### ToolPort
Defines tool execution operations:
```go
type ToolPort interface {
    Execute(ctx context.Context, name string, arguments map[string]interface{}) (*ToolResult, error)
    GetAvailableTools(ctx context.Context) ([]Tool, error)
    // ... other tool operations
}
```

## Adapters Layer (The Outside)

### Storage Adapters
- **SQLiteAdapter**: File-based storage for local development
- **PostgreSQLAdapter**: (Planned) Production relational database
- **DynamoDBAdapter**: (Planned) NoSQL cloud storage

### Messaging Adapters
- **NATSAdapter**: Local/self-hosted messaging with JetStream
- **SQSAdapter**: (Planned) AWS managed messaging
- **RedisAdapter**: (Planned) Redis-based messaging

### LLM Adapters
- **OpenAIAdapter**: Works with OpenAI, Ollama, LM Studio, and compatible APIs
- **BedrockAdapter**: (Planned) AWS Bedrock integration
- **VertexAIAdapter**: (Planned) Google Cloud Vertex AI

### Tool Adapters
- **MCPTimeServerAdapter**: MCP-compatible time and date tools
- **MCPWebSearchAdapter**: (Planned) Web search capabilities
- **MCPCodeToolsAdapter**: (Planned) Code execution tools

## Event Flow Architecture

### Message Processing Flow

```
User Message → HTTP API → Storage → Context Request Event → 
Context Constructor → Context Ready Event → Inference Request Event → 
Inference Engine → LLM API → Tool Execution (if needed) → 
Response Event → WebSocket → User Interface
```

### Event Types

1. **Context Events**:
   - `context.request` - Request to build context
   - `context.ready` - Context construction complete

2. **Inference Events**:
   - `inference.request` - Request LLM completion
   - `inference.response` - LLM completion ready

3. **Tool Events**:
   - `tool.execute` - Execute a tool
   - `tool.result` - Tool execution result

4. **System Events**:
   - `system.error` - Error notifications
   - `system.health` - Health check events

## Configuration Architecture

### Environment-Specific Configuration
```yaml
# Local Development
database:
  adapter: "sqlite"
  path: "./data/hexarag.db"

messaging:
  adapter: "nats" 
  url: "nats://localhost:4222"

llm:
  adapter: "openai-compatible"
  base_url: "http://localhost:11434/v1"  # Ollama
```

```yaml
# Production (AWS)
database:
  adapter: "postgres"
  url: "postgres://..."

messaging:
  adapter: "sqs"
  queue_url: "https://sqs.us-west-2.amazonaws.com/..."

llm:
  adapter: "bedrock"
  region: "us-west-2"
  model: "anthropic.claude-v2"
```

## Dependency Injection

### Wire-up Pattern
All dependencies are injected at application startup:

```go
// cmd/server/main.go
func main() {
    // Load configuration
    cfg := config.Load()
    
    // Initialize adapters based on configuration
    storage := createStorageAdapter(cfg.Database)
    messaging := createMessagingAdapter(cfg.Messaging) 
    llm := createLLMAdapter(cfg.LLM)
    tools := createToolsAdapter(cfg.Tools)
    
    // Initialize services with injected dependencies
    contextConstructor := services.NewContextConstructor(storage, messaging, ...)
    inferenceEngine := services.NewInferenceEngine(storage, messaging, llm, tools)
    
    // Start services
    contextConstructor.StartListening(ctx)
    inferenceEngine.StartListening(ctx)
}
```

## Testing Architecture

### Unit Testing (Domain Layer)
Test business logic in isolation with mocks:

```go
func TestContextConstructor_BuildContext(t *testing.T) {
    // Arrange
    mockStorage := &MockStoragePort{}
    mockMessaging := &MockMessagingPort{}
    
    mockStorage.On("GetConversation", mock.Anything, "conv123").
        Return(testConversation, nil)
    
    cc := services.NewContextConstructor(mockStorage, mockMessaging, ...)
    
    // Act
    result, err := cc.BuildContext(ctx, &services.ContextRequest{
        ConversationID: "conv123",
    })
    
    // Assert
    assert.NoError(t, err)
    assert.NotNil(t, result)
    mockStorage.AssertExpectations(t)
}
```

### Integration Testing (Adapter Layer)
Test adapters with real dependencies:

```go
func TestSQLiteAdapter_Integration(t *testing.T) {
    // Use in-memory SQLite for fast tests
    adapter := sqlite.NewAdapter(":memory:", migrationPath)
    defer adapter.Close()
    
    // Test actual database operations
    message := entities.NewMessage("conv123", entities.RoleUser, "Hello")
    err := adapter.SaveMessage(ctx, message)
    assert.NoError(t, err)
    
    retrieved, err := adapter.GetMessage(ctx, message.ID)
    assert.NoError(t, err)
    assert.Equal(t, message.Content, retrieved.Content)
}
```

### End-to-End Testing
Test the complete system with test containers:

```go
func TestE2E_SendMessage(t *testing.T) {
    // Start test environment with real dependencies
    testEnv := setupTestEnvironment(t)
    defer testEnv.Cleanup()
    
    // Test complete flow
    response := testEnv.SendMessage("conv123", "Hello, world!")
    assert.Equal(t, http.StatusOK, response.StatusCode)
    
    // Verify message was processed
    messages := testEnv.GetMessages("conv123")
    assert.Len(t, messages, 2) // User message + AI response
}
```

## Deployment Architecture

### Local Development Stack
```
┌─────────────┐  ┌─────────────┐  ┌─────────────┐
│   HexaRAG   │  │    NATS     │  │   Ollama    │
│   Server    │◄─┤   Server    │  │   Server    │
│             │  │             │  │             │
│  ┌───────┐  │  └─────────────┘  └─────────────┘
│  │SQLite │  │
│  │ File  │  │
│  └───────┘  │
└─────────────┘
```

### Production Kubernetes Stack
```
┌─────────────┐  ┌─────────────┐  ┌─────────────┐
│  HexaRAG    │  │    NATS     │  │ PostgreSQL  │
│   Pods      │◄─┤   Cluster   │  │  Cluster    │
│(Horizontal  │  │             │  │             │
│ Pod Auto-   │  └─────────────┘  └─────────────┘
│ scaler)     │
└─────────────┘
       │
┌─────────────┐
│  External   │
│ LLM Service │
│(OpenAI/etc) │
└─────────────┘
```

### AWS Serverless Stack
```
┌─────────────┐  ┌─────────────┐  ┌─────────────┐
│   Lambda    │  │     SQS     │  │  DynamoDB/  │
│ Functions   │◄─┤   Queues    │  │     RDS     │
│             │  │             │  │             │
│ ┌─────────┐ │  └─────────────┘  └─────────────┘
│ │Context  │ │
│ │Construct│ │
│ │         │ │  ┌─────────────┐
│ │Inference│ │  │   Bedrock   │
│ │Engine   │ │◄─┤   Models    │
│ └─────────┘ │  │             │
└─────────────┘  └─────────────┘
```

## Error Handling Architecture

### Domain Errors
Business logic errors are represented as domain types:
```go
type DomainError struct {
    Code    string
    Message string
    Context map[string]interface{}
}
```

### Adapter Error Translation
Adapters translate infrastructure errors to domain errors:
```go
func (a *SQLiteAdapter) SaveMessage(ctx context.Context, msg *entities.Message) error {
    if err := a.db.Insert(msg); err != nil {
        if isForeignKeyError(err) {
            return &DomainError{
                Code: "CONVERSATION_NOT_FOUND",
                Message: "Referenced conversation does not exist",
            }
        }
        return fmt.Errorf("storage error: %w", err)
    }
    return nil
}
```

### Circuit Breaker Pattern
For external services (LLM APIs, etc.):
```go
type CircuitBreaker struct {
    failureThreshold int
    timeout          time.Duration
    state           State
}

func (cb *CircuitBreaker) Execute(operation func() error) error {
    if cb.state == Open {
        return ErrCircuitOpen
    }
    
    err := operation()
    cb.recordResult(err)
    return err
}
```

## Security Architecture

### Input Validation
All inputs are validated at the adapter boundaries:
```go
func (h *APIHandlers) sendMessage(c *gin.Context) {
    var req MessageRequest
    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(400, gin.H{"error": "Invalid request"})
        return
    }
    
    if err := req.Validate(); err != nil {
        c.JSON(400, gin.H{"error": err.Error()})
        return
    }
    
    // Continue with business logic
}
```

### Sanitization
Prevent injection attacks:
```go
func (a *SQLiteAdapter) SaveMessage(ctx context.Context, msg *entities.Message) error {
    // Use parameterized queries
    query := "INSERT INTO messages (id, content, ...) VALUES (?, ?, ...)"
    _, err := a.db.ExecContext(ctx, query, msg.ID, msg.Content, ...)
    return err
}
```

### Rate Limiting
Implemented at the adapter level:
```go
type RateLimitedLLMAdapter struct {
    wrapped LLMPort
    limiter *rate.Limiter
}

func (r *RateLimitedLLMAdapter) Complete(ctx context.Context, req *CompletionRequest) (*CompletionResponse, error) {
    if !r.limiter.Allow() {
        return nil, ErrRateLimited
    }
    return r.wrapped.Complete(ctx, req)
}
```

## Monitoring Architecture

### Observability Ports
```go
type MetricsPort interface {
    Counter(name string) Counter
    Histogram(name string) Histogram
    Gauge(name string) Gauge
}

type TracingPort interface {
    StartSpan(ctx context.Context, name string) (context.Context, Span)
}
```

### Instrumentation
```go
func (ie *InferenceEngine) ExecuteInference(ctx context.Context, req *InferenceRequest) (*InferenceResponse, error) {
    ctx, span := ie.tracer.StartSpan(ctx, "inference.execute")
    defer span.End()
    
    ie.metrics.Counter("inference.requests").Inc()
    
    start := time.Now()
    response, err := ie.doInference(ctx, req)
    ie.metrics.Histogram("inference.duration").Observe(time.Since(start))
    
    if err != nil {
        ie.metrics.Counter("inference.errors").Inc()
        span.SetError(err)
    }
    
    return response, err
}
```

This architecture ensures that HexaRAG remains maintainable, testable, and portable across all deployment environments while providing a solid foundation for future enhancements.