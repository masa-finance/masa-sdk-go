// Package masatwitter provides functionality for interacting with the Masa Protocol Twitter API
package x

import (
	"fmt"
	"time"
)

// Common error status codes
const (
	StatusNoWorkers      = 417 // Expectation Failed - No workers available
	StatusRateLimit      = 429 // Too Many Requests - Rate limit exceeded
	StatusWorkerLimit    = 500 // Internal Server Error - All workers rate limited
	StatusGatewayTimeout = 504 // Gateway Timeout - Request timed out
	StatusServiceDown    = 503 // Service Unavailable - Network/server issues
)

// Common retry delays and defaults
const (
	// TimeoutRetryDelay is used when a request times out (504 Gateway Timeout).
	// A moderate delay is used as timeouts may indicate temporary server load issues.
	TimeoutRetryDelay = 5 * time.Second

	// EmptyResponseDelay is used when the API returns an empty response (200 OK with empty body).
	// A short delay is used as this may be a temporary parsing/data issue.
	EmptyResponseDelay = 3 * time.Second

	// ConnectionRetryDelay is used when network connectivity issues occur (503 Service Unavailable).
	// A longer delay is used to allow network issues to resolve.
	ConnectionRetryDelay = 10 * time.Second

	// DefaultRetryDelay is the fallback delay for retryable errors that
	// don't have a specific retry strategy. Uses a short delay as the
	// issue might resolve quickly. Applied to unknown error types and
	// status codes not explicitly handled.
	DefaultRetryDelay = 1 * time.Second

	// DefaultRateLimitDelay is used when hitting API rate limits (429 Too Many Requests)
	// or worker limits (500 Worker Rate Limit). Uses a longer delay (60s) to allow
	// rate limit quotas to reset. This is typically used when no specific retry-after
	// time is provided in the response headers.
	DefaultRateLimitDelay = 60 * time.Second
)

// Base error types
// ----------------

// APIError represents a generic error returned by the Masa Twitter API
type APIError struct {
	StatusCode int
	Message    string
	Details    string
}

func (e *APIError) Error() string {
	if e.Details != "" {
		return fmt.Sprintf("masa twitter API error (status %d): %s - details: %s",
			e.StatusCode, e.Message, e.Details)
	}
	return fmt.Sprintf("masa twitter API error (status %d): %s",
		e.StatusCode, e.Message)
}

// Rate limiting errors
// ------------------

// RateLimitError represents a rate limit error (429) from the Masa Twitter API
type RateLimitError struct {
	RetryAfter time.Duration
	WorkerID   string
}

func (e *RateLimitError) Error() string {
	if e.WorkerID != "" {
		return fmt.Sprintf("rate limit exceeded for worker %s, retry after %v",
			e.WorkerID, e.RetryAfter)
	}
	return fmt.Sprintf("rate limit exceeded, retry after %v", e.RetryAfter)
}

// WorkerRateLimitError represents when all workers are rate-limited (500)
type WorkerRateLimitError struct {
	RetryAfter time.Duration
}

func (e *WorkerRateLimitError) Error() string {
	return fmt.Sprintf("all workers are rate-limited, retry after %v", e.RetryAfter)
}

// Worker availability errors
// ------------------------

// NoWorkersError represents when no workers are available (417)
type NoWorkersError struct {
	Message string
}

func (e *NoWorkersError) Error() string {
	return fmt.Sprintf("no workers available: %s", e.Message)
}

// Request errors
// -------------

// TimeoutError represents a request timeout
type TimeoutError struct {
	Operation string
	Duration  time.Duration
}

func (e *TimeoutError) Error() string {
	return fmt.Sprintf("operation %s timed out after %v", e.Operation, e.Duration)
}

// EmptyResponseError represents when the API returns an empty response
type EmptyResponseError struct {
	Query string
}

func (e *EmptyResponseError) Error() string {
	return fmt.Sprintf("received empty response for query: %s", e.Query)
}

// ConnectionError represents network connectivity issues
type ConnectionError struct {
	Err error
}

func (e *ConnectionError) Error() string {
	return fmt.Sprintf("connection error: %v", e.Err)
}

// Error handling utilities
// ----------------------

// IsRetryable returns true if the error is temporary and the operation can be retried
func IsRetryable(err error) bool {
	switch e := err.(type) {
	case *RateLimitError, *TimeoutError, *ConnectionError,
		*WorkerRateLimitError, *EmptyResponseError:
		return true
	case *APIError:
		return e.StatusCode == StatusGatewayTimeout ||
			e.StatusCode == StatusServiceDown
	default:
		return false
	}
}

// GetRetryDelay returns the recommended retry delay for the error
func GetRetryDelay(err error) time.Duration {
	switch e := err.(type) {
	case *RateLimitError:
		return e.RetryAfter
	case *WorkerRateLimitError:
		return e.RetryAfter
	case *TimeoutError:
		return TimeoutRetryDelay
	case *EmptyResponseError:
		return EmptyResponseDelay
	case *ConnectionError:
		return ConnectionRetryDelay
	default:
		return DefaultRetryDelay
	}
}

// NewRateLimitError creates a new RateLimitError with default retry delay if none specified
func NewRateLimitError(retryAfter time.Duration, workerID string) *RateLimitError {
	if retryAfter <= 0 {
		retryAfter = DefaultRateLimitDelay
	}
	return &RateLimitError{
		RetryAfter: retryAfter,
		WorkerID:   workerID,
	}
}
