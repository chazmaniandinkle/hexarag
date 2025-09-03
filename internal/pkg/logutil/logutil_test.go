package logutil

import (
	"bytes"
	"log"
	"strings"
	"testing"
)

func TestLogLevel_String(t *testing.T) {
	tests := []struct {
		level    LogLevel
		expected string
	}{
		{DEBUG, "DEBUG"},
		{INFO, "INFO"},
		{WARN, "WARN"},
		{ERROR, "ERROR"},
		{FATAL, "FATAL"},
		{LogLevel(99), "UNKNOWN"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			result := tt.level.String()
			if result != tt.expected {
				t.Errorf("Expected %s, got %s", tt.expected, result)
			}
		})
	}
}

func TestNewLogger(t *testing.T) {
	config := LogConfig{
		Level:       DEBUG,
		Format:      "text",
		ServiceName: "test-service",
		AddCaller:   true,
	}

	logger := NewLogger(config)

	if logger.config.Level != DEBUG {
		t.Errorf("Expected level DEBUG, got %s", logger.config.Level.String())
	}

	if logger.config.Format != "text" {
		t.Errorf("Expected format 'text', got %s", logger.config.Format)
	}

	if logger.config.ServiceName != "test-service" {
		t.Errorf("Expected service name 'test-service', got %s", logger.config.ServiceName)
	}
}

func TestNewDefaultLogger(t *testing.T) {
	logger := NewDefaultLogger()

	if logger.config.Level != DefaultLogConfig.Level {
		t.Errorf("Expected default level %s, got %s", DefaultLogConfig.Level.String(), logger.config.Level.String())
	}

	if logger.config.ServiceName != DefaultLogConfig.ServiceName {
		t.Errorf("Expected default service name %s, got %s", DefaultLogConfig.ServiceName, logger.config.ServiceName)
	}
}

