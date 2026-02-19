package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"time"
)

// Response represents the standard response format for all gerrit-cli commands
type Response struct {
	Success  bool             `json:"success"`
	Data     interface{}      `json:"data,omitempty"`
	Metadata ResponseMetadata `json:"metadata"`
	Error    *ErrorInfo       `json:"error,omitempty"`
}

// ResponseMetadata contains metadata about the response
type ResponseMetadata struct {
	Timestamp  time.Time `json:"timestamp"`
	DurationMs int64     `json:"duration_ms"`
	Command    string    `json:"command,omitempty"`
	Version    string    `json:"version,omitempty"`
}

// ErrorInfo contains error information
type ErrorInfo struct {
	Message string `json:"message"`
	Code    string `json:"code"`
	Details string `json:"details,omitempty"`
}

// Formatter handles output formatting for different output types
type Formatter interface {
	Format(response *Response) (string, error)
}

// JSONFormatter formats output as JSON
type JSONFormatter struct {
	Pretty bool
}

// Format formats the response as JSON
func (f *JSONFormatter) Format(response *Response) (string, error) {
	var data []byte
	var err error

	if f.Pretty {
		data, err = json.MarshalIndent(response, "", "  ")
	} else {
		data, err = json.Marshal(response)
	}

	if err != nil {
		return "", fmt.Errorf("failed to marshal JSON: %w", err)
	}

	return string(data), nil
}

// TextFormatter formats output as human-readable text
type TextFormatter struct{}

// Format formats the response as human-readable text
func (f *TextFormatter) Format(response *Response) (string, error) {
	if !response.Success {
		return formatError(response.Error), nil
	}

	// For text format, we need to handle different data types
	// This is a simplified implementation - can be enhanced based on data type
	switch data := response.Data.(type) {
	case string:
		return data, nil
	case map[string]interface{}, []interface{}:
		// For complex data structures, fall back to pretty JSON
		jsonData, err := json.MarshalIndent(data, "", "  ")
		if err != nil {
			return "", fmt.Errorf("failed to format data: %w", err)
		}
		return string(jsonData), nil
	default:
		// Try to marshal as JSON
		jsonData, err := json.MarshalIndent(data, "", "  ")
		if err != nil {
			return fmt.Sprintf("%v", data), nil
		}
		return string(jsonData), nil
	}
}

// formatError formats an error for text output
func formatError(err *ErrorInfo) string {
	if err == nil {
		return "Unknown error"
	}

	result := fmt.Sprintf("Error [%s]: %s", err.Code, err.Message)
	if err.Details != "" {
		result += fmt.Sprintf("\nDetails: %s", err.Details)
	}
	return result
}

// NewFormatter creates a new formatter based on the format type
func NewFormatter(format string, pretty bool) Formatter {
	switch format {
	case "json":
		return &JSONFormatter{Pretty: pretty}
	case "text":
		return &TextFormatter{}
	default:
		// Default to JSON
		return &JSONFormatter{Pretty: pretty}
	}
}

// ExecuteCommand is a helper function that wraps command execution with
// standard response formatting and error handling
func ExecuteCommand(format string, command string, version string, fn func() (interface{}, error)) error {
	startTime := time.Now()

	// Log command execution start to stderr (captured by Bash tool)
	fmt.Fprintf(os.Stderr, "[gerrit-cli] Executing: %s\n", command)

	response := &Response{
		Success: true,
		Metadata: ResponseMetadata{
			Timestamp: startTime,
			Command:   command,
			Version:   version,
		},
	}

	// Execute the command function
	data, err := fn()

	// Calculate duration
	response.Metadata.DurationMs = time.Since(startTime).Milliseconds()

	if err != nil {
		response.Success = false
		response.Error = &ErrorInfo{
			Message: err.Error(),
			Code:    "COMMAND_ERROR",
		}
	} else {
		response.Data = data
	}

	// Log command completion to stderr
	fmt.Fprintf(os.Stderr, "[gerrit-cli] %s completed in %dms (success=%v)\n",
		command, response.Metadata.DurationMs, response.Success)

	// Format and output
	formatter := NewFormatter(format, true)
	output, err := formatter.Format(response)
	if err != nil {
		return fmt.Errorf("failed to format output: %w", err)
	}

	fmt.Println(output)

	// Return error if command failed (for proper exit code)
	if !response.Success {
		return fmt.Errorf("command failed")
	}

	return nil
}

// FormatErrorResponse creates and formats an error response
func FormatErrorResponse(format string, errorMsg string, errorCode string) string {
	response := &Response{
		Success: false,
		Error: &ErrorInfo{
			Message: errorMsg,
			Code:    errorCode,
		},
		Metadata: ResponseMetadata{
			Timestamp: time.Now(),
		},
	}

	formatter := NewFormatter(format, true)
	output, err := formatter.Format(response)
	if err != nil {
		// Fallback to simple error message
		return fmt.Sprintf("Error: %s", errorMsg)
	}

	return output
}
