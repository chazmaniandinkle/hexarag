package httputil

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestWithTimeout(t *testing.T) {
	duration := 5 * time.Second
	ctx, cancel := WithTimeout(duration)
	defer cancel()

	deadline, ok := ctx.Deadline()
	assert.True(t, ok, "Context should have a deadline")
	assert.True(t, time.Until(deadline) <= duration, "Deadline should be within the specified duration")
}

func TestWithDefaultTimeout(t *testing.T) {
	ctx, cancel := WithDefaultTimeout()
	defer cancel()

	deadline, ok := ctx.Deadline()
	assert.True(t, ok, "Context should have a deadline")
	assert.True(t, time.Until(deadline) <= DefaultTimeouts.Default, "Deadline should be within default timeout")
}

func TestWithShortTimeout(t *testing.T) {
	ctx, cancel := WithShortTimeout()
	defer cancel()

	deadline, ok := ctx.Deadline()
	assert.True(t, ok, "Context should have a deadline")
	assert.True(t, time.Until(deadline) <= DefaultTimeouts.Short, "Deadline should be within short timeout")
}

func TestWithLongTimeout(t *testing.T) {
	ctx, cancel := WithLongTimeout()
	defer cancel()

	deadline, ok := ctx.Deadline()
	assert.True(t, ok, "Context should have a deadline")
	assert.True(t, time.Until(deadline) <= DefaultTimeouts.Long, "Deadline should be within long timeout")
}

func TestWithCustomTimeout(t *testing.T) {
	config := TimeoutConfig{
		Default: 15 * time.Second,
		Short:   3 * time.Second,
		Long:    45 * time.Second,
	}

	tests := []struct {
		name          string
		operationType string
		expectedMax   time.Duration
	}{
		{
			name:          "storage_operation",
			operationType: "storage",
			expectedMax:   config.Default,
		},
		{
			name:          "messaging_operation",
			operationType: "messaging",
			expectedMax:   config.Short,
		},
		{
			name:          "inference_operation",
			operationType: "inference",
			expectedMax:   config.Long,
		},
		{
			name:          "unknown_operation",
			operationType: "unknown",
			expectedMax:   config.Default,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := WithCustomTimeout(tt.operationType, config)
			defer cancel()

			deadline, ok := ctx.Deadline()
			assert.True(t, ok, "Context should have a deadline")
			assert.True(t, time.Until(deadline) <= tt.expectedMax, "Deadline should be within expected timeout")
		})
	}
}

func TestParsePaginationParams(t *testing.T) {
	tests := []struct {
		name           string
		queryParams    map[string]string
		expectedLimit  int
		expectedOffset int
	}{
		{
			name:           "default_values",
			queryParams:    map[string]string{},
			expectedLimit:  DefaultPagination.DefaultLimit,
			expectedOffset: 0,
		},
		{
			name: "custom_values",
			queryParams: map[string]string{
				"limit":  "50",
				"offset": "100",
			},
			expectedLimit:  50,
			expectedOffset: 100,
		},
		{
			name: "invalid_limit",
			queryParams: map[string]string{
				"limit":  "invalid",
				"offset": "10",
			},
			expectedLimit:  DefaultPagination.DefaultLimit,
			expectedOffset: 10,
		},
		{
			name: "negative_values",
			queryParams: map[string]string{
				"limit":  "-10",
				"offset": "-5",
			},
			expectedLimit:  DefaultPagination.DefaultLimit,
			expectedOffset: 0,
		},
		{
			name: "exceeds_max_limit",
			queryParams: map[string]string{
				"limit": "1000",
			},
			expectedLimit:  DefaultPagination.MaxLimit,
			expectedOffset: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup Gin context with query parameters
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)

			req := httptest.NewRequest("GET", "/test", nil)
			q := req.URL.Query()
			for key, value := range tt.queryParams {
				q.Add(key, value)
			}
			req.URL.RawQuery = q.Encode()
			c.Request = req

			// Test parsing
			result := ParsePaginationParams(c)

			assert.Equal(t, tt.expectedLimit, result.Limit, "Limit should match expected value")
			assert.Equal(t, tt.expectedOffset, result.Offset, "Offset should match expected value")
		})
	}
}

func TestParsePaginationParamsWithConfig(t *testing.T) {
	customConfig := PaginationConfig{
		DefaultLimit: 25,
		MaxLimit:     200,
	}

	// Setup Gin context with no query parameters
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	req := httptest.NewRequest("GET", "/test", nil)
	c.Request = req

	result := ParsePaginationParamsWithConfig(c, customConfig)

	assert.Equal(t, customConfig.DefaultLimit, result.Limit, "Should use custom default limit")
	assert.Equal(t, 0, result.Offset, "Should use default offset")
}

func TestRequiredParam(t *testing.T) {
	tests := []struct {
		name        string
		paramName   string
		paramValue  string
		expectError bool
	}{
		{
			name:        "valid_param",
			paramName:   "id",
			paramValue:  "123",
			expectError: false,
		},
		{
			name:        "empty_param",
			paramName:   "id",
			paramValue:  "",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup Gin context with URL parameter
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Params = gin.Params{
				{Key: tt.paramName, Value: tt.paramValue},
			}

			result, err := RequiredParam(c, tt.paramName)

			if tt.expectError {
				assert.Error(t, err, "Should return error for invalid param")
				assert.Empty(t, result, "Result should be empty on error")
			} else {
				assert.NoError(t, err, "Should not return error for valid param")
				assert.Equal(t, tt.paramValue, result, "Result should match param value")
			}
		})
	}
}

