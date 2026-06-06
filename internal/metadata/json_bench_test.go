package metadata

import (
	"testing"
	"time"

	"github.com/bytedance/sonic"
	jsoniter "github.com/json-iterator/go"
)

var jsoniterStd = jsoniter.ConfigCompatibleWithStandardLibrary

func getTestEntry() *AIQueryHistoryEntry {
	modelID := "gpt-4o"
	userID := "user-12345"
	score := 0.95
	latency := 1250
	tokens := 450
	compTokens := 150
	totTokens := 600
	cost := 0.003

	return &AIQueryHistoryEntry{
		ID:                 "hist-99f83a",
		DatasourceID:       "ds-88223",
		ModelID:            &modelID,
		UserID:             &userID,
		Question:           "What are the total sales by country last month?",
		PromptContext:      map[string]any{"tables": []string{"sales", "countries"}, "limit": 10},
		AIResponse:         map[string]any{"sql": "SELECT SUM(amount) FROM sales GROUP BY country", "explanation": "Grouped by country"},
		LogicalQuery:       map[string]any{"select": []any{"sales"}, "filter": "last_month"},
		ConfidenceScore:    &score,
		Warnings:           []string{"low-confidence-join", "missing-filter"},
		OutcomeStatus:      "success",
		RetryCount:         0,
		NeedsClarification: false,
		ModelUsed:          &modelID,
		PromptTokens:       &tokens,
		CompletionTokens:   &compTokens,
		TokenCount:         &totTokens,
		CostUSD:            &cost,
		LatencyMs:          &latency,
		CreatedAt:          time.Now(),
	}
}

func BenchmarkJSONMarshal_Std(b *testing.B) {
	entry := getTestEntry()
	b.ResetTimer()
	for b.Loop() {
		_, err := sonic.ConfigStd.Marshal(entry)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkJSONMarshal_Jsoniter(b *testing.B) {
	entry := getTestEntry()
	b.ResetTimer()
	for b.Loop() {
		_, err := jsoniterStd.Marshal(entry)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkJSONMarshal_Sonic(b *testing.B) {
	entry := getTestEntry()
	b.ResetTimer()
	for b.Loop() {
		_, err := sonic.Marshal(entry)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkJSONUnmarshal_Std(b *testing.B) {
	entry := getTestEntry()
	data, err := sonic.ConfigStd.Marshal(entry)
	if err != nil {
		b.Fatal(err)
	}

	var out AIQueryHistoryEntry
	b.ResetTimer()
	for b.Loop() {
		err := sonic.ConfigStd.Unmarshal(data, &out)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkJSONUnmarshal_Jsoniter(b *testing.B) {
	entry := getTestEntry()
	data, err := jsoniterStd.Marshal(entry)
	if err != nil {
		b.Fatal(err)
	}

	var out AIQueryHistoryEntry
	b.ResetTimer()
	for b.Loop() {
		err := jsoniterStd.Unmarshal(data, &out)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkJSONUnmarshal_Sonic(b *testing.B) {
	entry := getTestEntry()
	data, err := sonic.Marshal(entry)
	if err != nil {
		b.Fatal(err)
	}

	var out AIQueryHistoryEntry
	b.ResetTimer()
	for b.Loop() {
		err := sonic.Unmarshal(data, &out)
		if err != nil {
			b.Fatal(err)
		}
	}
}
