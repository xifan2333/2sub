// Package llm provides a unified interface for multiple LLM (Large Language Model) providers.
//
// This package allows you to use different LLM services through a single, consistent API.
// All providers return results in a standardized format, making it easy to switch between
// providers or use multiple providers in the same application.
//
// Example usage:
//
//	import (
//	    "context"
//	    "github.com/xifan2333/2sub/llm"
//	    _ "github.com/xifan2333/2sub/llm/providers/openai"
//	)
//
//	func main() {
//	    ctx := context.Background()
//	    opts := &llm.Options{
//	        APIKey: "your-api-key",
//	        Model: "gpt-4",
//	        Messages: []llm.Message{
//	            {Role: "user", Content: "Hello!"},
//	        },
//	    }
//	    result, err := llm.Chat(ctx, "openai", opts)
//	    if err != nil {
//	        panic(err)
//	    }
//	    fmt.Println(result.Content)
//	}
package llm

import "context"

// Provider defines the interface that all LLM providers must implement.
//
// A provider is responsible for:
//   - Sending requests to an LLM service
//   - Parsing responses into the standardized format
//
// Providers must be registered using the Register function, typically in their init() function.
type Provider interface {
	// Name returns the provider's unique identifier.
	// This name is used when calling Get() or Chat().
	//
	// Examples: "claude", "openai", "gemini"
	Name() string

	// Chat performs LLM chat completion and returns the standardized response.
	//
	// Parameters:
	//   - ctx: Context for cancellation and timeout
	//   - opts: Provider options including API key, model, messages, etc.
	//
	// Returns the standardized result or an error.
	Chat(ctx context.Context, opts *Options) (*StandardResult, error)
}

// Options contains unified options for LLM requests.
//
// These options work across all providers, though some providers may ignore
// certain fields (e.g., Gemini may not support all OpenAI-specific parameters).
type Options struct {
	// BaseURL is the API endpoint base URL.
	// If not specified, the provider's default URL will be used.
	// This is useful for third-party compatible APIs.
	//
	// Examples:
	//   - OpenAI: "https://api.openai.com/v1"
	//   - Third-party: "https://api.example.com/v1"
	BaseURL string

	// APIKey is the authentication API key.
	// Required by most providers.
	APIKey string

	// Model is the model identifier to use.
	// Required. The format depends on the provider.
	//
	// Examples:
	//   - OpenAI: "gpt-4", "gpt-3.5-turbo"
	//   - Claude: "claude-3-opus-20240229", "claude-3-sonnet-20240229"
	//   - Gemini: "gemini-pro", "gemini-pro-vision"
	Model string

	// Messages is the conversation history.
	// Required. Must contain at least one message.
	Messages []Message

	// Temperature controls randomness (0.0 to 2.0).
	// Higher values make output more random, lower values more deterministic.
	// Default: 1.0
	Temperature float64

	// MaxTokens is the maximum number of tokens to generate.
	// If not specified, the provider's default will be used.
	MaxTokens int

	// TopP controls nucleus sampling (0.0 to 1.0).
	// Alternative to temperature. Not supported by all providers.
	TopP float64

	// Stop is a list of sequences where the API will stop generating.
	// Not supported by all providers.
	Stop []string

	// SystemPrompt is a system-level instruction.
	// Some providers (like Claude) have a dedicated system parameter.
	// For others, this will be prepended as a system message.
	SystemPrompt string

	// Extra contains provider-specific options.
	// Use this for parameters that are not part of the standard interface.
	Extra map[string]interface{}
}

// Message represents a single message in the conversation.
type Message struct {
	// Role is the message sender role.
	// Common values: "system", "user", "assistant"
	Role string `json:"role"`

	// Content is the message content.
	Content string `json:"content"`

	// Name is an optional name for the message sender.
	// Not supported by all providers.
	Name string `json:"name,omitempty"`
}

// StandardResult represents the unified LLM completion result.
//
// All providers must convert their responses to this standardized format.
type StandardResult struct {
	// Content is the generated text.
	Content string `json:"content,omitempty"`

	// FinishReason indicates why the generation stopped.
	// Common values: "stop", "length", "content_filter"
	FinishReason string `json:"finish_reason,omitempty"`

	// Usage contains token usage information.
	Usage Usage `json:"usage,omitempty"`

	// Model is the actual model used (may differ from requested).
	Model string `json:"model,omitempty"`

	// Raw contains the original provider response for debugging.
	// The type depends on the provider.
	Raw interface{} `json:"raw,omitempty"`
}

// Usage contains token usage information.
type Usage struct {
	// PromptTokens is the number of tokens in the prompt.
	PromptTokens int `json:"prompt_tokens"`

	// CompletionTokens is the number of tokens in the completion.
	CompletionTokens int `json:"completion_tokens"`

	// TotalTokens is the total number of tokens used.
	TotalTokens int `json:"total_tokens"`
}

// Validate validates the options and sets default values.
//
// Returns an error if required fields are missing or invalid.
func (o *Options) Validate() error {
	if o.APIKey == "" {
		return &ValidationError{Field: "APIKey", Message: "API key is required"}
	}

	if o.Model == "" {
		return &ValidationError{Field: "Model", Message: "model is required"}
	}

	if len(o.Messages) == 0 {
		return &ValidationError{Field: "Messages", Message: "at least one message is required"}
	}

	// Set defaults
	if o.Temperature == 0 {
		o.Temperature = 1.0
	}

	if o.Temperature < 0 || o.Temperature > 2 {
		return &ValidationError{Field: "Temperature", Message: "must be between 0 and 2"}
	}

	if o.TopP != 0 && (o.TopP < 0 || o.TopP > 1) {
		return &ValidationError{Field: "TopP", Message: "must be between 0 and 1"}
	}

	if o.MaxTokens < 0 {
		return &ValidationError{Field: "MaxTokens", Message: "must be non-negative"}
	}

	return nil
}

// ValidationError represents a validation error.
type ValidationError struct {
	Field   string
	Message string
}

func (e *ValidationError) Error() string {
	return "validation error: " + e.Field + ": " + e.Message
}
