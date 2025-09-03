package configutil

import (
	"fmt"
	"time"
)

// ValidationError represents a configuration validation error
type ValidationError struct {
	Field   string
	Message string
}

func (e ValidationError) Error() string {
	return fmt.Sprintf("validation error for field '%s': %s", e.Field, e.Message)
}

// ValidationErrors holds multiple validation errors
type ValidationErrors []ValidationError

func (e ValidationErrors) Error() string {
	if len(e) == 0 {
		return "no validation errors"
	}
	if len(e) == 1 {
		return e[0].Error()
	}
	return fmt.Sprintf("multiple validation errors: %d errors found", len(e))
}

// Validator provides configuration validation utilities
type Validator struct {
	errors []ValidationError
}

// NewValidator creates a new configuration validator
func NewValidator() *Validator {
	return &Validator{
		errors: make([]ValidationError, 0),
	}
}

// RequiredString validates that a string field is not empty
func (v *Validator) RequiredString(field, value string) *Validator {
	if value == "" {
		v.errors = append(v.errors, ValidationError{
			Field:   field,
			Message: "is required and cannot be empty",
		})
	}
	return v
}

// RequiredInt validates that an integer field is greater than zero
func (v *Validator) RequiredInt(field string, value int) *Validator {
	if value <= 0 {
		v.errors = append(v.errors, ValidationError{
			Field:   field,
			Message: "must be greater than zero",
		})
	}
	return v
}

// IntRange validates that an integer field is within a specific range
func (v *Validator) IntRange(field string, value, min, max int) *Validator {
	if value < min || value > max {
		v.errors = append(v.errors, ValidationError{
			Field:   field,
			Message: fmt.Sprintf("must be between %d and %d", min, max),
		})
	}
	return v
}

// RequiredDuration validates that a duration field is positive
func (v *Validator) RequiredDuration(field string, value time.Duration) *Validator {
	if value <= 0 {
		v.errors = append(v.errors, ValidationError{
			Field:   field,
			Message: "must be a positive duration",
		})
	}
	return v
}

// DurationRange validates that a duration field is within a specific range
func (v *Validator) DurationRange(field string, value, min, max time.Duration) *Validator {
	if value < min || value > max {
		v.errors = append(v.errors, ValidationError{
			Field:   field,
			Message: fmt.Sprintf("must be between %v and %v", min, max),
		})
	}
	return v
}

// OneOf validates that a string field is one of the allowed values
func (v *Validator) OneOf(field, value string, allowed []string) *Validator {
	for _, allowedValue := range allowed {
		if value == allowedValue {
			return v
		}
	}
	v.errors = append(v.errors, ValidationError{
		Field:   field,
		Message: fmt.Sprintf("must be one of: %v", allowed),
	})
	return v
}

// ValidateURL validates that a string is a valid URL format
func (v *Validator) ValidateURL(field, value string) *Validator {
	if value == "" {
		return v // Allow empty URLs if not required
	}
	// Basic URL validation - could be enhanced
	if len(value) < 7 || (value[:7] != "http://" && value[:8] != "https://") {
		v.errors = append(v.errors, ValidationError{
			Field:   field,
			Message: "must be a valid HTTP or HTTPS URL",
		})
	}
	return v
}

// ValidateFilePath validates that a file path is not empty and has valid format
func (v *Validator) ValidateFilePath(field, value string) *Validator {
	if value == "" {
		v.errors = append(v.errors, ValidationError{
			Field:   field,
			Message: "file path cannot be empty",
		})
		return v
	}
	// Basic path validation - could be enhanced with actual file existence check
	if len(value) < 1 {
		v.errors = append(v.errors, ValidationError{
			Field:   field,
			Message: "must be a valid file path",
		})
	}
	return v
}

// Result returns validation errors if any exist
func (v *Validator) Result() error {
	if len(v.errors) == 0 {
		return nil
	}
	return ValidationErrors(v.errors)
}

// HasErrors returns true if there are validation errors
func (v *Validator) HasErrors() bool {
	return len(v.errors) > 0
}

// ErrorCount returns the number of validation errors
func (v *Validator) ErrorCount() int {
	return len(v.errors)
}

// DefaultValues provides default configuration values to avoid hardcoding
type DefaultValues struct {
	ServerPort       int
	ServerHost       string
	DatabaseTimeout  time.Duration
	MessagingTimeout time.Duration
	InferenceTimeout time.Duration
	DefaultPageLimit int
	MaxPageLimit     int
	LogLevel         string
	CORSEnabled      bool
}

// GetDefaults returns the default configuration values
func GetDefaults() DefaultValues {
	return DefaultValues{
		ServerPort:       8080,
		ServerHost:       "0.0.0.0",
		DatabaseTimeout:  10 * time.Second,
		MessagingTimeout: 5 * time.Second,
		InferenceTimeout: 30 * time.Second,
		DefaultPageLimit: 20,
		MaxPageLimit:     100,
		LogLevel:         "info",
		CORSEnabled:      true,
	}
}

// ApplyDefaults applies default values to configuration where values are missing
func ApplyDefaults(config interface{}) interface{} {
	// This would be implemented based on specific configuration structure
	// For now, return the config as-is
	return config
}
