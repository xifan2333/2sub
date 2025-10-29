package bijian

import "fmt"

// ValidationError represents a validation error
type ValidationError struct {
	Field   string
	Message string
}

func (e *ValidationError) Error() string {
	return fmt.Sprintf("validation error on field '%s': %s", e.Field, e.Message)
}

// FetchError represents an error during fetch operation
type FetchError struct {
	Step    string
	Message string
	Err     error
}

func (e *FetchError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("fetch error at step '%s': %s: %v", e.Step, e.Message, e.Err)
	}
	return fmt.Sprintf("fetch error at step '%s': %s", e.Step, e.Message)
}

func (e *FetchError) Unwrap() error {
	return e.Err
}

// ParseError represents an error during parse operation
type ParseError struct {
	Message string
	Err     error
}

func (e *ParseError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("parse error: %s: %v", e.Message, e.Err)
	}
	return fmt.Sprintf("parse error: %s", e.Message)
}

func (e *ParseError) Unwrap() error {
	return e.Err
}

// APIError represents an API response error
type APIError struct {
	StatusCode int
	Response   string
}

func (e *APIError) Error() string {
	return fmt.Sprintf("API error (status %d): %s", e.StatusCode, e.Response)
}
