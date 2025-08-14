package errors

import (
	"fmt"
)

// Error types for different categories of errors
type ErrorType string

const (
	ErrorTypeValidation    ErrorType = "validation"
	ErrorTypeNotFound      ErrorType = "not_found" 
	ErrorTypeConflict      ErrorType = "conflict"
	ErrorTypeExternal      ErrorType = "external"
	ErrorTypeInternal      ErrorType = "internal"
	ErrorTypeConfiguration ErrorType = "configuration"
)

// SproutError represents an error with additional context
type SproutError struct {
	Type       ErrorType
	Message    string
	Cause      error
	Code       string
	Details    map[string]interface{}
	Retryable  bool
}

// Error implements the error interface
func (e *SproutError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("%s: %v", e.Message, e.Cause)
	}
	return e.Message
}

// Unwrap returns the underlying error
func (e *SproutError) Unwrap() error {
	return e.Cause
}

// NewError creates a new SproutError
func NewError(errorType ErrorType, message string) *SproutError {
	return &SproutError{
		Type:    errorType,
		Message: message,
		Details: make(map[string]interface{}),
	}
}

// NewErrorWithCause creates a new SproutError with an underlying cause
func NewErrorWithCause(errorType ErrorType, message string, cause error) *SproutError {
	return &SproutError{
		Type:    errorType,
		Message: message,
		Cause:   cause,
		Details: make(map[string]interface{}),
	}
}

// WithCode adds an error code
func (e *SproutError) WithCode(code string) *SproutError {
	e.Code = code
	return e
}

// WithCause adds a causing error
func (e *SproutError) WithCause(cause error) *SproutError {
	e.Cause = cause
	return e
}

// WithDetail adds a detail key-value pair
func (e *SproutError) WithDetail(key string, value interface{}) *SproutError {
	e.Details[key] = value
	return e
}

// WithRetryable marks the error as retryable
func (e *SproutError) WithRetryable(retryable bool) *SproutError {
	e.Retryable = retryable
	return e
}

// IsType checks if the error is of a specific type
func IsType(err error, errorType ErrorType) bool {
	if sproutErr, ok := err.(*SproutError); ok {
		return sproutErr.Type == errorType
	}
	return false
}

// Common error constructors
func ValidationError(message string) *SproutError {
	return NewError(ErrorTypeValidation, message)
}

func NotFoundError(message string) *SproutError {
	return NewError(ErrorTypeNotFound, message)
}

func ConflictError(message string) *SproutError {
	return NewError(ErrorTypeConflict, message)
}

func ExternalError(message string, cause error) *SproutError {
	return NewErrorWithCause(ErrorTypeExternal, message, cause).WithRetryable(true)
}

func InternalError(message string, cause error) *SproutError {
	return NewErrorWithCause(ErrorTypeInternal, message, cause)
}

func ConfigurationError(message string) *SproutError {
	return NewError(ErrorTypeConfiguration, message)
}

// Common error instances
var (
	NoProvider = NewError(ErrorTypeConfiguration, "no provider configured")
)