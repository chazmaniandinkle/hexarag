package httputil
package httputil

import (
	"context"
	"time"
)

// TimeoutConfig holds timeout configurations for different operations
type TimeoutConfig struct {
	Default time.Duration
	Short   time.Duration
	Long    time.Duration
}

// DefaultTimeouts provides sensible default timeout values
var DefaultTimeouts = TimeoutConfig{
	Default: 10 * time.Second,
	Short:   5 * time.Second,
	Long:    30 * time.Second,
}

// WithTimeout creates a context with the specified timeout duration
func WithTimeout(duration time.Duration) (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), duration)
}

// WithDefaultTimeout creates a context with the default timeout
func WithDefaultTimeout() (context.Context, context.CancelFunc) {
	return WithTimeout(DefaultTimeouts.Default)
}

// WithShortTimeout creates a context with a short timeout for quick operations
func WithShortTimeout() (context.Context, context.CancelFunc) {
	return WithTimeout(DefaultTimeouts.Short)
}

// WithLongTimeout creates a context with a long timeout for expensive operations
func WithLongTimeout() (context.Context, context.CancelFunc) {
	return WithTimeout(DefaultTimeouts.Long)
}

// WithCustomTimeout creates a context with a custom timeout based on operation type
func WithCustomTimeout(operationType string, config TimeoutConfig) (context.Context, context.CancelFunc) {
	switch operationType {
	case "storage":
		return WithTimeout(config.Default)
	case "messaging":
		return WithTimeout(config.Short)
	case "inference":
		return WithTimeout(config.Long)
	default:
		return WithTimeout(config.Default)
	}
}