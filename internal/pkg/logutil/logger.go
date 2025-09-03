package logutil
package logutil

import (
	"fmt"
	"log"
	"os"
	"time"
)

// LogLevel represents the severity level of a log message
type LogLevel int

const (
	DEBUG LogLevel = iota
	INFO
	WARN
	ERROR
	FATAL
)

// String returns the string representation of log level
func (l LogLevel) String() string {
	switch l {
	case DEBUG:
		return "DEBUG"
	case INFO:
		return "INFO"
	case WARN:
		return "WARN"
	case ERROR:
		return "ERROR"
	case FATAL:
		return "FATAL"
	default:
		return "UNKNOWN"
	}
}

// LogConfig holds logging configuration
type LogConfig struct {
	Level       LogLevel
	Format      string // "json" or "text"
	ServiceName string
	AddCaller   bool
}

// Logger provides structured logging functionality
type Logger struct {
	config LogConfig
	logger *log.Logger
}

// DefaultLogConfig provides sensible logging defaults
var DefaultLogConfig = LogConfig{
	Level:       INFO,
	Format:      "text",
	ServiceName: "hexarag",
	AddCaller:   false,
}

// NewLogger creates a new logger with the specified configuration
func NewLogger(config LogConfig) *Logger {
	return &Logger{
		config: config,
		logger: log.New(os.Stdout, "", 0),
	}
}

// NewDefaultLogger creates a logger with default configuration
func NewDefaultLogger() *Logger {
	return NewLogger(DefaultLogConfig)
}

// Fields represents structured log fields
type Fields map[string]interface{}

// logMessage represents a structured log message
type logMessage struct {
	Timestamp string      `json:"timestamp"`
	Level     string      `json:"level"`
	Service   string      `json:"service"`
	Message   string      `json:"message"`
	Fields    Fields      `json:"fields,omitempty"`
	Caller    string      `json:"caller,omitempty"`
}

// shouldLog determines if a message should be logged based on level
func (l *Logger) shouldLog(level LogLevel) bool {
	return level >= l.config.Level
}

// formatMessage formats a log message according to the configured format
func (l *Logger) formatMessage(level LogLevel, msg string, fields Fields) string {
	timestamp := time.Now().Format(time.RFC3339)
	
	if l.config.Format == "json" {
		logMsg := logMessage{
			Timestamp: timestamp,
			Level:     level.String(),
			Service:   l.config.ServiceName,
			Message:   msg,
			Fields:    fields,
		}
		
		// Simple JSON formatting (could use json.Marshal for more complex cases)
		result := fmt.Sprintf(`{"timestamp":"%s","level":"%s","service":"%s","message":"%s"`,
			logMsg.Timestamp, logMsg.Level, logMsg.Service, logMsg.Message)
		
		if len(fields) > 0 {
			result += `,"fields":{`
			first := true
			for k, v := range fields {
				if !first {
					result += ","
				}
				result += fmt.Sprintf(`"%s":"%v"`, k, v)
				first = false
			}
			result += "}"
		}
		result += "}"
		return result
	}
	
	// Text format
	result := fmt.Sprintf("%s [%s] %s: %s", timestamp, level.String(), l.config.ServiceName, msg)
	if len(fields) > 0 {
		result += " |"
		for k, v := range fields {
			result += fmt.Sprintf(" %s=%v", k, v)
		}
	}
	return result
}

// log performs the actual logging
func (l *Logger) log(level LogLevel, msg string, fields Fields) {
	if !l.shouldLog(level) {
		return
	}
	
	formattedMsg := l.formatMessage(level, msg, fields)
	l.logger.Println(formattedMsg)
	
	if level == FATAL {
		os.Exit(1)
	}
}

// Debug logs a debug message
func (l *Logger) Debug(msg string, fields ...Fields) {
	var f Fields
	if len(fields) > 0 {
		f = fields[0]
	}
	l.log(DEBUG, msg, f)
}

// Info logs an info message
func (l *Logger) Info(msg string, fields ...Fields) {
	var f Fields
	if len(fields) > 0 {
		f = fields[0]
	}
	l.log(INFO, msg, f)
}

// Warn logs a warning message
func (l *Logger) Warn(msg string, fields ...Fields) {
	var f Fields
	if len(fields) > 0 {
		f = fields[0]
	}
	l.log(WARN, msg, f)
}

// Error logs an error message
func (l *Logger) Error(msg string, fields ...Fields) {
	var f Fields
	if len(fields) > 0 {
		f = fields[0]
	}
	l.log(ERROR, msg, f)
}

// Fatal logs a fatal message and exits
func (l *Logger) Fatal(msg string, fields ...Fields) {
	var f Fields
	if len(fields) > 0 {
		f = fields[0]
	}
	l.log(FATAL, msg, f)
}

// WithFields returns a logger with pre-set fields
func (l *Logger) WithFields(fields Fields) *FieldLogger {
	return &FieldLogger{
		logger: l,
		fields: fields,
	}
}

// FieldLogger is a logger with pre-set fields
type FieldLogger struct {
	logger *Logger
	fields Fields
}

// mergeFields merges pre-set fields with new fields
func (fl *FieldLogger) mergeFields(newFields Fields) Fields {
	merged := make(Fields)
	for k, v := range fl.fields {
		merged[k] = v
	}
	for k, v := range newFields {
		merged[k] = v
	}
	return merged
}

// Debug logs a debug message with pre-set fields
func (fl *FieldLogger) Debug(msg string, fields ...Fields) {
	var f Fields
	if len(fields) > 0 {
		f = fl.mergeFields(fields[0])
	} else {
		f = fl.fields
	}
	fl.logger.log(DEBUG, msg, f)
}

// Info logs an info message with pre-set fields
func (fl *FieldLogger) Info(msg string, fields ...Fields) {
	var f Fields
	if len(fields) > 0 {
		f = fl.mergeFields(fields[0])
	} else {
		f = fl.fields
	}
	fl.logger.log(INFO, msg, f)
}

// Warn logs a warning message with pre-set fields
func (fl *FieldLogger) Warn(msg string, fields ...Fields) {
	var f Fields
	if len(fields) > 0 {
		f = fl.mergeFields(fields[0])
	} else {
		f = fl.fields
	}
	fl.logger.log(WARN, msg, f)
}

// Error logs an error message with pre-set fields
func (fl *FieldLogger) Error(msg string, fields ...Fields) {
	var f Fields
	if len(fields) > 0 {
		f = fl.mergeFields(fields[0])
	} else {
		f = fl.fields
	}
	fl.logger.log(ERROR, msg, f)
}

// Fatal logs a fatal message with pre-set fields and exits
func (fl *FieldLogger) Fatal(msg string, fields ...Fields) {
	var f Fields
	if len(fields) > 0 {
		f = fl.mergeFields(fields[0])
	} else {
		f = fl.fields
	}
	fl.logger.log(FATAL, msg, f)
}

// Global logger instance
var globalLogger = NewDefaultLogger()

// SetGlobalLogger sets the global logger instance
func SetGlobalLogger(logger *Logger) {
	globalLogger = logger
}

// Global logging functions
func Debug(msg string, fields ...Fields) {
	globalLogger.Debug(msg, fields...)
}

func Info(msg string, fields ...Fields) {
	globalLogger.Info(msg, fields...)
}

func Warn(msg string, fields ...Fields) {
	globalLogger.Warn(msg, fields...)
}

func Error(msg string, fields ...Fields) {
	globalLogger.Error(msg, fields...)
}

func Fatal(msg string, fields ...Fields) {
	globalLogger.Fatal(msg, fields...)
}