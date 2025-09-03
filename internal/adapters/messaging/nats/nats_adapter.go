package nats

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/nats-io/nats.go"

	"hexarag/internal/domain/ports"
	"hexarag/internal/pkg/constants"
	"hexarag/internal/pkg/logutil"
)

// Adapter implements the MessagingPort interface using NATS
type Adapter struct {
	conn      *nats.Conn
	js        nats.JetStreamContext
	subs      map[string]*nats.Subscription
	subsMutex sync.RWMutex
	logger    *logutil.FieldLogger
	config    Config
}

// Config holds NATS adapter configuration
type Config struct {
	URL              string
	JetStreamEnabled bool
	RetentionDays    int
	ReconnectWait    time.Duration
	MaxReconnects    int
	ReconnectBufSize int
	PublishTimeout   time.Duration
	RequestTimeout   time.Duration
	MaxStreamMsgs    int64
	MaxStreamBytes   int64
}

// DefaultConfig provides sensible NATS defaults
var DefaultConfig = Config{
	ReconnectWait:    constants.DefaultHTTPTimeout / 5, // 2 seconds
	MaxReconnects:    -1,                               // Unlimited
	ReconnectBufSize: 5 * 1024 * 1024,                  // 5MB
	PublishTimeout:   constants.MessagingTimeout,
	RequestTimeout:   constants.DefaultHTTPTimeout,
	MaxStreamMsgs:    100000,
	MaxStreamBytes:   1024 * 1024 * 1024, // 1GB
}

// NewAdapter creates a new NATS messaging adapter
func NewAdapter(url string, jsEnabled bool, retentionDays int) (*Adapter, error) {
	config := DefaultConfig
	config.URL = url
	config.JetStreamEnabled = jsEnabled
	config.RetentionDays = retentionDays

	return NewAdapterWithConfig(config)
}

// NewAdapterWithConfig creates a new NATS messaging adapter with custom configuration
func NewAdapterWithConfig(config Config) (*Adapter, error) {
	// Create logger for this adapter
	logger := logutil.NewDefaultLogger().WithFields(logutil.Fields{
		"component": "nats_adapter",
		"url":       config.URL,
		"jetstream": config.JetStreamEnabled,
	})

	logger.Info("Initializing NATS adapter")

	// Connect to NATS with configuration
	conn, err := nats.Connect(config.URL,
		nats.ReconnectWait(config.ReconnectWait),
		nats.MaxReconnects(config.MaxReconnects),
		nats.ReconnectBufSize(config.ReconnectBufSize),
		nats.Name(constants.ServiceName+"-messaging"),
		nats.ReconnectHandler(func(nc *nats.Conn) {
			logger.Info("NATS reconnected", logutil.Fields{
				"url": nc.ConnectedUrl(),
			})
		}),
		nats.DisconnectHandler(func(nc *nats.Conn) {
			logger.Warn("NATS disconnected")
		}),
		nats.ErrorHandler(func(nc *nats.Conn, s *nats.Subscription, err error) {
			logger.Error("NATS error", logutil.Fields{
				"subject": s.Subject,
				"error":   err.Error(),
			})
		}),
	)
	if err != nil {
		logger.Error("Failed to connect to NATS", logutil.Fields{
			"error": err.Error(),
		})
		return nil, fmt.Errorf("failed to connect to NATS: %w", err)
	}

	adapter := &Adapter{
		conn:   conn,
		subs:   make(map[string]*nats.Subscription),
		logger: logger,
		config: config,
	}

	// Setup JetStream if enabled
	if config.JetStreamEnabled {
		js, err := conn.JetStream(nats.PublishAsyncMaxPending(256))
		if err != nil {
			conn.Close()
			logger.Error("Failed to get JetStream context", logutil.Fields{
				"error": err.Error(),
			})
			return nil, fmt.Errorf("failed to get JetStream context: %w", err)
		}
		adapter.js = js

		// Create streams
		if err := adapter.setupStreams(); err != nil {
			conn.Close()
			logger.Error("Failed to setup JetStream streams", logutil.Fields{
				"error": err.Error(),
			})
			return nil, fmt.Errorf("failed to setup JetStream streams: %w", err)
		}
	}

	logger.Info("NATS adapter initialized successfully")
	return adapter, nil
}

