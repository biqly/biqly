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
}

// NewClient creates a new AI client.
func NewClient(cfg config.AIConfig) *Client {
	baseURL := cfg.BaseURL
	if cfg.Provider == "openai" && baseURL == "" {
		baseURL = "https://api.openai.com/v1"
	}

	return &Client{
		httpClient: &http.Client{
			Timeout: 60 * time.Second,
		},
		baseURL:     baseURL,
		apiKey:      cfg.APIKey,
		model:       cfg.Model,
		maxTokens:   cfg.MaxTokens,
		temperature: cfg.Temperature,
	}
}

type openAIRequest struct {
	Model       string          `json:"model"`
	Messages    []openAIMessage `json:"messages"`
	MaxTokens   int             `json:"max_tokens"`
	Temperature float64         `json:"temperature"`
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
func (c *Client) GenerateAt(ctx context.Context, prompt string, temperature float64) (string, error) {
	reqBody := openAIRequest{
		Model: c.model,
		Messages: []openAIMessage{
			{Role: "system", Content: "You are a Business Intelligence query assistant. Output only valid JSON."},
			{Role: "user", Content: prompt},
		},
		MaxTokens:   c.maxTokens,
		Temperature: temperature,
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.apiKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("send request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("API error %d: %s", resp.StatusCode, string(respBody))
	}

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
