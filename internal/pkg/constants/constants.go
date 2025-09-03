package constants

import "time"

// Application constants
const (
	// Service identification
	ServiceName    = "hexarag"
	ServiceVersion = "v1.0.0"
	APIVersion     = "v1"
)

// Default timeouts
const (
	DefaultHTTPTimeout      = 10 * time.Second
	ShortHTTPTimeout        = 5 * time.Second
	LongHTTPTimeout         = 30 * time.Second
	DatabaseTimeout         = 10 * time.Second
	MessagingTimeout        = 5 * time.Second
	InferenceTimeout        = 30 * time.Second
	HealthCheckTimeout      = 5 * time.Second
	GracefulShutdownTimeout = 30 * time.Second
)

// Pagination defaults
const (
	DefaultPageLimit = 20
	MaxPageLimit     = 100
	MinPageLimit     = 1
)

// Database configuration
const (
	DefaultMaxOpenConns    = 25
	DefaultMaxIdleConns    = 25
	DefaultConnMaxLifetime = 5 * time.Minute

	// Database-specific constants
	DatabaseMaxOpenConns    = DefaultMaxOpenConns
	DatabaseMaxIdleConns    = DefaultMaxIdleConns
	DatabaseConnMaxLifetime = DefaultConnMaxLifetime

	// Query limits
	DefaultQueryLimit  = 50
	MaxQueryLimit      = 1000
	DefaultQueryOffset = 0

	// Migration configuration
	MigrationsTableName = "schema_migrations"
)

// HTTP status messages
const (
	StatusOK                 = "ok"
	StatusError              = "error"
	StatusProcessing         = "processing"
	StatusServiceUnavailable = "service_unavailable"
)

// Error messages
const (
	ErrMsgConversationNotFound = "conversation not found"
	ErrMsgMessageNotFound      = "message not found"
	ErrMsgSystemPromptNotFound = "system prompt not found"
	ErrMsgModelNotFound        = "model not found"
	ErrMsgInvalidRequest       = "invalid request"
	ErrMsgInternalServer       = "internal server error"
	ErrMsgServiceUnavailable   = "service unavailable"
	ErrMsgUnauthorized         = "unauthorized"
	ErrMsgForbidden            = "forbidden"
	ErrMsgRateLimited          = "rate limited"
)

// Success messages
const (
	MsgConversationDeleted = "conversation deleted"
	MsgMessageSent         = "message sent"
	MsgSystemPromptCreated = "system prompt created"
	MsgSystemPromptUpdated = "system prompt updated"
	MsgSystemPromptDeleted = "system prompt deleted"
	MsgModelSwitched       = "model switched successfully"
	MsgModelPulled         = "model pulled successfully"
	MsgModelDeleted        = "model deleted successfully"
)

// Context keys
const (
	ContextKeyUserID         = "user_id"
	ContextKeyConversationID = "conversation_id"
	ContextKeyMessageID      = "message_id"
	ContextKeyRequestID      = "request_id"
	ContextKeyTimeout        = "timeout"
	ContextKeyOperationType  = "operation_type"
)

// Operation types for timeout selection
const (
	OperationTypeStorage   = "storage"
	OperationTypeMessaging = "messaging"
	OperationTypeInference = "inference"
	OperationTypeDefault   = "default"
)

// Log levels
const (
	LogLevelDebug = "debug"
	LogLevelInfo  = "info"
	LogLevelWarn  = "warn"
	LogLevelError = "error"
	LogLevelFatal = "fatal"
)

// Log formats
const (
	LogFormatText = "text"
	LogFormatJSON = "json"
)

// HTTP headers
const (
	HeaderContentType   = "Content-Type"
	HeaderAuthorization = "Authorization"
	HeaderRequestID     = "X-Request-ID"
	HeaderCorrelationID = "X-Correlation-ID"
	HeaderUserAgent     = "User-Agent"
	HeaderAccept        = "Accept"
)

// Content types
const (
	ContentTypeJSON        = "application/json"
	ContentTypeTextPlain   = "text/plain"
	ContentTypeHTML        = "text/html"
	ContentTypeEventStream = "text/event-stream"
)

// Default system prompt ID
const (
	DefaultSystemPromptID = "default"
)

// Model defaults
const (
	DefaultModel       = "qwen3:0.6b"
	DefaultMaxTokens   = 4096
	DefaultTemperature = 0.7
)

// WebSocket configuration
const (
	WebSocketWriteWait      = 10 * time.Second
	WebSocketPongWait       = 60 * time.Second
	WebSocketPingPeriod     = (WebSocketPongWait * 9) / 10
	WebSocketMaxMessageSize = 512
)

// Rate limiting
const (
	DefaultRateLimit       = 100 // requests per minute
	DefaultRateLimitWindow = time.Minute
	DefaultBurstLimit      = 10
)

// File and directory paths
const (
	DefaultDataDir        = "./data"
	DefaultWebDir         = "./web"
	DefaultDevWebDir      = "./web/dev"
	DefaultMigrationsPath = "./internal/adapters/storage/sqlite/migrations"
	DefaultDBPath         = "./data/hexarag.db"
)

// Environment variable names
const (
	EnvPort        = "HEXARAG_PORT"
	EnvHost        = "HEXARAG_HOST"
	EnvLogLevel    = "HEXARAG_LOG_LEVEL"
	EnvLogFormat   = "HEXARAG_LOG_FORMAT"
	EnvDBPath      = "HEXARAG_DB_PATH"
	EnvNATSURL     = "HEXARAG_NATS_URL"
	EnvLLMProvider = "HEXARAG_LLM_PROVIDER"
	EnvLLMBaseURL  = "HEXARAG_LLM_BASE_URL"
	EnvLLMAPIKey   = "HEXARAG_LLM_API_KEY"
	EnvLLMModel    = "HEXARAG_LLM_MODEL"
	EnvDebugMode   = "HEXARAG_DEBUG_MODE"
	EnvCORSEnabled = "HEXARAG_CORS_ENABLED"
)

// Validation constraints
const (
	MinConversationTitleLength = 1
	MaxConversationTitleLength = 200
	MinMessageContentLength    = 1
	MaxMessageContentLength    = 10000
	MinSystemPromptNameLength  = 1
	MaxSystemPromptNameLength  = 100
	MinModelNameLength         = 1
	MaxModelNameLength         = 100
)

// Cache keys and TTLs
const (
	CacheKeyModels        = "models"
	CacheKeyConversations = "conversations"
	CacheKeySystemPrompts = "system_prompts"
	CacheTTLShort         = 5 * time.Minute
	CacheTTLMedium        = 30 * time.Minute
	CacheTTLLong          = 2 * time.Hour
)
