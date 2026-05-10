// Package ai provides AI-powered schema description and prompt building.
package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/biqly/biqly/internal/config"
)

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
	Error *struct {
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

// Generate sends a prompt to the LLM at the configured temperature.
func (c *Client) Generate(ctx context.Context, prompt string) (string, error) {
	return c.GenerateAt(ctx, prompt, c.temperature)
}

// GenerateAt sends a prompt with an explicit temperature override.
// Transient HTTP failures (429, 502–504) and network errors trigger a short
// exponential backoff retry (up to 4 attempts total).
func (c *Client) GenerateAt(ctx context.Context, prompt string, temperature float64) (string, error) {
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
		return "", fmt.Errorf("marshal request: %w", err)
	}

	const maxAttempts = 4
	var lastErr error
	for attempt := 0; attempt < maxAttempts; attempt++ {
		if attempt > 0 {
			delay := time.Duration(250*(1<<uint(attempt-1))) * time.Millisecond
			if err := sleepCtx(ctx, delay); err != nil {
				return "", err
			}
		}

		req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/chat/completions", bytes.NewReader(body))
		if err != nil {
			return "", fmt.Errorf("create request: %w", err)
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+c.apiKey)

		resp, err := c.httpClient.Do(req)
		if err != nil {
			lastErr = err
			if attempt+1 < maxAttempts && isRetriableNetErr(err) {
				continue
			}
			return "", fmt.Errorf("send request: %w", err)
		}

		respBody, readErr := io.ReadAll(resp.Body)
		closeErr := resp.Body.Close()
		if readErr != nil {
			return "", fmt.Errorf("read response: %w", readErr)
		}
		if closeErr != nil {
			return "", fmt.Errorf("close response: %w", closeErr)
		}

		if resp.StatusCode == http.StatusOK {
			var aiResp openAIResponse
			if err := json.Unmarshal(respBody, &aiResp); err != nil {
				return "", fmt.Errorf("unmarshal response: %w", err)
			}
			if aiResp.Error != nil {
				return "", fmt.Errorf("API error: %s", aiResp.Error.Message)
			}
			if len(aiResp.Choices) == 0 {
				return "", fmt.Errorf("no choices in response")
			}
			return aiResp.Choices[0].Message.Content, nil
		}

		lastErr = fmt.Errorf("API error %d: %s", resp.StatusCode, string(respBody))
		if attempt+1 < maxAttempts && isRetriableHTTPStatus(resp.StatusCode) {
			continue
		}
		return "", lastErr
	}
	return "", fmt.Errorf("send request: %w", lastErr)
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
