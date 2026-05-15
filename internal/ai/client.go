// Package ai provides AI-powered schema description and prompt building.
package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/biqly/biqly/internal/config"
)

const openAIProviderName = "openai"

// Client handles communication with LLM providers.
type Client struct {
	httpClient  *http.Client
	baseURL     string
	apiKey      string
	model       string
	maxTokens   int
	temperature float64
	topP        float64
	numCtx      int
}

// NewClient creates a new AI client.
func NewClient(cfg config.AIConfig) *Client {
	baseURL := cfg.BaseURL
	if cfg.Provider == "openai" && baseURL == "" {
		baseURL = "https://api.openai.com/v1"
	}

	return &Client{
		httpClient: &http.Client{
			Timeout: cfg.AIHTTPTimeout(),
		},
		baseURL:     baseURL,
		apiKey:      cfg.APIKey,
		model:       cfg.Model,
		maxTokens:   cfg.MaxTokens,
		temperature: cfg.Temperature,
		topP:        cfg.TopP,
		numCtx:      cfg.NumCtx,
	}
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
	return c.GenerateAt(ctx, prompt, c.temperature)
}

// GenerateAt sends a prompt with an explicit temperature override.
// Transient HTTP failures (429, 502–504) and network errors trigger a short
// exponential backoff retry (up to 4 attempts total).
func (c *Client) GenerateAt(ctx context.Context, prompt string, temperature float64) (GenerationResult, error) {
	estPrompt := EstimateTokens(prompt)
	reqBody := openAIRequest{
		Model: c.model,
		Messages: []openAIMessage{
			{Role: "system", Content: "You are a Business Intelligence query assistant. Output only valid JSON."},
			{Role: "user", Content: prompt},
		},
		MaxTokens:   c.maxTokens,
		Temperature: temperature,
		Options:     c.ollamaOptions(temperature),
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return GenerationResult{}, fmt.Errorf("marshal request: %w", err)
	}

	result, err := execLLMHTTPRetry(ctx, func() (GenerationResult, error, bool) {
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/chat/completions", bytes.NewReader(body))
		if err != nil {
			return GenerationResult{}, fmt.Errorf("create request: %w", err), false
		}
		req.Header.Set("Content-Type", "application/json")
		if c.apiKey != "" {
			req.Header.Set("Authorization", "Bearer "+c.apiKey)
		}

		resp, err := c.httpClient.Do(req)
		if err != nil {
			return GenerationResult{}, fmt.Errorf("send request: %w", err), isRetriableNetErr(err)
		}

		respBody, readErr := io.ReadAll(resp.Body)
		closeErr := resp.Body.Close()
		if readErr != nil {
			return GenerationResult{}, fmt.Errorf("read response: %w", readErr), false
		}
		if closeErr != nil {
			return GenerationResult{}, fmt.Errorf("close response: %w", closeErr), false
		}

		if resp.StatusCode == http.StatusOK {
			var aiResp openAIResponse
			if err := json.Unmarshal(respBody, &aiResp); err != nil {
				return GenerationResult{}, fmt.Errorf("unmarshal response: %w", err), false
			}
			if aiResp.Error != nil {
				return GenerationResult{}, fmt.Errorf("API error: %s", aiResp.Error.Message), false
			}
			if len(aiResp.Choices) == 0 {
				return GenerationResult{}, fmt.Errorf("no choices in response"), false
			}
			gen := GenerationResult{Content: aiResp.Choices[0].Message.Content}
			if aiResp.Usage != nil {
				gen.Usage = &TokenUsage{
					Prompt:     aiResp.Usage.PromptTokens,
					Completion: aiResp.Usage.CompletionTokens,
					Total:      aiResp.Usage.TotalTokens,
				}
			}
			return gen, nil, false
		}

		apiErr := fmt.Errorf("API error %d: %s", resp.StatusCode, string(respBody))
		return GenerationResult{}, apiErr, isRetriableHTTPStatus(resp.StatusCode)
	})
	if err != nil {
		return GenerationResult{}, err
	}
	logLLMCompletion(ctx, openAIProviderName, c.model, estPrompt, result)
	return result, nil
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
