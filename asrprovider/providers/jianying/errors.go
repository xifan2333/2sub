package jianying

import "fmt"

// ValidationError represents a validation error for JianYing options.
type ValidationError struct {
	// Field is the name of the field that failed validation.
	Field string

	// Message describes what validation failed.
	Message string
}

func (e *ValidationError) Error() string {
	return fmt.Sprintf("validation error on field '%s': %s", e.Field, e.Message)
}

// FetchError represents an error that occurred during the fetch operation.
//
// The error includes the step where the error occurred and a descriptive message.
type FetchError struct {
	// Step identifies which step of the fetch process failed
	// (e.g., "upload_sign", "upload_file", "submit_task", "query_result").
	Step string

	// Message provides a human-readable description of the error.
	Message string

	// Err is the underlying error, if any.
	Err error
}

func (e *FetchError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("fetch error at step '%s': %s: %v", e.Step, e.Message, e.Err)
	}
	return fmt.Sprintf("fetch error at step '%s': %s", e.Step, e.Message)
}

// Unwrap returns the underlying error for error chain inspection.
func (e *FetchError) Unwrap() error {
	return e.Err
}

// ParseError represents an error that occurred during response parsing.
type ParseError struct {
	// Message provides a human-readable description of the parsing error.
	Message string

	// Err is the underlying error, if any.
	Err error
}

func (e *ParseError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("parse error: %s: %v", e.Message, e.Err)
	}
	return fmt.Sprintf("parse error: %s", e.Message)
}

// Unwrap returns the underlying error for error chain inspection.
func (e *ParseError) Unwrap() error {
	return e.Err
}

// APIError represents an HTTP API error response.
//
// This error is returned when the JianYing API returns a non-200 status code.
type APIError struct {
	// StatusCode is the HTTP status code returned by the API.
	StatusCode int

	// Response is the raw response body from the API.
	Response string
}

func (e *APIError) Error() string {
	return fmt.Sprintf("API error (status %d): %s", e.StatusCode, e.Response)
}
