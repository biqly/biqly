// Package ai provides AI-powered schema description and prompt building.
package ai

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/biqly/biqly/internal/config"
)

const openAIProviderName = "openai"

// Client handles communication with LLM providers.
type Client struct {
	base   baseProvider
	topP   float64
	numCtx int
}

// NewClient creates a new AI client.
func NewClient(cfg config.AIConfig) *Client {
	baseURL := cfg.BaseURL
	if cfg.Provider == "openai" && baseURL == "" {
		baseURL = "https://api.openai.com/v1"
	}
	http := newHTTPProvider(cfg.AIHTTPTimeout(), baseURL, cfg.APIKey)
	c := &Client{
		topP:   cfg.TopP,
		numCtx: cfg.NumCtx,
	}
	c.base = baseProvider{
		http:        http,
		model:       cfg.Model,
		maxTokens:   cfg.MaxTokens,
		temperature: cfg.Temperature,
		logName:     openAIProviderName,
		hooks: providerHooks{
			path:    "/chat/completions",
			headers: func(p httpProvider) map[string]string { return p.bearerAuthHeaders() },
			marshal: c.marshalOpenAIRequest(cfg.Model, cfg.MaxTokens),
			parse:   parseOpenAIResponse,
		},
	}
	return c
}

type openAIRequest struct {
	Model       string          `json:"model"`
	Messages    []openAIMessage `json:"messages"`
	MaxTokens   int             `json:"max_tokens"`
	Temperature float64         `json:"temperature"`
	Options     *ollamaOptions  `json:"options,omitempty"`
}

type ollamaOptions struct {
	Temperature *float64 `json:"temperature,omitempty"`
	TopP        *float64 `json:"top_p,omitempty"`
	NumCtx      *int     `json:"num_ctx,omitempty"`
}

type openAIMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type openAIResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
	Usage *struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
		TotalTokens      int `json:"total_tokens"`
	} `json:"usage,omitempty"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

// Generate sends a prompt to the LLM at the configured temperature.
func (c *Client) Generate(ctx context.Context, prompt string) (GenerationResult, error) {
	return c.base.generate(ctx, prompt)
}

// GenerateAt sends a prompt with an explicit temperature override.
func (c *Client) GenerateAt(ctx context.Context, prompt string, temperature float64) (GenerationResult, error) {
	return c.base.generateAt(ctx, prompt, temperature)
}

func (c *Client) marshalOpenAIRequest(model string, maxTokens int) func(string, float64) ([]byte, error) {
	return func(prompt string, temperature float64) ([]byte, error) {
		reqBody := openAIRequest{
			Model: model,
			Messages: []openAIMessage{
				{Role: "system", Content: "You are a Business Intelligence query assistant. Output only valid JSON."},
				{Role: "user", Content: prompt},
			},
			MaxTokens:   maxTokens,
			Temperature: temperature,
			Options:     c.ollamaOptions(temperature),
		}
		return json.Marshal(reqBody)
	}
}

func parseOpenAIResponse(respBody []byte) (GenerationResult, error) {
	var aiResp openAIResponse
	if err := json.Unmarshal(respBody, &aiResp); err != nil {
		return GenerationResult{}, fmt.Errorf("unmarshal response: %w", err)
	}
	if aiResp.Error != nil {
		return GenerationResult{}, fmt.Errorf("API error: %s", aiResp.Error.Message)
	}
	if len(aiResp.Choices) == 0 {
		return GenerationResult{}, fmt.Errorf("no choices in response")
	}
	gen := GenerationResult{Content: aiResp.Choices[0].Message.Content}
	if aiResp.Usage != nil {
		gen.Usage = &TokenUsage{
			Prompt:     aiResp.Usage.PromptTokens,
			Completion: aiResp.Usage.CompletionTokens,
			Total:      aiResp.Usage.TotalTokens,
		}
	}
	return gen, nil
}

func (c *Client) ollamaOptions(temperature float64) *ollamaOptions {
	if c.topP <= 0 && c.numCtx <= 0 {
		return nil
	}
	opts := &ollamaOptions{Temperature: &temperature}
	if c.topP > 0 {
		opts.TopP = &c.topP
	}
	if c.numCtx > 0 {
		opts.NumCtx = &c.numCtx
	}
	return opts
}
