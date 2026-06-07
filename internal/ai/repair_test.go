package ai

import (
	"context"
	"strings"
	"testing"

	"github.com/biqly/biqly/internal/config"
	"github.com/biqly/biqly/internal/query"
	"github.com/biqly/biqly/internal/semantic"
	"github.com/stretchr/testify/assert"
)

func TestProcessQuestionRepairLoop(t *testing.T) {
	// First response contains an unknown field, second response corrects it.
	replies := []string{
		`{"select":[{"type":"metric","name":"ciro"}],"limit":10}`,
		`{"select":[{"type":"metric","name":"gross_revenue"}],"limit":10}`,
	}
	client := &scriptedProvider{replies: replies}

	cfg := config.AIConfig{
		Connection: config.AIConnectionConfig{Provider: "test", Model: "test"},
		Generation: config.AIGenerationConfig{MaxRetries: 2},
	}
	svc := NewServiceWithProvider(&cfg, query.NewValidator(100), client)

	model := &semantic.SemanticModel{
		ID:           "model-id",
		DatasourceID: "ds-id",
		Name:         "orders",
		Metrics: []semantic.Metric{
			{Name: "gross_revenue", Expression: "amount", Aggregation: "sum"},
		},
	}

	resp, err := svc.ProcessQuestion(context.Background(), "ciroyu göster", model)
	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.NotNil(t, resp.Result)
	assert.NotNil(t, resp.Result.LogicalQuery)
	assert.Equal(t, "gross_revenue", resp.Result.LogicalQuery.Select[0].Name)
	assert.Equal(t, 1, resp.Metadata.RetryCount)
	// Verify that the prompt used for attempt 1 (retry 1) contains the structured error repair hints.
	assert.True(t, strings.Contains(resp.Metadata.Prompt, "gross_revenue"))
}