// setupStreams creates the necessary JetStream streams
func (a *Adapter) setupStreams() error {
	a.logger.Info("Setting up JetStream streams")

	streams := []struct {
		name     string
		subjects []string
	}{
		{
			name:     "CONVERSATION_EVENTS",
			subjects: []string{"conversation.>"},
		},
		{
			name:     "INFERENCE_EVENTS",
			subjects: []string{"inference.>"},
		},
		{
			name:     "TOOL_EVENTS",
			subjects: []string{"tool.>"},
		},
		{
			name:     "CONTEXT_EVENTS",
			subjects: []string{"context.>"},
		},
		{
			name:     "SYSTEM_EVENTS",
			subjects: []string{"system.>"},
		},
	}

	for _, stream := range streams {
		cfg := &nats.StreamConfig{
			Name:        stream.name,
			Subjects:    stream.subjects,
			Retention:   nats.LimitsPolicy,
			MaxAge:      time.Duration(a.config.RetentionDays) * 24 * time.Hour,
			MaxMsgs:     a.config.MaxStreamMsgs,
			MaxBytes:    a.config.MaxStreamBytes,
			Storage:     nats.FileStorage,
			Compression: nats.S2Compression,
		}

		// Check if stream exists
		info, err := a.js.StreamInfo(stream.name)
		if err != nil {
			if err == nats.ErrStreamNotFound {
				// Create new stream
				_, err = a.js.AddStream(cfg)
				if err != nil {
					a.logger.Error("Failed to create stream", logutil.Fields{
						"stream": stream.name,
						"error":  err.Error(),
					})
					return fmt.Errorf("failed to create stream %s: %w", stream.name, err)
				}
				a.logger.Debug("Stream created successfully", logutil.Fields{
					"stream": stream.name,
				})
			} else {
				a.logger.Error("Failed to get stream info", logutil.Fields{
					"stream": stream.name,
					"error":  err.Error(),
				})
				return fmt.Errorf("failed to get stream info for %s: %w", stream.name, err)
			}
		} else {
			// Update existing stream if configuration changed
			if a.needsUpdate(info.Config, *cfg) {
				_, err = a.js.UpdateStream(cfg)
				if err != nil {
					a.logger.Error("Failed to update stream", logutil.Fields{
						"stream": stream.name,
						"error":  err.Error(),
					})
					return fmt.Errorf("failed to update stream %s: %w", stream.name, err)
				}
				a.logger.Debug("Stream updated successfully", logutil.Fields{
					"stream": stream.name,
				})
			}
		}
	}

	a.logger.Info("JetStream streams setup completed")
	return nil
}

// needsUpdate checks if a stream configuration needs updating
func (a *Adapter) needsUpdate(existing, desired nats.StreamConfig) bool {
	return existing.MaxAge != desired.MaxAge ||
		existing.MaxMsgs != desired.MaxMsgs ||
		existing.MaxBytes != desired.MaxBytes ||
		existing.Compression != desired.Compression
}

// Publish sends a message to the specified subject
func (a *Adapter) Publish(ctx context.Context, subject string, data []byte) error {
	a.logger.Debug("Publishing message", logutil.Fields{
		"subject": subject,
		"size":    len(data),
	})

	if a.js != nil {
		// Use JetStream for persistent messaging
		_, err := a.js.PublishAsync(subject, data)
		if err != nil {
			a.logger.Error("Failed to publish to JetStream", logutil.Fields{
				"subject": subject,
				"error":   err.Error(),
			})
			return fmt.Errorf("failed to publish to JetStream subject %s: %w", subject, err)
		}

		// Wait for publish acknowledgment with timeout from configuration
		select {
		case <-a.js.PublishAsyncComplete():
			a.logger.Debug("Message published successfully", logutil.Fields{
				"subject": subject,
			})
			return nil
		case <-ctx.Done():
			a.logger.Error("Publish context cancelled", logutil.Fields{
				"subject": subject,
				"error":   ctx.Err().Error(),
			})
			return fmt.Errorf("publish timeout for subject %s: %w", subject, ctx.Err())
		case <-time.After(a.config.PublishTimeout):
			a.logger.Error("Publish timeout", logutil.Fields{
				"subject": subject,
				"timeout": a.config.PublishTimeout,
			})
			return fmt.Errorf("publish timeout for subject %s", subject)
		}
	} else {
		// Use core NATS for non-persistent messaging
		err := a.conn.Publish(subject, data)
		if err != nil {
			a.logger.Error("Failed to publish to core NATS", logutil.Fields{
				"subject": subject,
				"error":   err.Error(),
			})
			return fmt.Errorf("failed to publish to subject %s: %w", subject, err)
		}
		a.logger.Debug("Message published successfully", logutil.Fields{
			"subject": subject,
		})
		return nil
	}
}

