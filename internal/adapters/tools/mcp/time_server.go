package mcp

import (
	"context"
	"fmt"
	"time"

	"github.com/username/hexarag/internal/domain/ports"
)

// TimeServerAdapter implements a simple MCP-compatible time server
type TimeServerAdapter struct {
	enabled   bool
	timezones []string
}

// NewTimeServerAdapter creates a new MCP time server adapter
func NewTimeServerAdapter(enabled bool, timezones []string) *TimeServerAdapter {
	if len(timezones) == 0 {
		timezones = []string{"UTC"}
	}

	return &TimeServerAdapter{
		enabled:   enabled,
		timezones: timezones,
	}
}

// Execute runs a tool with the given arguments
func (t *TimeServerAdapter) Execute(ctx context.Context, name string, arguments map[string]interface{}) (*ports.ToolResult, error) {
	if !t.enabled {
		return &ports.ToolResult{
			Success: false,
			Error:   "MCP time server is disabled",
		}, nil
	}

	switch name {
	case "get_current_time":
		return t.getCurrentTime(arguments)
	case "get_time_in_timezone":
		return t.getTimeInTimezone(arguments)
	case "list_supported_timezones":
		return t.listSupportedTimezones()
	default:
		return &ports.ToolResult{
			Success: false,
			Error:   fmt.Sprintf("unknown tool: %s", name),
		}, nil
	}
}

// GetAvailableTools returns the list of available tools
func (t *TimeServerAdapter) GetAvailableTools(ctx context.Context) ([]ports.Tool, error) {
	if !t.enabled {
		return []ports.Tool{}, nil
	}

	tools := []ports.Tool{
		{
			Type: "function",
			Function: ports.ToolFunction{
				Name:        "get_current_time",
				Description: "Get the current local system time",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"format": map[string]interface{}{
							"type":        "string",
							"description": "Time format (optional): 'iso', 'unix', 'human'",
							"enum":        []string{"iso", "unix", "human"},
							"default":     "iso",
						},
					},
				},
			},
		},
		{
			Type: "function",
			Function: ports.ToolFunction{
				Name:        "get_time_in_timezone",
				Description: "Get the current time in a specific timezone",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"timezone": map[string]interface{}{
							"type":        "string",
							"description": "Timezone name (e.g., 'America/New_York', 'Europe/London', 'UTC')",
						},
						"format": map[string]interface{}{
							"type":        "string",
							"description": "Time format (optional): 'iso', 'unix', 'human'",
							"enum":        []string{"iso", "unix", "human"},
							"default":     "iso",
						},
					},
					"required": []string{"timezone"},
				},
			},
		},
		{
			Type: "function",
			Function: ports.ToolFunction{
				Name:        "list_supported_timezones",
				Description: "List all supported timezones configured for this server",
				Parameters: map[string]interface{}{
					"type":       "object",
					"properties": map[string]interface{}{},
				},
			},
		},
	}

	return tools, nil
}

// GetTool returns information about a specific tool
func (t *TimeServerAdapter) GetTool(ctx context.Context, name string) (*ports.Tool, error) {
	tools, err := t.GetAvailableTools(ctx)
	if err != nil {
		return nil, err
	}

	for _, tool := range tools {
		if tool.Function.Name == name {
			return &tool, nil
		}
	}

	return nil, fmt.Errorf("tool not found: %s", name)
}

// Ping checks tool connectivity
func (t *TimeServerAdapter) Ping(ctx context.Context) error {
	if !t.enabled {
		return fmt.Errorf("MCP time server is disabled")
	}
	return nil
}

// Tool implementation methods

func (t *TimeServerAdapter) getCurrentTime(arguments map[string]interface{}) (*ports.ToolResult, error) {
	format := t.getFormat(arguments)
	now := time.Now()

	timeData := map[string]interface{}{
		"timestamp":  t.formatTime(now, format),
		"timezone":   now.Location().String(),
		"utc_offset": now.Format("-07:00"),
	}

	return &ports.ToolResult{
		Success: true,
		Data:    timeData,
		Metadata: map[string]interface{}{
			"tool":   "get_current_time",
			"format": format,
		},
	}, nil
}

func (t *TimeServerAdapter) getTimeInTimezone(arguments map[string]interface{}) (*ports.ToolResult, error) {
	timezone, ok := arguments["timezone"].(string)
	if !ok {
		return &ports.ToolResult{
			Success: false,
			Error:   "timezone parameter is required and must be a string",
		}, nil
	}

	// Check if timezone is in our allowed list
	if !t.isTimezoneSupported(timezone) {
		return &ports.ToolResult{
			Success: false,
			Error:   fmt.Sprintf("timezone '%s' is not supported. Use list_supported_timezones to see available options", timezone),
		}, nil
	}

	location, err := time.LoadLocation(timezone)
	if err != nil {
		return &ports.ToolResult{
			Success: false,
			Error:   fmt.Sprintf("invalid timezone: %s", timezone),
		}, nil
	}

	format := t.getFormat(arguments)
	now := time.Now().In(location)

	timeData := map[string]interface{}{
		"timestamp":  t.formatTime(now, format),
		"timezone":   timezone,
		"utc_offset": now.Format("-07:00"),
		"is_dst":     now.IsDST(),
	}

	return &ports.ToolResult{
		Success: true,
		Data:    timeData,
		Metadata: map[string]interface{}{
			"tool":     "get_time_in_timezone",
			"timezone": timezone,
			"format":   format,
		},
	}, nil
}

func (t *TimeServerAdapter) listSupportedTimezones() (*ports.ToolResult, error) {
	timezoneData := map[string]interface{}{
		"timezones": t.timezones,
		"count":     len(t.timezones),
	}

	return &ports.ToolResult{
		Success: true,
		Data:    timezoneData,
		Metadata: map[string]interface{}{
			"tool": "list_supported_timezones",
		},
	}, nil
}

// Helper methods

func (t *TimeServerAdapter) getFormat(arguments map[string]interface{}) string {
	if format, ok := arguments["format"].(string); ok {
		switch format {
		case "iso", "unix", "human":
			return format
		}
	}
	return "iso" // default
}

func (t *TimeServerAdapter) formatTime(timestamp time.Time, format string) interface{} {
	switch format {
	case "unix":
		return timestamp.Unix()
	case "human":
		return timestamp.Format("Monday, January 2, 2006 at 3:04:05 PM MST")
	default: // iso
		return timestamp.Format(time.RFC3339)
	}
}

func (t *TimeServerAdapter) isTimezoneSupported(timezone string) bool {
	for _, tz := range t.timezones {
		if tz == timezone {
			return true
		}
	}
	return false
}

// GetStatus returns the current status of the time server
func (t *TimeServerAdapter) GetStatus() map[string]interface{} {
	status := map[string]interface{}{
		"enabled":             t.enabled,
		"supported_timezones": t.timezones,
		"timezone_count":      len(t.timezones),
	}

	if t.enabled {
		status["server_time"] = time.Now().Format(time.RFC3339)
		status["server_timezone"] = time.Now().Location().String()
	}

	return status
}
