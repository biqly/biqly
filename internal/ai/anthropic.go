package ai

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/biqly/biqly/internal/config"
)

const (
	anthropicDefaultBaseURL = "https://api.anthropic.com/v1"
	anthropicAPIVersion     = "2023-06-01"
)

// AnthropicProvider speaks Anthropic's Messages API. It mirrors the OpenAI
// adapter's contract so callers can swap providers via config alone.
type AnthropicProvider struct {
	base baseProvider
}

// NewAnthropicProvider configures the Anthropic adapter from AIConfig.
func NewAnthropicProvider(cfg config.AIConfig) *AnthropicProvider {
	baseURL := cfg.BaseURL
	if baseURL == "" {
		baseURL = anthropicDefaultBaseURL
	}
	http := newHTTPProvider(cfg.AIHTTPTimeout(), baseURL, cfg.APIKey)
	return &AnthropicProvider{
		base: baseProvider{
			http:        http,
			model:       cfg.Model,
			maxTokens:   cfg.MaxTokens,
			temperature: cfg.Temperature,
			logName:     "anthropic",
			hooks: providerHooks{
				path: "/messages",
				headers: func(p httpProvider) map[string]string {
					return map[string]string{
						"x-api-key":         p.apiKey,
						"anthropic-version": anthropicAPIVersion,
					}
				},
				marshal: marshalAnthropicRequest(cfg.Model, cfg.MaxTokens),
				parse:   parseAnthropicResponse,
			},
		},
	}
}

type anthropicMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type anthropicRequest struct {
	Model       string             `json:"model"`
	System      string             `json:"system,omitempty"`
	Messages    []anthropicMessage `json:"messages"`
	MaxTokens   int                `json:"max_tokens"`
	Temperature float64            `json:"temperature"`
}

type anthropicResponse struct {
	Content []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	} `json:"content"`
	Usage *struct {
		InputTokens  int `json:"input_tokens"`
		OutputTokens int `json:"output_tokens"`
	} `json:"usage,omitempty"`
	Error *struct {
		Type    string `json:"type"`
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

// Generate sends a prompt at the configured temperature.
func (p *AnthropicProvider) Generate(ctx context.Context, prompt string) (GenerationResult, error) {
	return p.base.generate(ctx, prompt)
}

// GenerateAt sends a prompt with an explicit temperature override.
func (p *AnthropicProvider) GenerateAt(ctx context.Context, prompt string, temperature float64) (GenerationResult, error) {
	return p.base.generateAt(ctx, prompt, temperature)
}

func marshalAnthropicRequest(model string, maxTokens int) func(string, float64) ([]byte, error) {
	return func(prompt string, temperature float64) ([]byte, error) {
		reqBody := anthropicRequest{
			Model:       model,
			System:      SystemDirective,
			Messages:    []anthropicMessage{{Role: "user", Content: prompt}},
			MaxTokens:   maxTokens,
			Temperature: temperature,
		}
		return json.Marshal(reqBody)
	}
}

func parseAnthropicResponse(respBody []byte) (GenerationResult, error) {
	var ar anthropicResponse
	if err := json.Unmarshal(respBody, &ar); err != nil {
		return GenerationResult{}, fmt.Errorf("unmarshal response: %w", err)
	}
	if ar.Error != nil {
		return GenerationResult{}, fmt.Errorf("Anthropic API error: %s", ar.Error.Message)
	}
	var text string
	for _, block := range ar.Content {
		if block.Type == "text" && block.Text != "" {
			text = block.Text
			break
		}
	}
	if text == "" {
		return GenerationResult{}, fmt.Errorf("no text content in Anthropic response")
	}
	gen := GenerationResult{Content: text}
	if ar.Usage != nil {
		gen.Usage = newTokenUsage(ar.Usage.InputTokens, ar.Usage.OutputTokens, 0)
	}
	return gen, nil
}