// PublishJSON publishes a JSON-serializable object to the subject
func (a *Adapter) PublishJSON(ctx context.Context, subject string, obj interface{}) error {
	data, err := json.Marshal(obj)
	if err != nil {
		a.logger.Error("Failed to marshal object", logutil.Fields{
			"subject": subject,
			"error":   err.Error(),
		})
		return fmt.Errorf("failed to marshal object for subject %s: %w", subject, err)
	}

	return a.Publish(ctx, subject, data)
}

// Subscribe listens for messages on the specified subject
func (a *Adapter) Subscribe(ctx context.Context, subject string, handler ports.MessageHandler) error {
	a.subsMutex.Lock()
	defer a.subsMutex.Unlock()

	a.logger.Info("Creating subscription", logutil.Fields{
		"subject": subject,
	})

	// Check if already subscribed
	if _, exists := a.subs[subject]; exists {
		a.logger.Warn("Already subscribed to subject", logutil.Fields{
			"subject": subject,
		})
		return fmt.Errorf("already subscribed to subject: %s", subject)
	}

	// Create message handler wrapper
	msgHandler := func(msg *nats.Msg) {
		if err := handler(ctx, msg.Subject, msg.Data); err != nil {
			a.logger.Error("Message handler error", logutil.Fields{
				"subject": msg.Subject,
				"error":   err.Error(),
			})
		}
	}

	var sub *nats.Subscription
	var err error

	if a.js != nil {
		// Use JetStream subscription for durability
		durableName := fmt.Sprintf("%s_%s", constants.ServiceName, sanitizeSubjectForDurable(subject))
		sub, err = a.js.Subscribe(subject, msgHandler,
			nats.Durable(durableName),
			nats.DeliverAll(),
			nats.AckExplicit(),
		)
	} else {
		// Use core NATS subscription
		sub, err = a.conn.Subscribe(subject, msgHandler)
	}

	if err != nil {
		return fmt.Errorf("failed to subscribe to subject %s: %w", subject, err)
	}

	a.subs[subject] = sub
	return nil
}

// SubscribeQueue creates a queue subscription for load balancing
func (a *Adapter) SubscribeQueue(ctx context.Context, subject, queue string, handler ports.MessageHandler) error {
	a.subsMutex.Lock()
	defer a.subsMutex.Unlock()

	key := fmt.Sprintf("%s:%s", subject, queue)

	// Check if already subscribed
	if _, exists := a.subs[key]; exists {
		return fmt.Errorf("already subscribed to subject %s with queue %s", subject, queue)
	}

	// Create message handler wrapper
	msgHandler := func(msg *nats.Msg) {
		if err := handler(ctx, msg.Subject, msg.Data); err != nil {
			// Log error but don't fail subscription
			fmt.Printf("Queue handler error for subject %s queue %s: %v\n", msg.Subject, queue, err)
		}
	}

	var sub *nats.Subscription
	var err error

	if a.js != nil {
		// Use JetStream queue subscription
		sub, err = a.js.QueueSubscribe(subject, queue, msgHandler,
			nats.Durable(fmt.Sprintf("hexarag_%s_%s", sanitizeSubjectForDurable(subject), queue)),
			nats.DeliverAll(),
			nats.AckExplicit(),
		)
	} else {
		// Use core NATS queue subscription
		sub, err = a.conn.QueueSubscribe(subject, queue, msgHandler)
	}

	if err != nil {
		return fmt.Errorf("failed to subscribe to subject %s with queue %s: %w", subject, queue, err)
	}

	a.subs[key] = sub
	return nil
}

