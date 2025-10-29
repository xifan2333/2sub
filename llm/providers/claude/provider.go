// Package claude provides an LLM provider implementation for Anthropic's Claude models.
//
// Features:
//   - Supports all Claude models (Claude 3 Opus, Sonnet, Haiku, etc.)
//   - System prompts as a dedicated parameter
//   - Full control over temperature, max tokens, and other parameters
//
// Example usage:
//
//	import (
//	    "context"
//	    "github.com/xifan2333/2sub/llm"
//	    _ "github.com/xifan2333/2sub/llm/providers/claude"
//	)
//
//	opts := &llm.Options{
//	    APIKey: "sk-ant-...",
//	    Model: "claude-3-opus-20240229",
//	    SystemPrompt: "You are a helpful assistant.",
//	    Messages: []llm.Message{
//	        {Role: "user", Content: "Hello!"},
//	    },
//	}
//	result, err := llm.Chat(ctx, "claude", opts)
package claude

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/xifan2333/2sub/llm"
)

const (
	defaultBaseURL    = "https://api.anthropic.com"
	defaultAPIVersion = "2023-06-01"
)

// Provider implements the LLM provider interface for Claude.
type Provider struct{}

// Ensure Provider implements llm.Provider interface at compile time.
var _ llm.Provider = (*Provider)(nil)

func init() {
	// Register the provider on package initialization.
	llm.Register(&Provider{})
}

// Name returns the provider's unique identifier.
func (p *Provider) Name() string {
	return "claude"
}

// Chat performs LLM chat completion using Claude API.
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

	req, err := http.NewRequestWithContext(ctx, "POST", baseURL+"/v1/messages", bytes.NewReader(reqData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", opts.APIKey)
	req.Header.Set("anthropic-version", defaultAPIVersion)

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

// buildRequest builds the Claude API request body.
func (p *Provider) buildRequest(opts *llm.Options) map[string]interface{} {
	req := map[string]interface{}{
		"model":    opts.Model,
		"messages": p.convertMessages(opts.Messages),
	}

	// Claude has a dedicated system parameter
	if opts.SystemPrompt != "" {
		req["system"] = opts.SystemPrompt
	}

	if opts.Temperature > 0 {
		req["temperature"] = opts.Temperature
	}

	if opts.MaxTokens > 0 {
		req["max_tokens"] = opts.MaxTokens
	} else {
		// Claude requires max_tokens, set a reasonable default
		req["max_tokens"] = 4096
	}

	if opts.TopP > 0 {
		req["top_p"] = opts.TopP
	}

	if len(opts.Stop) > 0 {
		req["stop_sequences"] = opts.Stop
	}

	// Merge extra options
	for k, v := range opts.Extra {
		req[k] = v
	}

	return req
}

// convertMessages converts unified messages to Claude format.
func (p *Provider) convertMessages(messages []llm.Message) []map[string]interface{} {
	result := make([]map[string]interface{}, 0, len(messages))

	for _, msg := range messages {
		// Skip system messages as they're handled separately in Claude
		if msg.Role == "system" {
			continue
		}

		m := map[string]interface{}{
			"role":    msg.Role,
			"content": msg.Content,
		}
		result = append(result, m)
	}

	return result
}

// handleNonStream handles non-streaming response.
func (p *Provider) handleNonStream(resp *http.Response) (*llm.StandardResult, error) {
	defer resp.Body.Close()

	var apiResp claudeResponse
	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	if len(apiResp.Content) == 0 {
		return nil, fmt.Errorf("no content in response")
	}

	// Concatenate all text content
	var content strings.Builder
	for _, c := range apiResp.Content {
		if c.Type == "text" {
			content.WriteString(c.Text)
		}
	}

	return &llm.StandardResult{
		Content:      content.String(),
		FinishReason: apiResp.StopReason,
		Model:        apiResp.Model,
		Usage: llm.Usage{
			PromptTokens:     apiResp.Usage.InputTokens,
			CompletionTokens: apiResp.Usage.OutputTokens,
			TotalTokens:      apiResp.Usage.InputTokens + apiResp.Usage.OutputTokens,
		},
		Raw: apiResp,
	}, nil
}

// Claude API response structures
type claudeResponse struct {
	ID         string `json:"id"`
	Type       string `json:"type"`
	Role       string `json:"role"`
	Model      string `json:"model"`
	StopReason string `json:"stop_reason"`
	Content    []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	} `json:"content"`
	Usage struct {
		InputTokens  int `json:"input_tokens"`
		OutputTokens int `json:"output_tokens"`
	} `json:"usage"`
}
