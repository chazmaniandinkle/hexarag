package configutil

import (
	"testing"
	"time"
)

func TestValidator_RequiredString(t *testing.T) {
	tests := []struct {
		name      string
		field     string
		value     string
		wantError bool
	}{
		{
			name:      "valid_string",
			field:     "test_field",
			value:     "valid_value",
			wantError: false,
		},
		{
			name:      "empty_string",
			field:     "test_field",
			value:     "",
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			validator := NewValidator()
			result := validator.RequiredString(tt.field, tt.value).Result()

			if tt.wantError && result == nil {
				t.Errorf("Expected error for field %s with value %s, but got none", tt.field, tt.value)
			}
			if !tt.wantError && result != nil {
				t.Errorf("Expected no error for field %s with value %s, but got: %v", tt.field, tt.value, result)
			}
		})
	}
}

func TestValidator_IntRange(t *testing.T) {
	tests := []struct {
		name      string
		field     string
		value     int
		min       int
		max       int
		wantError bool
	}{
		{
			name:      "valid_range",
			field:     "port",
			value:     8080,
			min:       1,
			max:       65535,
			wantError: false,
		},
		{
			name:      "below_min",
			field:     "port",
			value:     0,
			min:       1,
			max:       65535,
			wantError: true,
		},
		{
			name:      "above_max",
			field:     "port",
			value:     70000,
			min:       1,
			max:       65535,
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			validator := NewValidator()
			result := validator.IntRange(tt.field, tt.value, tt.min, tt.max).Result()

			if tt.wantError && result == nil {
				t.Errorf("Expected error for field %s with value %d, but got none", tt.field, tt.value)
			}
			if !tt.wantError && result != nil {
				t.Errorf("Expected no error for field %s with value %d, but got: %v", tt.field, tt.value, result)
			}
		})
	}
}

func TestValidator_OneOf(t *testing.T) {
	tests := []struct {
		name      string
		field     string
		value     string
		allowed   []string
		wantError bool
	}{
		{
			name:      "valid_option",
			field:     "log_level",
			value:     "info",
			allowed:   []string{"debug", "info", "warn", "error"},
			wantError: false,
		},
		{
			name:      "invalid_option",
			field:     "log_level",
			value:     "invalid",
			allowed:   []string{"debug", "info", "warn", "error"},
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			validator := NewValidator()
			result := validator.OneOf(tt.field, tt.value, tt.allowed).Result()

			if tt.wantError && result == nil {
				t.Errorf("Expected error for field %s with value %s, but got none", tt.field, tt.value)
			}
			if !tt.wantError && result != nil {
				t.Errorf("Expected no error for field %s with value %s, but got: %v", tt.field, tt.value, result)
			}
		})
	}
}

func TestValidator_ValidateURL(t *testing.T) {
	tests := []struct {
		name      string
		field     string
		url       string
		wantError bool
	}{
		{
			name:      "valid_http_url",
			field:     "base_url",
			url:       "http://localhost:8080",
			wantError: false,
		},
		{
			name:      "valid_https_url",
			field:     "base_url",
			url:       "https://api.example.com",
			wantError: false,
		},
		{
			name:      "invalid_url",
			field:     "base_url",
			url:       "not-a-url",
			wantError: true,
		},
		{
			name:      "empty_url",
			field:     "base_url",
			url:       "",
			wantError: false, // Empty URLs are allowed by ValidateURL
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			validator := NewValidator()
			result := validator.ValidateURL(tt.field, tt.url).Result()

			if tt.wantError && result == nil {
				t.Errorf("Expected error for field %s with URL %s, but got none", tt.field, tt.url)
			}
			if !tt.wantError && result != nil {
				t.Errorf("Expected no error for field %s with URL %s, but got: %v", tt.field, tt.url, result)
			}
		})
	}
}

