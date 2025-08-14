package logging

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"time"
)

// Logger provides structured logging functionality
type Logger interface {
	Debug(msg string, args ...interface{})
	Info(msg string, args ...interface{})
	Warn(msg string, args ...interface{})
	Error(msg string, args ...interface{})
	
	With(args ...interface{}) Logger
	WithContext(ctx context.Context) Logger
}

// SlogLogger wraps the standard library slog.Logger
type SlogLogger struct {
	logger *slog.Logger
}

// NewLogger creates a new logger instance
func NewLogger(output io.Writer, level slog.Level) Logger {
	opts := &slog.HandlerOptions{
		Level: level,
	}
	
	var handler slog.Handler
	if output == os.Stdout || output == os.Stderr {
		// Use text handler for console output
		handler = slog.NewTextHandler(output, opts)
	} else {
		// Use JSON handler for file output
		handler = slog.NewJSONHandler(output, opts)
	}
	
	return &SlogLogger{
		logger: slog.New(handler),
	}
}

// NewDefaultLogger creates a logger with sensible defaults
func NewDefaultLogger() Logger {
	return NewLogger(os.Stderr, slog.LevelWarn)
}

// Debug logs a debug message
func (l *SlogLogger) Debug(msg string, args ...interface{}) {
	l.logger.Debug(msg, convertArgs(args...)...)
}

// Info logs an info message
func (l *SlogLogger) Info(msg string, args ...interface{}) {
	l.logger.Info(msg, convertArgs(args...)...)
}

// Warn logs a warning message
func (l *SlogLogger) Warn(msg string, args ...interface{}) {
	l.logger.Warn(msg, convertArgs(args...)...)
}

// Error logs an error message
func (l *SlogLogger) Error(msg string, args ...interface{}) {
	l.logger.Error(msg, convertArgs(args...)...)
}

// With creates a new logger with additional context
func (l *SlogLogger) With(args ...interface{}) Logger {
	return &SlogLogger{
		logger: l.logger.With(convertArgs(args...)...),
	}
}

// WithContext creates a new logger with context
func (l *SlogLogger) WithContext(ctx context.Context) Logger {
	// Extract common context values
	var ctxArgs []interface{}
	
	if userID, ok := ctx.Value("user_id").(string); ok {
		ctxArgs = append(ctxArgs, "user_id", userID)
	}
	
	if requestID, ok := ctx.Value("request_id").(string); ok {
		ctxArgs = append(ctxArgs, "request_id", requestID)
	}
	
	if len(ctxArgs) > 0 {
		return l.With(ctxArgs...)
	}
	
	return l
}

// convertArgs converts interface{} args to slog.Attr for better type safety
func convertArgs(args ...interface{}) []interface{} {
	result := make([]interface{}, 0, len(args))
	
	for i := 0; i < len(args); i += 2 {
		if i+1 < len(args) {
			key, ok := args[i].(string)
			if !ok {
				key = fmt.Sprintf("%v", args[i])
			}
			
			value := args[i+1]
			
			// Convert special types
			switch v := value.(type) {
			case error:
				result = append(result, key, v.Error())
			case time.Time:
				result = append(result, key, v.Format(time.RFC3339))
			case time.Duration:
				result = append(result, key, v.String())
			default:
				result = append(result, key, v)
			}
		} else {
			// Odd number of args, treat as message
			result = append(result, "extra", args[i])
		}
	}
	
	return result
}

// NoOpLogger provides a logger that does nothing (for testing)
type NoOpLogger struct{}

func (NoOpLogger) Debug(msg string, args ...interface{}) {}
func (NoOpLogger) Info(msg string, args ...interface{})  {}
func (NoOpLogger) Warn(msg string, args ...interface{})  {}
func (NoOpLogger) Error(msg string, args ...interface{}) {}

func (l NoOpLogger) With(args ...interface{}) Logger {
	return l
}

func (l NoOpLogger) WithContext(ctx context.Context) Logger {
	return l
}

// NewNoOpLogger creates a no-op logger for testing
func NewNoOpLogger() Logger {
	return NoOpLogger{}
}