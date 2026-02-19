package types

import (
	"strings"
	"testing"
)

func TestReviewResult_String(t *testing.T) {
	result := &ReviewResult{
		Summary: "Code looks good overall",
		Vote:    1,
		Comments: []Comment{
			{File: "main.go", Line: 42, Message: "Consider adding error handling"},
			{File: "utils.go", Line: 15, Message: "Good use of defer"},
		},
	}

	output := result.String()

	// Check that all expected parts are present
	if !strings.Contains(output, "Vote: +1") {
		t.Error("Expected vote to be in output")
	}
	if !strings.Contains(output, "Code looks good overall") {
		t.Error("Expected summary to be in output")
	}
	if !strings.Contains(output, "main.go:42") {
		t.Error("Expected first comment to be in output")
	}
	if !strings.Contains(output, "utils.go:15") {
		t.Error("Expected second comment to be in output")
	}
}

func TestReviewResult_VoteLabel(t *testing.T) {
	tests := []struct {
		vote     int
		expected string
	}{
		{-1, "I would prefer this is not merged as is"},
		{0, "No score"},
		{1, "Looks good to me, but someone else must approve"},
	}

	for _, tt := range tests {
		result := &ReviewResult{Vote: tt.vote}
		label := result.VoteLabel()
		if label != tt.expected {
			t.Errorf("VoteLabel() for vote %d = %q, want %q", tt.vote, label, tt.expected)
		}
	}
}

func TestReviewResult_HasComments(t *testing.T) {
	tests := []struct {
		name     string
		comments []Comment
		expected bool
	}{
		{
			name:     "no comments",
			comments: []Comment{},
			expected: false,
		},
		{
			name: "has comments",
			comments: []Comment{
				{File: "test.go", Line: 1, Message: "test"},
			},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := &ReviewResult{Comments: tt.comments}
			if result.HasComments() != tt.expected {
				t.Errorf("HasComments() = %v, want %v", result.HasComments(), tt.expected)
			}
		})
	}
}

func TestComment(t *testing.T) {
	comment := Comment{
		File:    "example.go",
		Line:    123,
		Message: "This is a test comment",
	}

	if comment.File != "example.go" {
		t.Errorf("Expected File 'example.go', got '%s'", comment.File)
	}
	if comment.Line != 123 {
		t.Errorf("Expected Line 123, got %d", comment.Line)
	}
	if comment.Message != "This is a test comment" {
		t.Errorf("Expected Message 'This is a test comment', got '%s'", comment.Message)
	}
}
