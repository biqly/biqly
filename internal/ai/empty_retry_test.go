package ai

import (
	"context"
	stderrors "errors"
	"strings"
	"testing"

	"github.com/biqly/biqly/internal/ai/prompt"
	providerpkg "github.com/biqly/biqly/internal/ai/provider"
	"github.com/biqly/biqly/internal/config"
	"github.com/biqly/biqly/internal/i18n"
	"github.com/biqly/biqly/internal/query"
	"github.com/biqly/biqly/internal/semantic"
)

type recordingProvider struct {
	replies []string
	prompts []string
	calls   int
}

func (p *recordingProvider) Generate(_ context.Context, promptStr string) (providerpkg.GenerationResult, error) {
	idx := p.calls
	p.calls++
	p.prompts = append(p.prompts, promptStr)
	if idx >= len(p.replies) {
		idx = len(p.replies) - 1
	}
	return providerpkg.GenerationResult{Content: p.replies[idx]}, nil
}

func (p *recordingProvider) GenerateAt(ctx context.Context, promptStr string, _ float64) (providerpkg.GenerationResult, error) {
	return p.Generate(ctx, promptStr)
}

func TestParseAndValidateEmptyResponseIsSentinel(t *testing.T) {
	service := &Service{validator: query.NewValidator(1000)}
	_, _, _, _, err := service.parseAndValidate("   ", &semantic.SemanticModel{})
	if !stderrors.Is(err, ErrEmptyAIResponse) {
		t.Fatalf("parseAndValidate empty err = %v, want errors.Is ErrEmptyAIResponse", err)
	}
}

func TestBuildNextAttemptPromptEmptyResponseUsesJSONOnly(t *testing.T) {
	svc := &Service{promptBuilder: &prompt.Builder{}}

	emptyPrompt := svc.buildNextAttemptPrompt(context.Background(), i18n.DefaultLocale, "ORIGINAL PROMPT", "", "empty AI response", nil, 0, &genLoopState{}, true)
	if !strings.Contains(emptyPrompt, "Output ONLY the LogicalQuery JSON object") {
		t.Fatalf("empty-response retry prompt missing JSON-only emphasis:\n%s", emptyPrompt)
	}
	if !strings.Contains(emptyPrompt, "Start your response with `{`") {
		t.Errorf("empty-response retry prompt missing start-with-brace instruction:\n%s", emptyPrompt)
	}

	genericPrompt := svc.buildNextAttemptPrompt(context.Background(), i18n.DefaultLocale, "ORIGINAL PROMPT", "bad", "some parse error", nil, 0, &genLoopState{}, false)
	if strings.Contains(genericPrompt, "Output ONLY the LogicalQuery JSON object") {
		t.Errorf("generic retry prompt should not use the empty-response emphasis:\n%s", genericPrompt)
	}
}

func TestProcessQuestionEmptyResponseRetriesWithJSONOnlyCompactTier(t *testing.T) {
	provider := &recordingProvider{replies: []string{
		"", // blank completion triggers the empty-response branch
		`{"select":[{"type":"metric","name":"row_count"}],"limit":10}`,
	}}
	cfg := config.AIConfig{Generation: config.AIGenerationConfig{MaxRetries: 2}}
	svc := NewServiceWithProvider(&cfg, query.NewValidator(1000), provider)

	model := &semantic.SemanticModel{
		ID:           "model-uuid",
		DatasourceID: "ds-uuid",
		Name:         "public.orders",
		Metrics:      []semantic.Metric{{Name: "row_count", Aggregation: "count", Expression: "*"}},
	}

	resp, err := svc.ProcessQuestion(context.Background(), "how many orders are there", model)
	resp = requireProcessQuestionResponse(t, resp, err)
	if resp.Result == nil || resp.Result.LogicalQuery == nil {
		t.Fatalf("expected LogicalQuery after empty-response retry, got %+v", resp.Result)
	}
	if provider.calls < 2 {
		t.Fatalf("provider calls = %d, want >= 2 (retry after empty)", provider.calls)
	}
	if !strings.Contains(provider.prompts[1], "Output ONLY the LogicalQuery JSON object") {
		t.Fatalf("retry prompt after empty response missing JSON-only emphasis:\n%s", provider.prompts[1])
	}
}
