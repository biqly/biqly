package ai

import (
	"context"
	"strings"
	"testing"

	providerpkg "github.com/biqly/biqly/internal/ai/provider"
)

type fakeTranslationProvider struct {
	response string
	prompt   string
}

func (p *fakeTranslationProvider) Generate(ctx context.Context, prompt string) (providerpkg.GenerationResult, error) {
	return p.GenerateAt(ctx, prompt, 0)
}

func (p *fakeTranslationProvider) GenerateAt(ctx context.Context, prompt string, temperature float64) (providerpkg.GenerationResult, error) {
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