func TestLogger_ShouldLog(t *testing.T) {
	tests := []struct {
		name        string
		configLevel LogLevel
		logLevel    LogLevel
		expected    bool
	}{
		{
			name:        "debug_config_debug_log",
			configLevel: DEBUG,
			logLevel:    DEBUG,
			expected:    true,
		},
		{
			name:        "info_config_debug_log",
			configLevel: INFO,
			logLevel:    DEBUG,
			expected:    false,
		},
		{
			name:        "info_config_error_log",
			configLevel: INFO,
			logLevel:    ERROR,
			expected:    true,
		},
		{
			name:        "error_config_warn_log",
			configLevel: ERROR,
			logLevel:    WARN,
			expected:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := LogConfig{
				Level:       tt.configLevel,
				Format:      "text",
				ServiceName: "test",
			}
			logger := NewLogger(config)

			result := logger.shouldLog(tt.logLevel)
			if result != tt.expected {
				t.Errorf("Expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestLogger_FormatMessage_Text(t *testing.T) {
	config := LogConfig{
		Level:       INFO,
		Format:      "text",
		ServiceName: "test-service",
	}
	logger := NewLogger(config)

	fields := Fields{
		"key1": "value1",
		"key2": 42,
	}

	result := logger.formatMessage(INFO, "test message", fields)

	// Check if result contains expected components
	if !strings.Contains(result, "INFO") {
		t.Error("Formatted message should contain log level")
	}

	if !strings.Contains(result, "test-service") {
		t.Error("Formatted message should contain service name")
	}

	if !strings.Contains(result, "test message") {
		t.Error("Formatted message should contain the message")
	}

	if !strings.Contains(result, "key1=value1") {
		t.Error("Formatted message should contain fields")
	}

	if !strings.Contains(result, "key2=42") {
		t.Error("Formatted message should contain fields")
	}
}

func TestLogger_FormatMessage_JSON(t *testing.T) {
	config := LogConfig{
		Level:       INFO,
		Format:      "json",
		ServiceName: "test-service",
	}
	logger := NewLogger(config)

	fields := Fields{
		"key1": "value1",
	}

	result := logger.formatMessage(INFO, "test message", fields)

	// Check if result contains expected JSON components
	if !strings.Contains(result, `"level":"INFO"`) {
		t.Error("JSON formatted message should contain log level")
	}

	if !strings.Contains(result, `"service":"test-service"`) {
		t.Error("JSON formatted message should contain service name")
	}

	if !strings.Contains(result, `"message":"test message"`) {
		t.Error("JSON formatted message should contain the message")
	}

	if !strings.Contains(result, `"key1":"value1"`) {
		t.Error("JSON formatted message should contain fields")
	}
}

func TestLogger_LogMethods(t *testing.T) {
	// Capture log output
	var buf bytes.Buffer

	config := LogConfig{
		Level:       DEBUG,
		Format:      "text",
		ServiceName: "test",
	}
	logger := NewLogger(config)
	logger.logger = log.New(&buf, "", 0)

	tests := []struct {
		name   string
		method func(string, ...Fields)
		level  string
	}{
		{
			name:   "debug",
			method: logger.Debug,
			level:  "DEBUG",
		},
		{
			name:   "info",
			method: logger.Info,
			level:  "INFO",
		},
		{
			name:   "warn",
			method: logger.Warn,
			level:  "WARN",
		},
		{
			name:   "error",
			method: logger.Error,
			level:  "ERROR",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buf.Reset()
			tt.method("test message", Fields{"key": "value"})

			output := buf.String()
			if !strings.Contains(output, tt.level) {
				t.Errorf("Output should contain level %s, got: %s", tt.level, output)
			}

			if !strings.Contains(output, "test message") {
				t.Errorf("Output should contain message, got: %s", output)
			}
		})
	}
}

func TestLogger_WithFields(t *testing.T) {
	logger := NewDefaultLogger()
	fields := Fields{
		"component": "test",
		"user_id":   123,
	}

	fieldLogger := logger.WithFields(fields)

	if fieldLogger.fields["component"] != "test" {
		t.Error("Field logger should contain pre-set fields")
	}

	if fieldLogger.fields["user_id"] != 123 {
		t.Error("Field logger should contain pre-set fields")
	}
}

func TestFieldLogger_MergeFields(t *testing.T) {
	logger := NewDefaultLogger()
	presetFields := Fields{
		"component": "test",
		"user_id":   123,
	}

	fieldLogger := logger.WithFields(presetFields)

	newFields := Fields{
		"request_id": "abc123",
		"user_id":    456, // This should override the preset value
	}

	merged := fieldLogger.mergeFields(newFields)

	if merged["component"] != "test" {
		t.Error("Merged fields should contain preset component")
	}

	if merged["user_id"] != 456 {
		t.Error("Merged fields should override preset values with new values")
	}

	if merged["request_id"] != "abc123" {
		t.Error("Merged fields should contain new fields")
	}
}

func TestFieldLogger_LogMethods(t *testing.T) {
	// Capture log output
	var buf bytes.Buffer

	config := LogConfig{
		Level:       DEBUG,
		Format:      "text",
		ServiceName: "test",
	}
	logger := NewLogger(config)
	logger.logger = log.New(&buf, "", 0)

	presetFields := Fields{
		"component": "auth",
	}

	fieldLogger := logger.WithFields(presetFields)

	// Test that preset fields are included
	fieldLogger.Info("test message", Fields{"request_id": "123"})

	output := buf.String()
	if !strings.Contains(output, "component=auth") {
		t.Error("Output should contain preset fields")
	}

	if !strings.Contains(output, "request_id=123") {
		t.Error("Output should contain additional fields")
	}

	if !strings.Contains(output, "test message") {
		t.Error("Output should contain the message")
	}
}

func TestGlobalLoggerFunctions(t *testing.T) {
	// Set up a custom global logger for testing
	var buf bytes.Buffer
	config := LogConfig{
		Level:       DEBUG,
		Format:      "text",
		ServiceName: "global-test",
	}
	testLogger := NewLogger(config)
	testLogger.logger = log.New(&buf, "", 0)
	SetGlobalLogger(testLogger)

	// Test global logging functions
	Info("global test message", Fields{"global": true})

	output := buf.String()
	if !strings.Contains(output, "global test message") {
		t.Error("Global logger should output the message")
	}

	if !strings.Contains(output, "global=true") {
		t.Error("Global logger should include fields")
	}

	if !strings.Contains(output, "INFO") {
		t.Error("Global logger should include log level")
	}
}

func TestLogger_LevelFiltering(t *testing.T) {
	var buf bytes.Buffer

	// Create logger with WARN level
	config := LogConfig{
		Level:       WARN,
		Format:      "text",
		ServiceName: "test",
	}
	logger := NewLogger(config)
	logger.logger = log.New(&buf, "", 0)

	// DEBUG and INFO should be filtered out
	logger.Debug("debug message")
	logger.Info("info message")

	if buf.Len() > 0 {
		t.Error("DEBUG and INFO messages should be filtered out when level is WARN")
	}

	// WARN and ERROR should pass through
	logger.Warn("warn message")
	output := buf.String()

	if !strings.Contains(output, "warn message") {
		t.Error("WARN message should pass through when level is WARN")
	}

	buf.Reset()
	logger.Error("error message")
	output = buf.String()

	if !strings.Contains(output, "error message") {
		t.Error("ERROR message should pass through when level is WARN")
	}
}

// Note: We skip testing the Fatal method as it calls os.Exit(1)
// which would terminate the test runner. In a real implementation,
// you might want to make this behavior configurable for testing.

func TestFields_Type(t *testing.T) {
	fields := Fields{
		"string_field": "value",
		"int_field":    42,
		"bool_field":   true,
		"float_field":  3.14,
	}

	// Test that different types can be stored
	if fields["string_field"] != "value" {
		t.Error("Fields should store string values")
	}

	if fields["int_field"] != 42 {
		t.Error("Fields should store int values")
	}

	if fields["bool_field"] != true {
		t.Error("Fields should store bool values")
	}

	if fields["float_field"] != 3.14 {
		t.Error("Fields should store float values")
	}
}
