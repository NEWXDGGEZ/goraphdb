package goraphdb

import (
	"errors"
	"fmt"
)

// ---------------------------------------------------------------------------
// Sentinel errors
// ---------------------------------------------------------------------------

var (
	// ErrNotFound is returned when a requested resource does not exist (HTTP 404).
	ErrNotFound = errors.New("goraphdb: not found")

	// ErrBadRequest is returned when the server rejects the request (HTTP 400).
	ErrBadRequest = errors.New("goraphdb: bad request")

	// ErrReadOnlyReplica is returned when a write is attempted on a read-only
	// follower that cannot forward to a leader (HTTP 503).
	ErrReadOnlyReplica = errors.New("goraphdb: read-only replica")

	// ErrServerError is returned for unexpected server-side failures (HTTP 5xx).
	ErrServerError = errors.New("goraphdb: server error")

	// ErrConflict is returned when an operation conflicts with existing state
	// (e.g. duplicate constraint, HTTP 409).
	ErrConflict = errors.New("goraphdb: conflict")

	// ErrIteratorClosed is returned when operations are attempted on a closed iterator.
	ErrIteratorClosed = errors.New("goraphdb: iterator closed")
)

// ---------------------------------------------------------------------------
// APIError
// ---------------------------------------------------------------------------

// APIError represents a structured error response from the GoraphDB server.
type APIError struct {
	// StatusCode is the HTTP status code returned by the server.
	StatusCode int
	// Message is the error message from the server's JSON response.
	Message string
}

// Error implements the error interface.
func (e *APIError) Error() string {
	return fmt.Sprintf("goraphdb: HTTP %d: %s", e.StatusCode, e.Message)
}

// Unwrap returns the appropriate sentinel error based on the status code,
// enabling errors.Is() checks against sentinel errors.
func (e *APIError) Unwrap() error {
	switch {
	case e.StatusCode == 400:
		return ErrBadRequest
	case e.StatusCode == 404:
		return ErrNotFound
	case e.StatusCode == 409:
		return ErrConflict
	case e.StatusCode == 503:
		return ErrReadOnlyReplica
	case e.StatusCode >= 500:
		return ErrServerError
	default:
		return nil
	}
}

// ---------------------------------------------------------------------------
// Error classification helpers
// ---------------------------------------------------------------------------

// IsNotFound reports whether the error indicates a not-found condition.
func IsNotFound(err error) bool {
	return errors.Is(err, ErrNotFound)
}

// IsBadRequest reports whether the error indicates a bad request.
func IsBadRequest(err error) bool {
	return errors.Is(err, ErrBadRequest)
}

// IsReadOnly reports whether the error indicates a read-only replica rejection.
func IsReadOnly(err error) bool {
	return errors.Is(err, ErrReadOnlyReplica)
}

// IsServerError reports whether the error indicates a server-side failure.
func IsServerError(err error) bool {
	return errors.Is(err, ErrServerError)
}

// IsConflict reports whether the error indicates a conflict.
func IsConflict(err error) bool {
	return errors.Is(err, ErrConflict)
}
