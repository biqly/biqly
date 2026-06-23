package ai

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/bytedance/sonic"

	providerpkg "github.com/biqly/biqly/internal/ai/provider"
	"github.com/biqly/biqly/internal/config"
)

type fakeTranslationProvider struct {
	response string
	prompt   string
}

func (p *fakeTranslationProvider) Generate(ctx context.Context, prompt string) (providerpkg.GenerationResult, error) {
	return p.GenerateAt(ctx, prompt, 0)
}

func (p *fakeTranslationProvider) GenerateAt(_ context.Context, prompt string, _ float64) (providerpkg.GenerationResult, error) {
	p.prompt = prompt
	return providerpkg.GenerationResult{Content: p.response}, nil
}

func TestTranslationServiceTranslateDescribeResult(t *testing.T) {
	provider := &fakeTranslationProvider{
		response: `{
			"table_description": "Müşteri siparişlerinin iş süreci görünümü.",
			"columns": [
				{"name": "CustomerID", "description": "Müşteri kimliği."},
				{"name": "OrderDate", "description": "Siparişin oluşturulduğu tarih."}
			]
		}`,
	}
	service := NewTranslationService(provider, "translategemma:4b", "Turkish", "tr")
	result := &DescribeResult{
		Description: "Business view of customer orders.",
		Columns: []ColumnDescription{
			{Name: "CustomerID", Description: "Customer identifier."},
			{Name: "OrderDate", Description: "Date when the order was created."},
		},
	}

	if err := service.TranslateDescribeResult(context.Background(), result); err != nil {
		t.Fatalf("translate describe result: %v", err)
	}

	if result.Description != "Müşteri siparişlerinin iş süreci görünümü." {
		t.Fatalf("translated table description mismatch: %q", result.Description)
	}
	if result.Columns[0].Name != "CustomerID" || result.Columns[0].Description != "Müşteri kimliği." {
		t.Fatalf("translated first column mismatch: %+v", result.Columns[0])
	}
	if !result.TranslationApplied {
		t.Fatal("translation should be marked as applied")
	}
	if result.TranslationModel != "translategemma:4b" {
		t.Fatalf("translation model mismatch: %q", result.TranslationModel)
	}
	if !strings.Contains(provider.prompt, "Do not change column \"name\" values") {
		t.Fatalf("prompt should instruct identifier preservation, got: %s", provider.prompt)
	}
}

func TestTranslationServiceTranslateFields(t *testing.T) {
	provider := &fakeTranslationProvider{
		response: `{
			"fields": [
				{"key": "ignored-by-model", "label": "Yer İmi Sayısı", "description": "Kaç kez kaydedildiği."},
				{"key": "x", "label": "Yazar Adı", "description": ""}
			]
		}`,
	}
	service := NewTranslationService(provider, "m", "Turkish", "tr")

	in := []TranslatableField{
		{Key: "dim-1", Label: "Bookmark Count", Description: "How many times bookmarked."},
		{Key: "dim-2", Label: "Author Name", Description: ""},
	}
	out, err := service.TranslateFields(context.Background(), in)
	if err != nil {
		t.Fatalf("translate fields: %v", err)
	}
	if len(out) != 2 {
		t.Fatalf("want 2 fields, got %d", len(out))
	}
	// Keys are re-applied by index regardless of what the model echoed back.
	if out[0].Key != "dim-1" || out[1].Key != "dim-2" {
		t.Fatalf("keys not preserved by index: %+v", out)
	}
	if out[0].Label != "Yer İmi Sayısı" || out[0].Description != "Kaç kez kaydedildiği." {
		t.Fatalf("first field translation mismatch: %+v", out[0])
	}
	if !strings.Contains(provider.prompt, "Do not change \"key\" values") {
		t.Fatalf("prompt should instruct key preservation, got: %s", provider.prompt)
	}
}

func TestTranslationServiceTranslateFieldsEmpty(t *testing.T) {
	out, err := (*TranslationService)(nil).TranslateFields(context.Background(), nil)
	if err != nil {
		t.Fatalf("nil service should be a no-op, got: %v", err)
	}
	if out != nil {
		t.Fatalf("want nil passthrough, got %+v", out)
	}
}

func TestNewTranslationServiceFromConfig_DefaultMaxTokens(t *testing.T) {
	var gotMaxTokens int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("read body: %v", err)
		}
		var req struct {
			MaxTokens int `json:"max_tokens"`
		}
		if err := sonic.ConfigStd.Unmarshal(body, &req); err != nil {
			t.Fatalf("unmarshal request: %v", err)
		}
		gotMaxTokens = req.MaxTokens
		if err := sonic.ConfigStd.NewEncoder(w).Encode(map[string]any{
			"choices": []map[string]any{
				{"message": map[string]string{"content": `{"table_description":"Siparişler","columns":[{"name":"ID","description":"Tanımlayıcı"}]}`}},
			},
		}); err != nil {
			t.Fatalf("encode response: %v", err)
		}
	}))
	defer srv.Close()

	cfg := config.AIConfig{
		Translation: config.TranslationConfig{
			Model:   "translategemma:4b",
			BaseURL: srv.URL,
			APIKey:  "test",
		},
		Generation: config.AIGenerationConfig{MaxTokens: 0},
	}
	service := NewTranslationServiceFromConfig(cfg)
	if service == nil {
		t.Fatal("expected translation service")
	}
	if err := service.TranslateDescribeResult(context.Background(), &DescribeResult{
		Description: "Orders",
		Columns:     []ColumnDescription{{Name: "ID", Description: "Identifier"}},
	}); err != nil {
		t.Fatalf("translate: %v", err)
	}
	if gotMaxTokens != defaultTranslationMaxTokens {
		t.Fatalf("max_tokens = %d, want %d", gotMaxTokens, defaultTranslationMaxTokens)
	}
}

func TestEffectiveTranslationMaxTokens(t *testing.T) {
	if got := effectiveTranslationMaxTokens(2048); got != 2048 {
		t.Fatalf("positive max_tokens: got %d", got)
	}
	if got := effectiveTranslationMaxTokens(0); got != defaultTranslationMaxTokens {
		t.Fatalf("zero max_tokens: got %d, want %d", got, defaultTranslationMaxTokens)
	}
}

func TestTranslationServiceRejectsIdentifierChanges(t *testing.T) {
	provider := &fakeTranslationProvider{
		response: `{
			"table_description": "Çevrilmiş açıklama.",
			"columns": [
				{"name": "MusteriID", "description": "Müşteri kimliği."}
			]
		}`,
	}
	service := NewTranslationService(provider, "translategemma:4b", "Turkish", "tr")
	result := &DescribeResult{
		Description: "Customer table.",
		Columns:     []ColumnDescription{{Name: "CustomerID", Description: "Customer identifier."}},
	}

	err := service.TranslateDescribeResult(context.Background(), result)
	if err == nil {
		t.Fatal("expected identifier validation error")
	}
	if result.Columns[0].Name != "CustomerID" || result.Columns[0].Description != "Customer identifier." {
		t.Fatalf("failed translation should preserve original result, got %+v", result.Columns[0])
	}
	if result.TranslationApplied {
		t.Fatal("failed translation must not be marked as applied")
	}
}
