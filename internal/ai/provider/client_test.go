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
		BaseURL: srv.URL,
		APIKey:  "x",
		Model:   "gpt-test",
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
		BaseURL: srv.URL,
		APIKey:  "x",
		Model:   "claude-test",
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
