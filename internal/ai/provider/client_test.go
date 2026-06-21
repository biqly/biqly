package provider

import (
	"context"
	"github.com/bytedance/sonic"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	"github.com/biqly/biqly/internal/ai/prompt"
	"github.com/biqly/biqly/internal/config"
)

func TestClientGenerateAtParsesOpenAIUsage(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if err := sonic.ConfigStd.NewEncoder(w).Encode(map[string]any{
			"choices": []map[string]any{
				{"message": map[string]string{"content": `{"limit":10}`}},
			},
			"usage": map[string]int{
				"prompt_tokens":     120,
				"completion_tokens": 30,
				"total_tokens":      150,
			},
		}); err != nil {
			t.Fatalf("encode response: %v", err)
		}
	}))
	defer srv.Close()

	client := NewClient(config.AIConfig{
		Connection: config.AIConnectionConfig{
			BaseURL: srv.URL,
			APIKey:  "x",
			Model:   "gpt-test",
		},
	})
	gen, err := client.GenerateAt(context.Background(), "list orders", 0)
	if err != nil {
		t.Fatalf("GenerateAt: %v", err)
	}
	if gen.Content != `{"limit":10}` {
		t.Fatalf("content = %q", gen.Content)
	}
	if gen.Usage == nil {
		t.Fatal("expected usage from API")
	}
	if gen.Usage.Prompt != 120 || gen.Usage.Completion != 30 || gen.Usage.Total != 150 {
		t.Fatalf("usage = %+v", gen.Usage)
	}
}

func TestAnthropicGenerateAtParsesUsage(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if err := sonic.ConfigStd.NewEncoder(w).Encode(map[string]any{
			"content": []map[string]string{
				{"type": "text", "text": `{"limit":5}`},
			},
			"usage": map[string]int{
				"input_tokens":  80,
				"output_tokens": 20,
			},
		}); err != nil {
			t.Fatalf("encode response: %v", err)
		}
	}))
	defer srv.Close()

	p := NewAnthropicProvider(config.AIConfig{
		Connection: config.AIConnectionConfig{
			BaseURL: srv.URL,
			APIKey:  "x",
			Model:   "claude-test",
		},
	})
	gen, err := p.GenerateAt(context.Background(), "count rows", 0)
	if err != nil {
		t.Fatalf("GenerateAt: %v", err)
	}
	if gen.Usage == nil {
		t.Fatal("expected usage from API")
	}
	if gen.Usage.Prompt != 80 || gen.Usage.Completion != 20 || gen.Usage.Total != 100 {
		t.Fatalf("usage = %+v", gen.Usage)
	}
}

func TestTokenUsageFromGenerationPrefersAPI(t *testing.T) {
	stats := prompt.Stats{EstPromptTokens: 999}
	api := GenerationResult{
		Content: "x",
		Usage:   &TokenUsage{Prompt: 10, Completion: 5, Total: 15},
	}
	got := TokenUsageFromGeneration(stats, api)
	want := &TokenUsage{Prompt: 10, Completion: 5, Total: 15}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("TokenUsageFromGeneration API usage = %+v, want %+v", got, want)
	}

	est := TokenUsageFromGeneration(stats, GenerationResult{Content: "abcdabcd"})
	wantEstimate := &TokenUsage{Prompt: 999, Completion: 2, Total: 1001}
	if !reflect.DeepEqual(est, wantEstimate) {
		t.Fatalf("TokenUsageFromGeneration estimate fallback = %+v, want %+v", est, wantEstimate)
	}
}

func TestClientGenerate(t *testing.T) {
	// Test Client.Generate() which delegates through baseProvider.generate()
	// and covers the otherwise-untested generate() wrapper and Client.Generate().
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if err := sonic.ConfigStd.NewEncoder(w).Encode(map[string]any{
			"choices": []map[string]any{
				{"message": map[string]string{"content": `{"limit":10}`}},
			},
			"usage": map[string]int{
				"prompt_tokens":     120,
				"completion_tokens": 30,
				"total_tokens":      150,
			},
		}); err != nil {
			t.Fatalf("encode response: %v", err)
		}
	}))
	defer srv.Close()

	client := NewClient(config.AIConfig{
		Connection: config.AIConnectionConfig{
			BaseURL: srv.URL,
			APIKey:  "x",
			Model:   "gpt-test",
		},
	})
	// Call Generate (not GenerateAt) to cover the 0% path.
	gen, err := client.Generate(context.Background(), "list orders")
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if gen.Content != `{"limit":10}` {
		t.Fatalf("content = %q", gen.Content)
	}
}

