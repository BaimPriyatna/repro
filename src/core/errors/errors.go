// Package errors defines Repro's structured error type.
//
// Every error produced by the system carries:
//   - a stable machine-readable Code
//   - a human-readable Message (may evolve without changing Code)
//   - optional Details for structured context
//   - optional Cause wrapping a lower-level error
//   - optional Recovery hint
package errors

import "fmt"

// Code is a stable, machine-readable error identifier.
// Codes must not be renumbered once published.
type Code string

const (
	// CodeUnknown is the fallback when no specific code applies.
	CodeUnknown Code = "REPRO_UNKNOWN"

	// CodeInvalidInput is returned when caller-supplied input fails validation.
	CodeInvalidInput Code = "REPRO_INVALID_INPUT"

	// CodeNotFound is returned when a requested resource does not exist.
	CodeNotFound Code = "REPRO_NOT_FOUND"

	// CodeStorageFailure is returned on persistence-layer errors.
	CodeStorageFailure Code = "REPRO_STORAGE_FAILURE"

	// CodeInternal is returned for unexpected internal conditions.
	CodeInternal Code = "REPRO_INTERNAL"
)

// ReproError is the canonical error type for the entire Repro system.
// Use errors.New / errors.Wrap instead of constructing directly.
type ReproError struct {
	Code     Code
	Message  string
	Details  map[string]any
	Cause    error
	Recovery string
}

func (e *ReproError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("[%s] %s: %v", e.Code, e.Message, e.Cause)
	}
	return fmt.Sprintf("[%s] %s", e.Code, e.Message)
}

// Unwrap exposes the wrapped cause for use with errors.Is / errors.As.
func (e *ReproError) Unwrap() error { return e.Cause }

// New creates a ReproError with the given code and message.
func New(code Code, message string) *ReproError {
	return &ReproError{Code: code, Message: message}
}

// Wrap creates a ReproError that wraps an existing error.
func Wrap(code Code, message string, cause error) *ReproError {
	return &ReproError{Code: code, Message: message, Cause: cause}
}

// WithDetails attaches key-value context to a ReproError and returns it
// for chaining.
func (e *ReproError) WithDetails(details map[string]any) *ReproError {
	e.Details = details
	return e
}

// WithRecovery attaches a recovery hint and returns the error for chaining.
func (e *ReproError) WithRecovery(hint string) *ReproError {
	e.Recovery = hint
	return e
}

// Is reports whether any error in err's chain is a *ReproError with the specified Code.
func Is(err error, code Code) bool {
	for err != nil {
		if r, ok := err.(*ReproError); ok {
			if r.Code == code {
				return true
			}
		}
		u, ok := err.(interface{ Unwrap() error })
		if !ok {
			return false
		}
		err = u.Unwrap()
	}
	return false
}
