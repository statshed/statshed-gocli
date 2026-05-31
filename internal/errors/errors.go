// Package errors defines the StatShed CLI error types and exit-code scheme.
//
// AIDEV-NOTE: Exit codes follow a specific scheme (ported from the Python CLI):
//   - 0: Success
//   - 1: Unhealthy status (for health command)
//   - 2-4: API/network errors
//   - 5: Configuration errors
//   - 10-11: Invalid arguments or not found
package errors

import "fmt"

// ExitCode is the process exit status for a command outcome.
type ExitCode int

const (
	ExitSuccess     ExitCode = 0  // Success
	ExitUnhealthy   ExitCode = 1  // Health check returned unhealthy status
	ExitAPI         ExitCode = 2  // API returned an error response
	ExitConnection  ExitCode = 3  // Could not connect to the server
	ExitTimeout     ExitCode = 4  // Request timed out
	ExitConfig      ExitCode = 5  // Configuration file error
	ExitInvalidArgs ExitCode = 10 // Invalid command arguments
	ExitNotFound    ExitCode = 11 // Resource not found (group, job)
)

// StatShedError is the base error type carrying an exit code.
//
// AIDEV-NOTE: Mirrors the Python StatShedError hierarchy. Construct via the
// helper functions below so each kind gets its canonical exit code; use the
// Kind field with errors.As to branch on category.
type StatShedError struct {
	Message string
	Code    ExitCode
	Kind    Kind
	// StatusCode is the HTTP status for API errors (0 when not applicable).
	StatusCode int
}

// Kind categorizes an error independently of its message, so callers can
// branch (e.g. on NotFound) without string matching.
type Kind int

const (
	KindAPI Kind = iota
	KindConfig
	KindConnection
	KindTimeout
	KindNotFound
	KindInvalidArgs
)

func (e *StatShedError) Error() string { return e.Message }

// ExitCodeOf returns the exit code for any error, defaulting to ExitAPI.
func ExitCodeOf(err error) ExitCode {
	var se *StatShedError
	if asStatShed(err, &se) {
		return se.Code
	}
	return ExitAPI
}

// IsNotFound reports whether err is a not-found error.
func IsNotFound(err error) bool {
	var se *StatShedError
	return asStatShed(err, &se) && se.Kind == KindNotFound
}

// Config returns a configuration error (exit 5).
func Config(format string, a ...any) *StatShedError {
	return &StatShedError{Message: fmt.Sprintf(format, a...), Code: ExitConfig, Kind: KindConfig}
}

// Connection returns a connection error (exit 3).
func Connection(format string, a ...any) *StatShedError {
	return &StatShedError{Message: fmt.Sprintf(format, a...), Code: ExitConnection, Kind: KindConnection}
}

// Timeout returns a timeout error (exit 4).
func Timeout(format string, a ...any) *StatShedError {
	return &StatShedError{Message: fmt.Sprintf(format, a...), Code: ExitTimeout, Kind: KindTimeout}
}

// API returns a generic API error (exit 2).
func API(format string, a ...any) *StatShedError {
	return &StatShedError{Message: fmt.Sprintf(format, a...), Code: ExitAPI, Kind: KindAPI}
}

// APIStatus returns an API error annotated with the HTTP status code.
func APIStatus(statusCode int, format string, a ...any) *StatShedError {
	return &StatShedError{
		Message:    fmt.Sprintf(format, a...),
		Code:       ExitAPI,
		Kind:       KindAPI,
		StatusCode: statusCode,
	}
}

// NotFound returns a not-found error (exit 11).
func NotFound(format string, a ...any) *StatShedError {
	return &StatShedError{Message: fmt.Sprintf(format, a...), Code: ExitNotFound, Kind: KindNotFound}
}

// InvalidArgs returns an invalid-arguments error (exit 10).
func InvalidArgs(format string, a ...any) *StatShedError {
	return &StatShedError{Message: fmt.Sprintf(format, a...), Code: ExitInvalidArgs, Kind: KindInvalidArgs}
}

// asStatShed unwraps err looking for a *StatShedError. Kept tiny to avoid
// importing the stdlib errors package everywhere.
func asStatShed(err error, target **StatShedError) bool {
	for err != nil {
		if se, ok := err.(*StatShedError); ok {
			*target = se
			return true
		}
		u, ok := err.(interface{ Unwrap() error })
		if !ok {
			return false
		}
		err = u.Unwrap()
	}
	return false
}
