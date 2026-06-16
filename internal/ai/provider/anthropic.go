package provider

import (
	"context"
	"errors"
	"fmt"
	"github.com/bytedance/sonic"

	promptpkg "github.com/biqly/biqly/internal/ai/prompt"
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
	conn := cfg.Connection
	gen := cfg.Generation
	baseURL := conn.BaseURL
	if baseURL == "" {
		baseURL = anthropicDefaultBaseURL
	}
	http := newHTTPProvider(cfg.HTTPTimeout(), baseURL, conn.APIKey)
	return &AnthropicProvider{
		base: baseProvider{
			http:        http,
			model:       conn.Model,
			maxTokens:   gen.MaxTokens,
			temperature: gen.Temperature,
			logName:     "anthropic",
			hooks: providerHooks{
				path: "/messages",
				headers: func(p httpProvider) map[string]string {
					return map[string]string{
						"x-api-key":         p.apiKey,
						"anthropic-version": anthropicAPIVersion,
					}
				},
				marshal: marshalAnthropicRequest(conn.Model, gen.MaxTokens),
				parse:   parseAnthropicResponse,
			},
		},
	}
}

// Close closes idle HTTP keepalive connections held by the provider.
func (a *AnthropicProvider) Close() error {
	return a.base.http.Close()
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
	StopReason string `json:"stop_reason,omitempty"`
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
func (a *AnthropicProvider) Generate(ctx context.Context, prompt string) (GenerationResult, error) {
	return a.base.generate(ctx, prompt)
}

// GenerateAt sends a prompt with an explicit temperature override.
func (a *AnthropicProvider) GenerateAt(ctx context.Context, prompt string, temperature float64) (GenerationResult, error) {
	return a.base.generateAt(ctx, prompt, temperature)
}

func marshalAnthropicRequest(model string, maxTokens int) func(string, float64) ([]byte, error) {
	return func(prompt string, temperature float64) ([]byte, error) {
		reqBody := anthropicRequest{
			Model:       model,
			System:      promptpkg.SystemDirective,
			Messages:    []anthropicMessage{{Role: "user", Content: prompt}},
			MaxTokens:   maxTokens,
			Temperature: temperature,
		}
		return sonic.ConfigStd.Marshal(reqBody)
	}
}

func parseAnthropicResponse(respBody []byte) (GenerationResult, error) {
	var ar anthropicResponse
	if err := sonic.ConfigStd.Unmarshal(respBody, &ar); err != nil {
		return GenerationResult{}, fmt.Errorf("unmarshal response: %w", err)
	}
	if ar.Error != nil {
		return GenerationResult{}, fmt.Errorf("anthropic API error: %s", ar.Error.Message)
	}
	var text string
	for _, block := range ar.Content {
		if block.Type == "text" && block.Text != "" {
			text = block.Text
			break
		}
	}
	if text == "" {
		return GenerationResult{}, errors.New("no text content in Anthropic response")
	}
	gen := GenerationResult{Content: text}
	if ar.StopReason != "" {
		if ar.StopReason == "max_tokens" {
			gen.FinishReason = "length"
		} else {
			gen.FinishReason = "stop"
		}
	}
	if ar.Usage != nil {
		gen.Usage = newTokenUsage(ar.Usage.InputTokens, ar.Usage.OutputTokens, 0)
	}
	return gen, nil
}
