// Package openai provides an LLM provider implementation for OpenAI's GPT models.
//
// Features:
//   - Supports all OpenAI chat models (GPT-3.5, GPT-4, etc.)
//   - Compatible with OpenAI-compatible APIs (via BaseURL)
//   - Full control over temperature, max tokens, and other parameters
//
// Example usage:
//
//	import (
//	    "context"
//	    "github.com/xifan2333/2sub/llm"
//	    _ "github.com/xifan2333/2sub/llm/providers/openai"
//	)
//
//	opts := &llm.Options{
//	    APIKey: "sk-...",
//	    Model: "gpt-4",
//	    Messages: []llm.Message{
//	        {Role: "user", Content: "Hello!"},
//	    },
//	}
//	result, err := llm.Chat(ctx, "openai", opts)
package openai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/xifan2333/2sub/pkgs/llm"
)

const defaultBaseURL = "https://api.openai.com/v1"

// Provider implements the LLM provider interface for OpenAI.
type Provider struct{}

// Ensure Provider implements llm.Provider interface at compile time.
var _ llm.Provider = (*Provider)(nil)

func init() {
	// Register the provider on package initialization.
	llm.Register(&Provider{})
}

// Name returns the provider's unique identifier.
func (p *Provider) Name() string {
	return "openai"
}

// Chat performs LLM chat completion using OpenAI API.
func (p *Provider) Chat(ctx context.Context, opts *llm.Options) (*llm.StandardResult, error) {
	baseURL := opts.BaseURL
	if baseURL == "" {
		baseURL = defaultBaseURL
	}

	// Build request
	reqBody := p.buildRequest(opts)
	reqData, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", baseURL+"/chat/completions", bytes.NewReader(reqData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+opts.APIKey)

	// Send request
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		return nil, fmt.Errorf("API error (status %d): %s", resp.StatusCode, string(body))
	}

	return p.handleNonStream(resp)
}

// buildRequest builds the OpenAI API request body.
func (p *Provider) buildRequest(opts *llm.Options) map[string]interface{} {
	req := map[string]interface{}{
		"model":    opts.Model,
		"messages": p.convertMessages(opts),
	}

	if opts.Temperature > 0 {
		req["temperature"] = opts.Temperature
	}

	if opts.MaxTokens > 0 {
		req["max_tokens"] = opts.MaxTokens
	}

	if opts.TopP > 0 {
		req["top_p"] = opts.TopP
	}

	if len(opts.Stop) > 0 {
		req["stop"] = opts.Stop
	}

	// Merge extra options
	for k, v := range opts.Extra {
		req[k] = v
	}

	return req
}

// convertMessages converts unified messages to OpenAI format.
func (p *Provider) convertMessages(opts *llm.Options) []map[string]interface{} {
	messages := make([]map[string]interface{}, 0, len(opts.Messages)+1)

	// Add system prompt if present
	if opts.SystemPrompt != "" {
		messages = append(messages, map[string]interface{}{
			"role":    "system",
			"content": opts.SystemPrompt,
		})
	}

	// Add conversation messages
	for _, msg := range opts.Messages {
		m := map[string]interface{}{
			"role":    msg.Role,
			"content": msg.Content,
		}
		if msg.Name != "" {
			m["name"] = msg.Name
		}
		messages = append(messages, m)
	}

	return messages
}

// handleNonStream handles non-streaming response.
func (p *Provider) handleNonStream(resp *http.Response) (*llm.StandardResult, error) {
	defer resp.Body.Close()

	var apiResp openAIResponse
	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	if len(apiResp.Choices) == 0 {
		return nil, fmt.Errorf("no choices in response")
	}

	choice := apiResp.Choices[0]
	return &llm.StandardResult{
		Content:      choice.Message.Content,
		FinishReason: choice.FinishReason,
		Model:        apiResp.Model,
		Usage: llm.Usage{
			PromptTokens:     apiResp.Usage.PromptTokens,
			CompletionTokens: apiResp.Usage.CompletionTokens,
			TotalTokens:      apiResp.Usage.TotalTokens,
		},
		Raw: apiResp,
	}, nil
}

// OpenAI API response structures
type openAIResponse struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Created int64  `json:"created"`
	Model   string `json:"model"`
	Choices []struct {
		Index        int    `json:"index"`
		FinishReason string `json:"finish_reason"`
		Message      struct {
			Role    string `json:"role"`
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
	Usage struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
		TotalTokens      int `json:"total_tokens"`
	} `json:"usage"`
}
