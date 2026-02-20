package logger

import (
	"fmt"
	"io"
	"log"
	"os"
	"strings"
	"sync"
	"time"
)

var (
	globalLogger *Logger
	once         sync.Once
)

// Logger provides structured logging for the reviewer
type Logger struct {
	verbose bool
	logFile *os.File
	logger  *log.Logger
}

// NewLogger creates a new logger instance
func NewLogger(verbose bool, logFilePath string) (*Logger, error) {
	l := &Logger{
		verbose: verbose,
	}

	// Setup log file if path provided
	if logFilePath != "" {
		f, err := os.OpenFile(logFilePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
		if err != nil {
			return nil, fmt.Errorf("failed to open log file: %w", err)
		}
		l.logFile = f

		// Log to both file and stderr if verbose
		var writer io.Writer
		if verbose {
			writer = io.MultiWriter(os.Stderr, f)
		} else {
			writer = f
		}

		l.logger = log.New(writer, "", log.LstdFlags)
	} else {
		// Log only to stderr
		l.logger = log.New(os.Stderr, "", log.LstdFlags)
	}

	return l, nil
}

// Close closes the log file if open
func (l *Logger) Close() error {
	if l.logFile != nil {
		err := l.logFile.Close()
		l.logFile = nil // Prevent double close
		return err
	}
	return nil
}

// Info logs an informational message
func (l *Logger) Info(format string, args ...interface{}) {
	l.logger.Printf("[INFO] "+format, args...)
}

// Error logs an error message
func (l *Logger) Error(format string, args ...interface{}) {
	l.logger.Printf("[ERROR] "+format, args...)
}

// Debug logs a debug message (only if verbose)
func (l *Logger) Debug(format string, args ...interface{}) {
	if l.verbose {
		l.logger.Printf("[DEBUG] "+format, args...)
	}
}

// Warn logs a warning message
func (l *Logger) Warn(format string, args ...interface{}) {
	l.logger.Printf("[WARN] "+format, args...)
}

// Step logs a step in the process with timing
func (l *Logger) Step(name string) *Step {
	return &Step{
		logger:    l,
		name:      name,
		startTime: time.Now(),
	}
}

// Infof is an alias for Info (for compatibility)
func (l *Logger) Infof(format string, args ...interface{}) {
	l.Info(format, args...)
}

// Errorf is an alias for Error (for compatibility)
func (l *Logger) Errorf(format string, args ...interface{}) {
	l.Error(format, args...)
}

// Debugf is an alias for Debug (for compatibility)
func (l *Logger) Debugf(format string, args ...interface{}) {
	l.Debug(format, args...)
}

// Warnf is an alias for Warn (for compatibility)
func (l *Logger) Warnf(format string, args ...interface{}) {
	l.Warn(format, args...)
}

// Get returns the global logger instance, creating it if necessary
func Get() *Logger {
	once.Do(func() {
		globalLogger, _ = NewLogger(defaultVerboseFromEnv(), "")
	})
	return globalLogger
}

// SetGlobal sets the global logger instance
func SetGlobal(l *Logger) {
	globalLogger = l
}

// Step represents a timed step in the process
type Step struct {
	logger    *Logger
	name      string
	startTime time.Time
}

// Complete marks the step as complete and logs the duration
func (s *Step) Complete() {
	duration := time.Since(s.startTime)
	s.logger.Info("%s completed in %.2fs", s.name, duration.Seconds())
}

// Fail marks the step as failed and logs the error
func (s *Step) Fail(err error) {
	duration := time.Since(s.startTime)
	s.logger.Error("%s failed after %.2fs: %v", s.name, duration.Seconds(), err)
}

func defaultVerboseFromEnv() bool {
	if parseBoolEnv(os.Getenv("GERRIT_REVIEWER_DEBUG")) {
		return true
	}
	if parseBoolEnv(os.Getenv("LOG_VERBOSE")) {
		return true
	}

	level := strings.ToLower(strings.TrimSpace(os.Getenv("LOG_LEVEL")))
	return level == "debug" || level == "trace"
}

func parseBoolEnv(v string) bool {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}