func TestClientGenerate_BadRequest(t *testing.T) {
	// Test the early-return path in generateAt where the API returns a non-OK
	// status (400 Bad Request is non-retriable, so it errors immediately).
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":"bad input"}`))
	}))
	defer srv.Close()

	client := NewClient(config.AIConfig{
		Connection: config.AIConnectionConfig{
			BaseURL: srv.URL,
			APIKey:  "x",
			Model:   "gpt-test",
		},
	})
	_, err := client.Generate(context.Background(), "bad request test")
	if err == nil {
		t.Fatal("expected error for 400 response, got nil")
	}
}

func TestClientGenerate_ParseError(t *testing.T) {
	// Test the early-return path in generateAt where HTTP 200 is returned but
	// the response body contains invalid data that the parse hook cannot handle.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		// Valid JSON but missing required "choices" field — parseOpenAIResponse
		// will return "no choices in response".
		if err := sonic.ConfigStd.NewEncoder(w).Encode(map[string]any{
			"error": map[string]string{"message": "no choices"},
		}); err != nil {
			t.Fatalf("encode response: %v", err)
		}
	}))
	defer srv.Close()

	client := NewClient(config.AIConfig{
		Connection: config.AIConnectionConfig{
			BaseURL: srv.URL,
			APIKey:  "x",
			Model:   "gpt-test",
		},
	})
	_, err := client.Generate(context.Background(), "parse error test")
	if err == nil {
		t.Fatal("expected error for bad response body, got nil")
	}
}

func TestClientGenerate_NoAPIKey(t *testing.T) {
	// When APIKey is empty, bearerAuthHeaders() returns nil, exercising the
	// "headers == nil" branch in generateAt.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if err := sonic.ConfigStd.NewEncoder(w).Encode(map[string]any{
			"choices": []map[string]any{
				{"message": map[string]string{"content": "ok"}},
			},
		}); err != nil {
			t.Fatalf("encode response: %v", err)
		}
	}))
	defer srv.Close()

	client := NewClient(config.AIConfig{
		Connection: config.AIConnectionConfig{
			BaseURL: srv.URL,
			APIKey:  "", // empty — triggers headers == nil path
			Model:   "gpt-test",
		},
	})
	gen, err := client.Generate(context.Background(), "no key test")
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if gen.Content != "ok" {
		t.Fatalf("content = %q", gen.Content)
	}
}

func TestParseOpenAIResponseReasoningFallback(t *testing.T) {
	tests := []struct {
		name        string
		body        string
		wantContent string
		wantFinish  string
	}{
		{
			name:        "empty content falls back to reasoning_content",
			body:        `{"choices":[{"message":{"content":"","reasoning_content":"thinking... {\"limit\":5}"},"finish_reason":"length"}]}`,
			wantContent: `thinking... {"limit":5}`,
			wantFinish:  "length",
		},
		{
			name:        "empty content falls back to reasoning",
			body:        `{"choices":[{"message":{"content":"","reasoning":"{\"limit\":7}"},"finish_reason":"stop"}]}`,
			wantContent: `{"limit":7}`,
			wantFinish:  "stop",
		},
		{
			name:        "content wins over reasoning channels",
			body:        `{"choices":[{"message":{"content":"{\"limit\":1}","reasoning_content":"ignored"},"finish_reason":"stop"}]}`,
			wantContent: `{"limit":1}`,
			wantFinish:  "stop",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gen, err := parseOpenAIResponse([]byte(tt.body))
			if err != nil {
				t.Fatalf("parseOpenAIResponse: %v", err)
			}
			if gen.Content != tt.wantContent {
				t.Errorf("Content = %q, want %q", gen.Content, tt.wantContent)
			}
			if gen.FinishReason != tt.wantFinish {
				t.Errorf("FinishReason = %q, want %q", gen.FinishReason, tt.wantFinish)
			}
		})
	}
}