// Unsubscribe stops listening to a subject
func (a *Adapter) Unsubscribe(ctx context.Context, subject string) error {
	a.subsMutex.Lock()
	defer a.subsMutex.Unlock()

	sub, exists := a.subs[subject]
	if !exists {
		return fmt.Errorf("not subscribed to subject: %s", subject)
	}

	if err := sub.Unsubscribe(); err != nil {
		return fmt.Errorf("failed to unsubscribe from subject %s: %w", subject, err)
	}

	delete(a.subs, subject)
	return nil
}

// Request sends a request and waits for a response
func (a *Adapter) Request(ctx context.Context, subject string, data []byte, timeout ...interface{}) ([]byte, error) {
	// Default timeout
	requestTimeout := 10 * time.Second

	// Override timeout if provided
	if len(timeout) > 0 {
		if t, ok := timeout[0].(time.Duration); ok {
			requestTimeout = t
		}
	}

	// Create a context with timeout
	reqCtx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	// Send request
	msg, err := a.conn.RequestWithContext(reqCtx, subject, data)
	if err != nil {
		return nil, fmt.Errorf("failed to send request to subject %s: %w", subject, err)
	}

	return msg.Data, nil
}

// Close closes the messaging connection
func (a *Adapter) Close() error {
	a.subsMutex.Lock()
	defer a.subsMutex.Unlock()

	// Unsubscribe from all subscriptions
	for subject, sub := range a.subs {
		if err := sub.Unsubscribe(); err != nil {
			fmt.Printf("Error unsubscribing from %s: %v\n", subject, err)
		}
	}

	// Clear subscriptions map
	a.subs = make(map[string]*nats.Subscription)

	// Close connection
	if a.conn != nil {
		a.conn.Close()
	}

	return nil
}

// Ping checks messaging connectivity
func (a *Adapter) Ping() error {
	if a.conn == nil {
		return fmt.Errorf("connection is nil")
	}

	if !a.conn.IsConnected() {
		return fmt.Errorf("NATS connection is not active")
	}

	// Test with a simple RTT
	rtt, err := a.conn.RTT()
	if err != nil {
		return fmt.Errorf("failed to get RTT: %w", err)
	}

	if rtt > 5*time.Second {
		return fmt.Errorf("high latency detected: %v", rtt)
	}

	return nil
}

// GetConnectionStatus returns detailed connection information
func (a *Adapter) GetConnectionStatus() map[string]interface{} {
	status := make(map[string]interface{})

	if a.conn == nil {
		status["connected"] = false
		status["error"] = "connection is nil"
		return status
	}

	status["connected"] = a.conn.IsConnected()
	status["url"] = a.conn.ConnectedUrl()
	status["server_id"] = a.conn.ConnectedServerId()
	status["server_name"] = a.conn.ConnectedServerName()

	stats := a.conn.Stats()
	status["messages_in"] = stats.InMsgs
	status["messages_out"] = stats.OutMsgs
	status["bytes_in"] = stats.InBytes
	status["bytes_out"] = stats.OutBytes
	status["reconnects"] = stats.Reconnects

	if a.js != nil {
		status["jetstream_enabled"] = true
		// Could add JetStream-specific stats here
	} else {
		status["jetstream_enabled"] = false
	}

	a.subsMutex.RLock()
	status["active_subscriptions"] = len(a.subs)
	a.subsMutex.RUnlock()

	return status
}

// sanitizeSubjectForDurable converts a subject pattern to a valid durable name
func sanitizeSubjectForDurable(subject string) string {
	// Replace wildcards and dots with underscores for durable consumer names
	result := subject
	result = strings.Replace(result, ".", "_", -1)
	result = strings.Replace(result, "*", "star", -1)
	result = strings.Replace(result, ">", "gt", -1)
	return result
}

// Helper method to format subjects with parameters
func FormatSubject(template string, params ...interface{}) string {
	return fmt.Sprintf(template, params...)
}
