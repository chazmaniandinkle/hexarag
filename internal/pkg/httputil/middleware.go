package httputil

import (
	"context"
	"time"

	"github.com/gin-gonic/gin"
)

// ContextKey represents a context key type to avoid collisions
type ContextKey string

const (
	// TimeoutConfigKey is the context key for timeout configuration
	TimeoutConfigKey ContextKey = "timeout_config"
	// OperationTypeKey is the context key for operation type
	OperationTypeKey ContextKey = "operation_type"
)

// MiddlewareConfig holds middleware configuration
type MiddlewareConfig struct {
	Timeouts       TimeoutConfig
	EnableCORS     bool
	AllowedOrigins []string
	AllowedMethods []string
	AllowedHeaders []string
}

// DefaultMiddlewareConfig provides sensible defaults
var DefaultMiddlewareConfig = MiddlewareConfig{
	Timeouts:       DefaultTimeouts,
	EnableCORS:     true,
	AllowedOrigins: []string{"*"},
	AllowedMethods: []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
	AllowedHeaders: []string{"Content-Type", "Authorization"},
}

// TimeoutMiddleware creates a middleware that injects timeout configuration into context
func TimeoutMiddleware(config TimeoutConfig) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Inject timeout configuration into Gin context
		c.Set(string(TimeoutConfigKey), config)
		c.Next()
	}
}

// OperationTypeMiddleware sets the operation type for timeout selection
func OperationTypeMiddleware(operationType string) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Set(string(OperationTypeKey), operationType)
		c.Next()
	}
}

// CORSMiddleware creates a configurable CORS middleware
func CORSMiddleware(config MiddlewareConfig) gin.HandlerFunc {
	return func(c *gin.Context) {
		if config.EnableCORS {
			for _, origin := range config.AllowedOrigins {
				c.Header("Access-Control-Allow-Origin", origin)
			}

			methods := "GET, POST, PUT, DELETE, OPTIONS"
			if len(config.AllowedMethods) > 0 {
				methods = ""
				for i, method := range config.AllowedMethods {
					if i > 0 {
						methods += ", "
					}
					methods += method
				}
			}
			c.Header("Access-Control-Allow-Methods", methods)

			headers := "Content-Type, Authorization"
			if len(config.AllowedHeaders) > 0 {
				headers = ""
				for i, header := range config.AllowedHeaders {
					if i > 0 {
						headers += ", "
					}
					headers += header
				}
			}
			c.Header("Access-Control-Allow-Headers", headers)

			if c.Request.Method == "OPTIONS" {
				c.AbortWithStatus(204)
				return
			}
		}
		c.Next()
	}
}

// GetTimeoutForOperation retrieves appropriate timeout for operation from context
func GetTimeoutForOperation(c *gin.Context, operationType string) time.Duration {
	configInterface, exists := c.Get(string(TimeoutConfigKey))
	if !exists {
		return DefaultTimeouts.Default
	}

	config, ok := configInterface.(TimeoutConfig)
	if !ok {
		return DefaultTimeouts.Default
	}

	switch operationType {
	case "storage":
		return config.Default
	case "messaging":
		return config.Short
	case "inference":
		return config.Long
	default:
		return config.Default
	}
}

// WithOperationContext creates a context with timeout for specific operation
func WithOperationContext(c *gin.Context, operationType string) (context.Context, context.CancelFunc) {
	timeout := GetTimeoutForOperation(c, operationType)
	return context.WithTimeout(context.Background(), timeout)
}