func TestSuccessResponse(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	data := map[string]string{"message": "test"}
	SuccessResponse(c, data)

	assert.Equal(t, http.StatusOK, w.Code, "Should return 200 status")
	assert.Contains(t, w.Body.String(), "\"success\":true", "Response should contain success:true")
	assert.Contains(t, w.Body.String(), "\"message\":\"test\"", "Response should contain data")
}

func TestCreatedResponse(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	data := map[string]string{"id": "123"}
	CreatedResponse(c, data)

	assert.Equal(t, http.StatusCreated, w.Code, "Should return 201 status")
	assert.Contains(t, w.Body.String(), "\"success\":true", "Response should contain success:true")
	assert.Contains(t, w.Body.String(), "\"id\":\"123\"", "Response should contain data")
}

func TestErrorResponses(t *testing.T) {
	tests := []struct {
		name            string
		errorFunc       func(*gin.Context, error)
		expectedStatus  int
		expectedSuccess bool
	}{
		{
			name:            "bad_request_error",
			errorFunc:       BadRequestError,
			expectedStatus:  http.StatusBadRequest,
			expectedSuccess: false,
		},
		{
			name:            "not_found_error",
			errorFunc:       NotFoundError,
			expectedStatus:  http.StatusNotFound,
			expectedSuccess: false,
		},
		{
			name:            "internal_server_error",
			errorFunc:       InternalServerError,
			expectedStatus:  http.StatusInternalServerError,
			expectedSuccess: false,
		},
		{
			name:            "service_unavailable_error",
			errorFunc:       ServiceUnavailableError,
			expectedStatus:  http.StatusServiceUnavailable,
			expectedSuccess: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)

			testError := assert.AnError
			tt.errorFunc(c, testError)

			assert.Equal(t, tt.expectedStatus, w.Code, "Should return correct status code")
			assert.Contains(t, w.Body.String(), "\"success\":false", "Response should contain success:false")
			assert.Contains(t, w.Body.String(), "\"error\":", "Response should contain error field")
		})
	}
}

func TestSuccessResponseWithMeta(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	data := map[string]string{"message": "test"}
	meta := map[string]interface{}{"total": 100, "page": 1}
	SuccessResponseWithMeta(c, data, meta)

	assert.Equal(t, http.StatusOK, w.Code, "Should return 200 status")
	assert.Contains(t, w.Body.String(), "\"success\":true", "Response should contain success:true")
	assert.Contains(t, w.Body.String(), "\"message\":\"test\"", "Response should contain data")
	assert.Contains(t, w.Body.String(), "\"total\":100", "Response should contain meta data")
	assert.Contains(t, w.Body.String(), "\"page\":1", "Response should contain meta data")
}

func TestCORSMiddleware(t *testing.T) {
	// Setup middleware with custom config
	config := MiddlewareConfig{
		EnableCORS:     true,
		AllowedOrigins: []string{"https://example.com"},
		AllowedMethods: []string{"GET", "POST"},
		AllowedHeaders: []string{"Content-Type"},
	}

	middleware := CORSMiddleware(config)

	// Test OPTIONS request
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("OPTIONS", "/test", nil)

	middleware(c)

	assert.Equal(t, http.StatusNoContent, w.Code, "OPTIONS request should return 204")
	assert.Equal(t, "https://example.com", w.Header().Get("Access-Control-Allow-Origin"))
	assert.Equal(t, "GET, POST", w.Header().Get("Access-Control-Allow-Methods"))
	assert.Equal(t, "Content-Type", w.Header().Get("Access-Control-Allow-Headers"))
}

func TestTimeoutMiddleware(t *testing.T) {
	config := TimeoutConfig{
		Default: 15 * time.Second,
		Short:   3 * time.Second,
		Long:    45 * time.Second,
	}

	middleware := TimeoutMiddleware(config)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/test", nil)

	middleware(c)

	// Check if timeout config was set in context
	value, exists := c.Get(string(TimeoutConfigKey))
	assert.True(t, exists, "Timeout config should be set in context")

	timeoutConfig, ok := value.(TimeoutConfig)
	assert.True(t, ok, "Value should be TimeoutConfig type")
	assert.Equal(t, config.Default, timeoutConfig.Default, "Config should match")
}

func TestGetTimeoutForOperation(t *testing.T) {
	config := TimeoutConfig{
		Default: 15 * time.Second,
		Short:   3 * time.Second,
		Long:    45 * time.Second,
	}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Set(string(TimeoutConfigKey), config)

	tests := []struct {
		operationType string
		expected      time.Duration
	}{
		{"storage", config.Default},
		{"messaging", config.Short},
		{"inference", config.Long},
		{"unknown", config.Default},
	}

	for _, tt := range tests {
		t.Run(tt.operationType, func(t *testing.T) {
			timeout := GetTimeoutForOperation(c, tt.operationType)
			assert.Equal(t, tt.expected, timeout, "Should return correct timeout for operation type")
		})
	}
}

func TestWithOperationContext(t *testing.T) {
	config := TimeoutConfig{
		Default: 15 * time.Second,
		Short:   3 * time.Second,
		Long:    45 * time.Second,
	}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Set(string(TimeoutConfigKey), config)

	ctx, cancel := WithOperationContext(c, "storage")
	defer cancel()

	deadline, ok := ctx.Deadline()
	assert.True(t, ok, "Context should have a deadline")
	assert.True(t, time.Until(deadline) <= config.Default, "Deadline should be within expected timeout")
}