func TestValidator_DurationRange(t *testing.T) {
	tests := []struct {
		name      string
		field     string
		value     time.Duration
		min       time.Duration
		max       time.Duration
		wantError bool
	}{
		{
			name:      "valid_duration",
			field:     "timeout",
			value:     10 * time.Second,
			min:       1 * time.Second,
			max:       60 * time.Second,
			wantError: false,
		},
		{
			name:      "below_min_duration",
			field:     "timeout",
			value:     500 * time.Millisecond,
			min:       1 * time.Second,
			max:       60 * time.Second,
			wantError: true,
		},
		{
			name:      "above_max_duration",
			field:     "timeout",
			value:     120 * time.Second,
			min:       1 * time.Second,
			max:       60 * time.Second,
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			validator := NewValidator()
			result := validator.DurationRange(tt.field, tt.value, tt.min, tt.max).Result()

			if tt.wantError && result == nil {
				t.Errorf("Expected error for field %s with duration %v, but got none", tt.field, tt.value)
			}
			if !tt.wantError && result != nil {
				t.Errorf("Expected no error for field %s with duration %v, but got: %v", tt.field, tt.value, result)
			}
		})
	}
}

func TestValidator_ChainedValidation(t *testing.T) {
	validator := NewValidator()
	
	// Test chaining multiple validations
	result := validator.
		RequiredString("server.host", "localhost").
		IntRange("server.port", 8080, 1, 65535).
		OneOf("log.level", "info", []string{"debug", "info", "warn", "error"}).
		ValidateURL("api.url", "https://api.example.com").
		Result()

	if result != nil {
		t.Errorf("Expected no errors from chained validation, but got: %v", result)
	}

	// Test chaining with errors
	validator2 := NewValidator()
	result2 := validator2.
		RequiredString("server.host", "").
		IntRange("server.port", 0, 1, 65535).
		OneOf("log.level", "invalid", []string{"debug", "info", "warn", "error"}).
		Result()

	if result2 == nil {
		t.Errorf("Expected errors from chained validation with invalid values, but got none")
	}

	// Check if we got multiple errors
	if validationErrors, ok := result2.(ValidationErrors); ok {
		if len(validationErrors) != 3 {
			t.Errorf("Expected 3 validation errors, but got %d", len(validationErrors))
		}
	} else {
		t.Errorf("Expected ValidationErrors type, but got %T", result2)
	}
}

func TestValidator_ErrorCount(t *testing.T) {
	validator := NewValidator()
	
	// Add some errors
	validator.RequiredString("field1", "")
	validator.IntRange("field2", 0, 1, 10)
	validator.OneOf("field3", "invalid", []string{"valid"})

	expectedCount := 3
	if validator.ErrorCount() != expectedCount {
		t.Errorf("Expected error count %d, but got %d", expectedCount, validator.ErrorCount())
	}

	if !validator.HasErrors() {
		t.Errorf("Expected HasErrors() to return true, but got false")
	}
}

func TestValidationErrors_Error(t *testing.T) {
	// Test single error
	singleError := ValidationErrors{
		ValidationError{Field: "test", Message: "is required"},
	}
	expected := "validation error for field 'test': is required"
	if singleError.Error() != expected {
		t.Errorf("Expected single error message '%s', but got '%s'", expected, singleError.Error())
	}

	// Test multiple errors
	multipleErrors := ValidationErrors{
		ValidationError{Field: "field1", Message: "is required"},
		ValidationError{Field: "field2", Message: "is invalid"},
	}
	expectedMultiple := "multiple validation errors: 2 errors found"
	if multipleErrors.Error() != expectedMultiple {
		t.Errorf("Expected multiple error message '%s', but got '%s'", expectedMultiple, multipleErrors.Error())
	}

	// Test no errors
	noErrors := ValidationErrors{}
	expectedNone := "no validation errors"
	if noErrors.Error() != expectedNone {
		t.Errorf("Expected no error message '%s', but got '%s'", expectedNone, noErrors.Error())
	}
}