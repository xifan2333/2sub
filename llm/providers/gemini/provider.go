// Package gemini provides an LLM provider implementation for Google's Gemini models.
//
// Features:
//   - Supports Gemini Pro and other Gemini models
//   - Full control over temperature, max tokens, and other parameters
//
// Example usage:
//
//	import (
//	    "context"
//	    "github.com/xifan2333/2sub/llm"
//	    _ "github.com/xifan2333/2sub/llm/providers/gemini"
//	)
//
//	opts := &llm.Options{
//	    APIKey: "your-api-key",
//	    Model: "gemini-pro",
//	    Messages: []llm.Message{
//	        {Role: "user", Content: "Hello!"},
//	    },
//	}
//	result, err := llm.Chat(ctx, "gemini", opts)
package gemini

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

const defaultBaseURL = "https://generativelanguage.googleapis.com"

// Provider implements the LLM provider interface for Gemini.
type Provider struct{}

// Ensure Provider implements llm.Provider interface at compile time.
var _ llm.Provider = (*Provider)(nil)

func init() {
	// Register the provider on package initialization.
	llm.Register(&Provider{})
}

// Name returns the provider's unique identifier.
func (p *Provider) Name() string {
	return "gemini"
}

// Chat performs LLM chat completion using Gemini API.
func (p *Provider) Chat(ctx context.Context, opts *llm.Options) (*llm.StandardResult, error) {
	baseURL := opts.BaseURL
	if baseURL == "" {
		baseURL = defaultBaseURL
	}

	// Build endpoint
	endpoint := fmt.Sprintf("%s/v1beta/models/%s:generateContent?key=%s", baseURL, opts.Model, opts.APIKey)

	// Build request
	reqBody := p.buildRequest(opts)
	reqData, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", endpoint, bytes.NewReader(reqData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

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

// buildRequest builds the Gemini API request body.
func (p *Provider) buildRequest(opts *llm.Options) map[string]interface{} {
	req := map[string]interface{}{
		"contents": p.convertMessages(opts),
	}

	// Build generation config
	genConfig := make(map[string]interface{})

	if opts.Temperature > 0 {
		genConfig["temperature"] = opts.Temperature
	}

	if opts.MaxTokens > 0 {
		genConfig["maxOutputTokens"] = opts.MaxTokens
	}

	if opts.TopP > 0 {
		genConfig["topP"] = opts.TopP
	}

	if len(opts.Stop) > 0 {
		genConfig["stopSequences"] = opts.Stop
	}

	if len(genConfig) > 0 {
		req["generationConfig"] = genConfig
	}

	// System instruction (if supported by the model)
	if opts.SystemPrompt != "" {
		req["systemInstruction"] = map[string]interface{}{
			"parts": []map[string]interface{}{
				{"text": opts.SystemPrompt},
			},
		}
	}

	// Merge extra options
	for k, v := range opts.Extra {
		req[k] = v
	}

	return req
}

// convertMessages converts unified messages to Gemini format.
func (p *Provider) convertMessages(opts *llm.Options) []map[string]interface{} {
	contents := make([]map[string]interface{}, 0, len(opts.Messages))

	for _, msg := range opts.Messages {
		// Skip system messages as they're handled separately
		if msg.Role == "system" {
			continue
		}

		// Gemini uses "user" and "model" roles
		role := msg.Role
		if role == "assistant" {
			role = "model"
		}

		content := map[string]interface{}{
			"role": role,
			"parts": []map[string]interface{}{
				{"text": msg.Content},
			},
		}
		contents = append(contents, content)
	}

	return contents
}

// handleNonStream handles non-streaming response.
func (p *Provider) handleNonStream(resp *http.Response) (*llm.StandardResult, error) {
	defer resp.Body.Close()

	var apiResp geminiResponse
	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	if len(apiResp.Candidates) == 0 {
		return nil, fmt.Errorf("no candidates in response")
	}

	candidate := apiResp.Candidates[0]
	if len(candidate.Content.Parts) == 0 {
		return nil, fmt.Errorf("no parts in candidate content")
	}

	// Concatenate all text parts
	var content strings.Builder
	for _, part := range candidate.Content.Parts {
		content.WriteString(part.Text)
	}

	result := &llm.StandardResult{
		Content:      content.String(),
		FinishReason: candidate.FinishReason,
		Model:        apiResp.ModelVersion,
		Raw:          apiResp,
	}

	// Add usage information if available
	if apiResp.UsageMetadata.PromptTokenCount > 0 {
		result.Usage = llm.Usage{
			PromptTokens:     apiResp.UsageMetadata.PromptTokenCount,
			CompletionTokens: apiResp.UsageMetadata.CandidatesTokenCount,
			TotalTokens:      apiResp.UsageMetadata.TotalTokenCount,
		}
	}

	return result, nil
}

// Gemini API response structures
type geminiResponse struct {
	Candidates []struct {
		Content struct {
			Parts []struct {
				Text string `json:"text"`
			} `json:"parts"`
			Role string `json:"role"`
		} `json:"content"`
		FinishReason string `json:"finishReason"`
		Index        int    `json:"index"`
	} `json:"candidates"`
	UsageMetadata struct {
		PromptTokenCount     int `json:"promptTokenCount"`
		CandidatesTokenCount int `json:"candidatesTokenCount"`
		TotalTokenCount      int `json:"totalTokenCount"`
	} `json:"usageMetadata"`
	ModelVersion string `json:"modelVersion"`
}
