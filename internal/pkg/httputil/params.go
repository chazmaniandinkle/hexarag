package httputil

import (
	"fmt"
	"strconv"

	"github.com/gin-gonic/gin"
)

// ParseIntParam parses an integer parameter with a default value
func ParseIntParam(c *gin.Context, param string, defaultValue int) int {
	if value := c.Query(param); value != "" {
		if parsed, err := strconv.Atoi(value); err == nil && parsed > 0 {
			return parsed
		}
	}
	return defaultValue
}

// ParseIntParamWithRange parses an integer parameter within a specified range
func ParseIntParamWithRange(c *gin.Context, param string, defaultValue, min, max int) int {
	value := ParseIntParam(c, param, defaultValue)
	if value < min {
		return min
	}
	if value > max {
		return max
	}
	return value
}

// ParseBoolParam parses a boolean parameter with a default value
func ParseBoolParam(c *gin.Context, param string, defaultValue bool) bool {
	if value := c.Query(param); value != "" {
		if parsed, err := strconv.ParseBool(value); err == nil {
			return parsed
		}
	}
	return defaultValue
}

// PaginationParams represents standard pagination parameters
type PaginationParams struct {
	Limit  int `json:"limit"`
	Offset int `json:"offset"`
}

// DefaultPaginationConfig holds default pagination values
type PaginationConfig struct {
	DefaultLimit int
	MaxLimit     int
}

// DefaultPagination provides sensible defaults for pagination
var DefaultPagination = PaginationConfig{
	DefaultLimit: 20,
	MaxLimit:     100,
}

// ParsePaginationParams extracts pagination parameters from the request
func ParsePaginationParams(c *gin.Context) PaginationParams {
	return ParsePaginationParamsWithConfig(c, DefaultPagination)
}

// ParsePaginationParamsWithConfig extracts pagination parameters with custom config
func ParsePaginationParamsWithConfig(c *gin.Context, config PaginationConfig) PaginationParams {
	limit := ParseIntParamWithRange(c, "limit", config.DefaultLimit, 1, config.MaxLimit)
	offset := ParseIntParam(c, "offset", 0)
	if offset < 0 {
		offset = 0
	}

	return PaginationParams{
		Limit:  limit,
		Offset: offset,
	}
}

// RequiredParam extracts a required parameter and returns an error if missing
func RequiredParam(c *gin.Context, param string) (string, error) {
	value := c.Param(param)
	if value == "" {
		return "", fmt.Errorf("required parameter '%s' is missing", param)
	}
	return value, nil
}

// RequiredQueryParam extracts a required query parameter and returns an error if missing
func RequiredQueryParam(c *gin.Context, param string) (string, error) {
	value := c.Query(param)
	if value == "" {
		return "", fmt.Errorf("required query parameter '%s' is missing", param)
	}
	return value, nil
}
