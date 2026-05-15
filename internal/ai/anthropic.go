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
		httpClient:  &http.Client{Timeout: cfg.AIHTTPTimeout()},
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
	return p.GenerateAt(ctx, prompt, p.temperature)
}

// GenerateAt sends a prompt with an explicit temperature override.
// Transient HTTP failures (429, 502–504) and network errors trigger a short
// exponential backoff retry (up to 4 attempts total).
func (p *AnthropicProvider) GenerateAt(ctx context.Context, prompt string, temperature float64) (GenerationResult, error) {
	estPrompt := EstimateTokens(prompt)
	reqBody := anthropicRequest{
		Model:       p.model,
		System:      anthropicSystemDirective,
		Messages:    []anthropicMessage{{Role: "user", Content: prompt}},
		MaxTokens:   p.maxTokens,
		Temperature: temperature,
	}
	body, err := json.Marshal(reqBody)
	if err != nil {
		return GenerationResult{}, fmt.Errorf("marshal request: %w", err)
	}

	result, err := execLLMHTTPRetry(ctx, func() (GenerationResult, error, bool) {
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.baseURL+"/messages", bytes.NewReader(body))
		if err != nil {
			return GenerationResult{}, fmt.Errorf("create request: %w", err), false
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("x-api-key", p.apiKey)
		req.Header.Set("anthropic-version", anthropicAPIVersion)

		resp, err := p.httpClient.Do(req)
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
			var ar anthropicResponse
			if err := json.Unmarshal(respBody, &ar); err != nil {
				return GenerationResult{}, fmt.Errorf("unmarshal response: %w", err), false
			}
			if ar.Error != nil {
				return GenerationResult{}, fmt.Errorf("Anthropic API error: %s", ar.Error.Message), false
			}
			var text string
			for _, c := range ar.Content {
				if c.Type == "text" && c.Text != "" {
					text = c.Text
					break
				}
			}
			if text == "" {
				return GenerationResult{}, fmt.Errorf("no text content in Anthropic response"), false
			}
			gen := GenerationResult{Content: text}
			if ar.Usage != nil {
				gen.Usage = &TokenUsage{
					Prompt:     ar.Usage.InputTokens,
					Completion: ar.Usage.OutputTokens,
					Total:      ar.Usage.InputTokens + ar.Usage.OutputTokens,
				}
			}
			return gen, nil, false
		}

		apiErr := fmt.Errorf("Anthropic API error %d: %s", resp.StatusCode, string(respBody))
		return GenerationResult{}, apiErr, isRetriableHTTPStatus(resp.StatusCode)
	})
	if err != nil {
		return GenerationResult{}, err
	}
	logLLMCompletion(ctx, "anthropic", p.model, estPrompt, result)
	return result, nil
}
