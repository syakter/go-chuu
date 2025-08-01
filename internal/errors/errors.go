package errors

import (
	"fmt"
	"net/http"
)

// Error types for the application
type ErrorType string

const (
	ErrorTypeValidation   ErrorType = "validation"
	ErrorTypeNotFound     ErrorType = "not_found"
	ErrorTypeRateLimit    ErrorType = "rate_limit"
	ErrorTypeAPI          ErrorType = "api"
	ErrorTypeNetwork      ErrorType = "network"
	ErrorTypeInternal     ErrorType = "internal"
	ErrorTypeUnauthorized ErrorType = "unauthorized"
	ErrorTypeTimeout      ErrorType = "timeout"
)

// AppError represents application-specific errors
type AppError struct {
	Type       ErrorType `json:"type"`
	Message    string    `json:"message"`
	Code       string    `json:"code,omitempty"`
	StatusCode int       `json:"status_code,omitempty"`
	Cause      error     `json:"-"`
}

func (e *AppError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("%s: %s (caused by: %v)", e.Type, e.Message, e.Cause)
	}
	return fmt.Sprintf("%s: %s", e.Type, e.Message)
}

func (e *AppError) Unwrap() error {
	return e.Cause
}

// IsType checks if the error is of a specific type
func IsType(err error, errorType ErrorType) bool {
	if appErr, ok := err.(*AppError); ok {
		return appErr.Type == errorType
	}
	return false
}

// Error constructors
func NewValidationError(message string) *AppError {
	return &AppError{
		Type:       ErrorTypeValidation,
		Message:    message,
		StatusCode: http.StatusBadRequest,
	}
}

func NewNotFoundError(message string) *AppError {
	return &AppError{
		Type:       ErrorTypeNotFound,
		Message:    message,
		StatusCode: http.StatusNotFound,
	}
}

func NewRateLimitError(message string) *AppError {
	return &AppError{
		Type:       ErrorTypeRateLimit,
		Message:    message,
		StatusCode: http.StatusTooManyRequests,
	}
}

func NewAPIError(message string, cause error) *AppError {
	return &AppError{
		Type:       ErrorTypeAPI,
		Message:    message,
		StatusCode: http.StatusBadGateway,
		Cause:      cause,
	}
}

func NewNetworkError(message string, cause error) *AppError {
	return &AppError{
		Type:       ErrorTypeNetwork,
		Message:    message,
		StatusCode: http.StatusServiceUnavailable,
		Cause:      cause,
	}
}

func NewInternalError(message string, cause error) *AppError {
	return &AppError{
		Type:       ErrorTypeInternal,
		Message:    message,
		StatusCode: http.StatusInternalServerError,
		Cause:      cause,
	}
}

func NewUnauthorizedError(message string) *AppError {
	return &AppError{
		Type:       ErrorTypeUnauthorized,
		Message:    message,
		StatusCode: http.StatusUnauthorized,
	}
}

func NewTimeoutError(message string, cause error) *AppError {
	return &AppError{
		Type:       ErrorTypeTimeout,
		Message:    message,
		StatusCode: http.StatusRequestTimeout,
		Cause:      cause,
	}
}

// GetUserFriendlyMessage returns a user-friendly error message
func GetUserFriendlyMessage(err error) string {
	if appErr, ok := err.(*AppError); ok {
		switch appErr.Type {
		case ErrorTypeValidation:
			return fmt.Sprintf("Invalid input: %s", appErr.Message)
		case ErrorTypeNotFound:
			return "Sorry, I couldn't find that information. Please check your spelling and try again."
		case ErrorTypeRateLimit:
			return "I'm being rate limited. Please wait a moment and try again."
		case ErrorTypeAPI:
			return "There was an issue with the music service. Please try again later."
		case ErrorTypeNetwork:
			return "I'm having trouble connecting to the music service. Please try again."
		case ErrorTypeTimeout:
			return "The request took too long. Please try again."
		case ErrorTypeUnauthorized:
			return "I don't have permission to access that information."
		default:
			return "Something went wrong. Please try again later."
		}
	}
	return "An unexpected error occurred. Please try again later."
}
