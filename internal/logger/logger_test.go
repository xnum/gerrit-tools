package logger

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestNewLogger(t *testing.T) {
	l, err := NewLogger(true, "")
	if err != nil {
		t.Fatalf("NewLogger() failed: %v", err)
	}

	if !l.verbose {
		t.Error("Expected verbose to be true")
	}

	if l.logger == nil {
		t.Error("Expected logger to be initialized")
	}
}

func TestNewLogger_WithLogFile(t *testing.T) {
	tmpDir := t.TempDir()
	logFile := filepath.Join(tmpDir, "test.log")

	l, err := NewLogger(true, logFile)
	if err != nil {
		t.Fatalf("NewLogger() failed: %v", err)
	}
	defer l.Close()

	if l.logFile == nil {
		t.Error("Expected log file to be opened")
	}

	// Verify file was created
	if _, err := os.Stat(logFile); os.IsNotExist(err) {
		t.Error("Expected log file to be created")
	}
}

func TestLogger_Info(t *testing.T) {
	tmpDir := t.TempDir()
	logFile := filepath.Join(tmpDir, "test.log")

	l, err := NewLogger(true, logFile)
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()

	l.Info("Test message")

	// Read log file
	content, err := os.ReadFile(logFile)
	if err != nil {
		t.Fatal(err)
	}

	contentStr := string(content)
	if len(contentStr) == 0 {
		t.Error("Expected log file to contain content")
	}

	if !contains(contentStr, "[INFO]") {
		t.Error("Expected log to contain [INFO] prefix")
	}

	if !contains(contentStr, "Test message") {
		t.Error("Expected log to contain message")
	}
}

func TestLogger_Error(t *testing.T) {
	tmpDir := t.TempDir()
	logFile := filepath.Join(tmpDir, "test.log")

	l, err := NewLogger(false, logFile)
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()

	l.Error("Error message")

	content, err := os.ReadFile(logFile)
	if err != nil {
		t.Fatal(err)
	}

	contentStr := string(content)
	if !contains(contentStr, "[ERROR]") {
		t.Error("Expected log to contain [ERROR] prefix")
	}
}

func TestLogger_Debug(t *testing.T) {
	tmpDir := t.TempDir()
	logFile := filepath.Join(tmpDir, "test.log")

	// Test with verbose=true
	l, err := NewLogger(true, logFile)
	if err != nil {
		t.Fatal(err)
	}

	l.Debug("Debug message")
	l.Close()

	content, err := os.ReadFile(logFile)
	if err != nil {
		t.Fatal(err)
	}

	if !contains(string(content), "Debug message") {
		t.Error("Expected debug message to be logged when verbose=true")
	}

	// Test with verbose=false
	os.Remove(logFile)
	l2, err := NewLogger(false, logFile)
	if err != nil {
		t.Fatal(err)
	}

	l2.Debug("Debug message")
	l2.Close()

	content2, err := os.ReadFile(logFile)
	if err != nil {
		t.Fatal(err)
	}

	if contains(string(content2), "Debug message") {
		t.Error("Expected debug message NOT to be logged when verbose=false")
	}
}

func TestStep_Complete(t *testing.T) {
	tmpDir := t.TempDir()
	logFile := filepath.Join(tmpDir, "test.log")

	l, err := NewLogger(true, logFile)
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()

	step := l.Step("Test step")
	time.Sleep(10 * time.Millisecond)
	step.Complete()

	content, err := os.ReadFile(logFile)
	if err != nil {
		t.Fatal(err)
	}

	contentStr := string(content)
	if !contains(contentStr, "Test step completed") {
		t.Error("Expected step completion message")
	}
}

func TestStep_Fail(t *testing.T) {
	tmpDir := t.TempDir()
	logFile := filepath.Join(tmpDir, "test.log")

	l, err := NewLogger(true, logFile)
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()

	step := l.Step("Test step")
	step.Fail(os.ErrNotExist)

	content, err := os.ReadFile(logFile)
	if err != nil {
		t.Fatal(err)
	}

	contentStr := string(content)
	if !contains(contentStr, "Test step failed") {
		t.Error("Expected step failure message")
	}
	if !contains(contentStr, "[ERROR]") {
		t.Error("Expected ERROR prefix for failed step")
	}
}

func TestLogger_Close(t *testing.T) {
	tmpDir := t.TempDir()
	logFile := filepath.Join(tmpDir, "test.log")

	l, err := NewLogger(true, logFile)
	if err != nil {
		t.Fatal(err)
	}

	// Close should not error
	if err := l.Close(); err != nil {
		t.Errorf("Close() failed: %v", err)
	}

	// Double close should not error
	if err := l.Close(); err != nil {
		t.Errorf("Second Close() should not error")
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && containsHelper(s, substr)
}

func containsHelper(s, substr string) bool {
	if len(substr) > len(s) {
		return false
	}
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
