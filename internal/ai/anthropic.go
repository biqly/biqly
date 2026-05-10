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

const (
	anthropicDefaultBaseURL  = "https://api.anthropic.com/v1"
	anthropicAPIVersion      = "2023-06-01"
	anthropicSystemDirective = "You are a Business Intelligence query assistant. Output only valid JSON."
)

// AnthropicProvider speaks Anthropic's Messages API. It mirrors the OpenAI
// adapter's contract so callers can swap providers via config alone.
type AnthropicProvider struct {
	httpClient  *http.Client
	baseURL     string
	apiKey      string
	model       string
	maxTokens   int
	temperature float64
}

// NewAnthropicProvider configures the Anthropic adapter from AIConfig.
func NewAnthropicProvider(cfg config.AIConfig) *AnthropicProvider {
	baseURL := cfg.BaseURL
	if baseURL == "" {
		baseURL = anthropicDefaultBaseURL
	}
	return &AnthropicProvider{
		httpClient:  &http.Client{Timeout: 60 * time.Second},
		baseURL:     baseURL,
		apiKey:      cfg.APIKey,
		model:       cfg.Model,
		maxTokens:   cfg.MaxTokens,
		temperature: cfg.Temperature,
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
	Error *struct {
		Type    string `json:"type"`
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

// Generate sends a prompt at the configured temperature.
func (p *AnthropicProvider) Generate(ctx context.Context, prompt string) (string, error) {
	return p.GenerateAt(ctx, prompt, p.temperature)
}

// GenerateAt sends a prompt with an explicit temperature override.
func (p *AnthropicProvider) GenerateAt(ctx context.Context, prompt string, temperature float64) (string, error) {
	reqBody := anthropicRequest{
		Model:       p.model,
		System:      anthropicSystemDirective,
		Messages:    []anthropicMessage{{Role: "user", Content: prompt}},
		MaxTokens:   p.maxTokens,
		Temperature: temperature,
	}
	body, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.baseURL+"/messages", bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", p.apiKey)
	req.Header.Set("anthropic-version", anthropicAPIVersion)

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("send request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("Anthropic API error %d: %s", resp.StatusCode, string(respBody))
	}

	var ar anthropicResponse
	if err := json.Unmarshal(respBody, &ar); err != nil {
		return "", fmt.Errorf("unmarshal response: %w", err)
	}
	if ar.Error != nil {
		return "", fmt.Errorf("Anthropic API error: %s", ar.Error.Message)
	}
	for _, c := range ar.Content {
		if c.Type == "text" && c.Text != "" {
			return c.Text, nil
		}
	}
	return "", fmt.Errorf("no text content in Anthropic response")
}
