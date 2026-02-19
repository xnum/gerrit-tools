package cli

import (
	"encoding/json"
	"testing"
	"time"
)

func TestJSONFormatter(t *testing.T) {
	formatter := &JSONFormatter{Pretty: false}

	tests := []struct {
		name     string
		response *Response
		wantErr  bool
	}{
		{
			name: "success response",
			response: &Response{
				Success: true,
				Data:    map[string]string{"key": "value"},
				Metadata: ResponseMetadata{
					Timestamp:  time.Now(),
					DurationMs: 100,
				},
			},
			wantErr: false,
		},
		{
			name: "error response",
			response: &Response{
				Success: false,
				Error: &ErrorInfo{
					Message: "test error",
					Code:    "TEST_ERROR",
				},
				Metadata: ResponseMetadata{
					Timestamp: time.Now(),
				},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			output, err := formatter.Format(tt.response)
			if (err != nil) != tt.wantErr {
				t.Errorf("Format() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			// Verify it's valid JSON
			var result map[string]interface{}
			if err := json.Unmarshal([]byte(output), &result); err != nil {
				t.Errorf("Output is not valid JSON: %v", err)
			}

			// Verify success field
			if success, ok := result["success"].(bool); !ok || success != tt.response.Success {
				t.Errorf("success field mismatch: got %v, want %v", success, tt.response.Success)
			}
		})
	}
}

func TestTextFormatter(t *testing.T) {
	formatter := &TextFormatter{}

	tests := []struct {
		name     string
		response *Response
		wantErr  bool
	}{
		{
			name: "success with string data",
			response: &Response{
				Success: true,
				Data:    "test output",
				Metadata: ResponseMetadata{
					Timestamp: time.Now(),
				},
			},
			wantErr: false,
		},
		{
			name: "error response",
			response: &Response{
				Success: false,
				Error: &ErrorInfo{
					Message: "test error",
					Code:    "TEST_ERROR",
				},
				Metadata: ResponseMetadata{
					Timestamp: time.Now(),
				},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			output, err := formatter.Format(tt.response)
			if (err != nil) != tt.wantErr {
				t.Errorf("Format() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if output == "" {
				t.Error("Output is empty")
			}
		})
	}
}

func TestNewFormatter(t *testing.T) {
	tests := []struct {
		name       string
		format     string
		wantType   string
	}{
		{
			name:     "json formatter",
			format:   "json",
			wantType: "*cli.JSONFormatter",
		},
		{
			name:     "text formatter",
			format:   "text",
			wantType: "*cli.TextFormatter",
		},
		{
			name:     "default to json",
			format:   "unknown",
			wantType: "*cli.JSONFormatter",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			formatter := NewFormatter(tt.format, true)
			if formatter == nil {
				t.Error("NewFormatter returned nil")
			}
		})
	}
}
