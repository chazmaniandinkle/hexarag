package httputil

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// StandardResponse represents a consistent API response structure
type StandardResponse struct {
	Success bool        `json:"success"`
	Data    interface{} `json:"data,omitempty"`
	Error   string      `json:"error,omitempty"`
	Meta    interface{} `json:"meta,omitempty"`
}

// ErrorResponse sends a standardized error response
func ErrorResponse(c *gin.Context, status int, err error) {
	c.JSON(status, StandardResponse{
		Success: false,
		Error:   err.Error(),
	})
}

// SuccessResponse sends a standardized success response
func SuccessResponse(c *gin.Context, data interface{}) {
	c.JSON(http.StatusOK, StandardResponse{
		Success: true,
		Data:    data,
	})
}

// SuccessResponseWithMeta sends a success response with metadata
func SuccessResponseWithMeta(c *gin.Context, data interface{}, meta interface{}) {
	c.JSON(http.StatusOK, StandardResponse{
		Success: true,
		Data:    data,
		Meta:    meta,
	})
}

// CreatedResponse sends a 201 Created response
func CreatedResponse(c *gin.Context, data interface{}) {
	c.JSON(http.StatusCreated, StandardResponse{
		Success: true,
		Data:    data,
	})
}

// BadRequestError sends a 400 Bad Request error
func BadRequestError(c *gin.Context, err error) {
	ErrorResponse(c, http.StatusBadRequest, err)
}

// NotFoundError sends a 404 Not Found error
func NotFoundError(c *gin.Context, err error) {
	ErrorResponse(c, http.StatusNotFound, err)
}

// InternalServerError sends a 500 Internal Server Error
func InternalServerError(c *gin.Context, err error) {
	ErrorResponse(c, http.StatusInternalServerError, err)
}

// ServiceUnavailableError sends a 503 Service Unavailable error
func ServiceUnavailableError(c *gin.Context, err error) {
	ErrorResponse(c, http.StatusServiceUnavailable, err)
}
