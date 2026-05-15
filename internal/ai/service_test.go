package ai

import (
	"context"
	"encoding/json"
	stderrors "errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/biqly/biqly/internal/config"
	"github.com/biqly/biqly/internal/query"
	"github.com/biqly/biqly/internal/semantic"
)

var errPlannerSyntax = stderrors.New("planner: syntax error near 'FROO'")

func TestParseAndValidateNormalizesLogicalQueryContext(t *testing.T) {
	service := &Service{validator: query.NewValidator(1000)}
	model := &semantic.SemanticModel{
		ID:           "model-uuid",
		DatasourceID: "datasource-uuid",
		Name:         "public.orders",
		Metrics: []semantic.Metric{
			{Name: "first_order_created_at", Expression: "orders.created_at", Aggregation: "min"},
		},
	}
	raw := `{"datasource_id":"","model_id":"","select":[{"type":"metric","name":"first_order_created_at"}],"limit":100}`

	got, _, _, err := service.parseAndValidate(raw, model)
	if err != nil {
		t.Fatalf("parseAndValidate(%s) error = %v, want nil", raw, err)
	}
	if got.DatasourceID != model.DatasourceID {
		t.Errorf("parseAndValidate(%s).DatasourceID = %q, want %q", raw, got.DatasourceID, model.DatasourceID)
	}
	if got.ModelID != model.Name {
		t.Errorf("parseAndValidate(%s).ModelID = %q, want %q", raw, got.ModelID, model.Name)
	}
}

func TestParseAndValidateAddsMissingGroupByDimensionToSelect(t *testing.T) {
	service := &Service{validator: query.NewValidator(1000)}
	model := &semantic.SemanticModel{
		ID:           "model-uuid",
		DatasourceID: "datasource-uuid",
		Name:         "public.timeline_tweets",
		Dimensions: []semantic.Dimension{
			{Name: "created_at_ts_day", ColumnRef: "timeline_tweets.created_at_ts", Type: "timestamp", TimeGrain: query.TimeGrainDay},
		},
		Metrics: []semantic.Metric{
			{Name: "row_count", Expression: "*", Aggregation: "count"},
			{Name: "sum_retweets", Expression: "timeline_tweets.retweets", Aggregation: "sum"},
		},
	}
	raw := `{"select":[{"type":"metric","name":"row_count"},{"type":"metric","name":"sum_retweets"}],"group_by":[{"field":"created_at_ts_day"}],"limit":100}`

	got, _, _, err := service.parseAndValidate(raw, model)
	if err != nil {
		t.Fatalf("parseAndValidate(%s) error = %v, want nil", raw, err)
	}
	if len(got.Select) != 3 {
		t.Fatalf("select len = %d, want 3: %+v", len(got.Select), got.Select)
	}
	if got.Select[0].Type != query.SelectTypeDimension || got.Select[0].Name != "created_at_ts_day" {
		t.Fatalf("first select = %+v, want created_at_ts_day dimension", got.Select[0])
	}
}

// stubLLMServer returns an httptest.Server whose /chat/completions endpoint
// emits successive responses from `replies`, cycling on the last one.
func stubLLMServer(t *testing.T, replies []string) *httptest.Server {
	t.Helper()
	idx := 0
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		reply := replies[len(replies)-1]
		if idx < len(replies) {
			reply = replies[idx]
		}
		idx++
		body := map[string]any{
			"choices": []map[string]any{
				{"message": map[string]string{"content": reply}},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(body)
	}))
}

func TestProcessQuestionRetriesOnInvalidJSON(t *testing.T) {
	srv := stubLLMServer(t, []string{
		"this is not json",
		`{"select":[{"type":"metric","name":"row_count"}],"limit":10}`,
	})
	defer srv.Close()

	cfg := config.AIConfig{Provider: "openai", BaseURL: srv.URL, APIKey: "x", Model: "test", MaxRetries: 2}
	svc := NewService(cfg, query.NewValidator(1000))

	model := &semantic.SemanticModel{
		ID:           "model-uuid",
		DatasourceID: "ds-uuid",
		Name:         "public.orders",
		Metrics:      []semantic.Metric{{Name: "row_count", Aggregation: "count", Expression: "*"}},
	}

	resp, err := svc.ProcessQuestion(context.Background(), "kaç sipariş var", model)
	if err != nil {
		t.Fatalf("ProcessQuestion error = %v, want nil", err)
	}
	if resp.LogicalQuery == nil {
		t.Fatalf("expected LogicalQuery after successful retry, got nil; warnings=%v", resp.Warnings)
	}
	if resp.Confidence <= 0 || resp.Confidence >= 0.9 {
		t.Errorf("expected reduced (but non-zero) confidence after 1 retry, got %v", resp.Confidence)
	}
	if resp.PromptStats == nil || resp.TokenUsage == nil {
		t.Fatalf("expected prompt_stats and token_usage on success, got stats=%v usage=%v", resp.PromptStats, resp.TokenUsage)
	}
	if resp.TokenUsage.Total <= 0 {
		t.Fatalf("expected positive token estimate, got %+v", resp.TokenUsage)
	}
}

func TestProcessQuestionRetriesOnSQLDryRunFailure(t *testing.T) {
	srv := stubLLMServer(t, []string{
		`{"select":[{"type":"metric","name":"row_count"}],"limit":10}`,
		`{"select":[{"type":"metric","name":"row_count"}],"limit":5}`,
	})
	defer srv.Close()

	cfg := config.AIConfig{Provider: "openai", BaseURL: srv.URL, APIKey: "x", Model: "test", MaxRetries: 2}
	svc := NewService(cfg, query.NewValidator(1000))

	model := &semantic.SemanticModel{
		ID:           "m",
		DatasourceID: "d",
		Name:         "public.orders",
		Metrics:      []semantic.Metric{{Name: "row_count", Aggregation: "count", Expression: "*"}},
	}

	calls := 0
	failingValidator := func(_ context.Context, _ *query.LogicalQuery) error {
		calls++
		if calls == 1 {
			return errPlannerSyntax
		}
		return nil
	}

	resp, err := svc.ProcessQuestion(context.Background(), "kaç sipariş var", model, WithSQLValidator(failingValidator))
	if err != nil {
		t.Fatalf("ProcessQuestion error = %v, want nil", err)
	}
	if resp.LogicalQuery == nil {
		t.Fatalf("expected query after dry-run retry; warnings=%v", resp.Warnings)
	}
	if calls != 2 {
		t.Errorf("expected SQLValidator to be called twice (fail then succeed), got %d", calls)
	}
	if resp.Confidence <= 0 || resp.Confidence >= 0.9 {
		t.Errorf("expected reduced confidence after a dry-run retry, got %v", resp.Confidence)
	}
}

func TestProcessQuestionGivesUpAfterMaxRetries(t *testing.T) {
	srv := stubLLMServer(t, []string{"junk1", "junk2", "junk3"})
	defer srv.Close()

	cfg := config.AIConfig{Provider: "openai", BaseURL: srv.URL, APIKey: "x", Model: "test", MaxRetries: 1}
	svc := NewService(cfg, query.NewValidator(1000))

	model := &semantic.SemanticModel{ID: "m", DatasourceID: "d", Name: "x"}
	resp, err := svc.ProcessQuestion(context.Background(), "q", model)
	if err != nil {
		t.Fatalf("ProcessQuestion error = %v, want nil", err)
	}
	if resp.LogicalQuery != nil {
		t.Errorf("expected nil LogicalQuery after exhausted retries, got %+v", resp.LogicalQuery)
	}
	if resp.Confidence != 0 {
		t.Errorf("expected confidence 0 on total failure, got %v", resp.Confidence)
	}
}

func TestProcessQuestionDoesNotReturnInvalidLogicalQueryAfterValidationRetries(t *testing.T) {
	srv := stubLLMServer(t, []string{
		`{"select":[{"type":"metric","name":"row_count"}],"group_by":[{"field":"year(orders.created_at)"}],"limit":10}`,
		`{"select":[{"type":"metric","name":"row_count"}],"group_by":[{"field":"year(orders.created_at)"}],"limit":10}`,
		"Hangi yıl alanını kullanmalıyım?",
	})
	defer srv.Close()

	cfg := config.AIConfig{Provider: "openai", BaseURL: srv.URL, APIKey: "x", Model: "test", MaxRetries: 1}
	svc := NewService(cfg, query.NewValidator(1000))

	model := &semantic.SemanticModel{
		ID:           "m",
		DatasourceID: "d",
		Name:         "public.orders",
		Dimensions:   []semantic.Dimension{{Name: "created_at_year", ColumnRef: "orders.created_at", Type: "date", TimeGrain: "year"}},
		Metrics:      []semantic.Metric{{Name: "row_count", Aggregation: "count", Expression: "*"}},
	}
	resp, err := svc.ProcessQuestion(context.Background(), "yıllara göre siparişler", model)
	if err != nil {
		t.Fatalf("ProcessQuestion error = %v, want nil", err)
	}
	if resp.LogicalQuery != nil {
		t.Fatalf("expected nil LogicalQuery after exhausted validation retries, got %+v", resp.LogicalQuery)
	}
	if resp.Confidence != 0 {
		t.Errorf("expected confidence 0 on invalid exhausted validation, got %v", resp.Confidence)
	}
	if !resp.NeedsClarification {
		t.Errorf("expected clarification after invalid exhausted validation")
	}
}

func TestProcessQuestionEmitsClarificationAfterExhaustedRetries(t *testing.T) {
	srv := stubLLMServer(t, []string{
		"junk1",
		"junk2",
		"Hangi metriği kastediyorsunuz: ciro mu yoksa sipariş adedi mi?",
	})
	defer srv.Close()

	cfg := config.AIConfig{Provider: "openai", BaseURL: srv.URL, APIKey: "x", Model: "test", MaxRetries: 1}
	svc := NewService(cfg, query.NewValidator(1000))

	model := &semantic.SemanticModel{
		ID:           "m",
		DatasourceID: "d",
		Name:         "public.orders",
		Metrics:      []semantic.Metric{{Name: "row_count", Aggregation: "count", Expression: "*"}},
	}

	resp, err := svc.ProcessQuestion(context.Background(), "ne kadar", model)
	if err != nil {
		t.Fatalf("ProcessQuestion error = %v, want nil", err)
	}
	if !resp.NeedsClarification {
		t.Errorf("expected NeedsClarification=true after exhausted retries, got false")
	}
	if resp.ClarificationQuestion == "" {
		t.Errorf("expected non-empty ClarificationQuestion")
	}
}

// TestProcessQuestionMultiCandidateMajority verifies that when self-consistency
// voting is enabled, the service returns the structurally-equivalent majority
// candidate without entering the retry loop.
func TestProcessQuestionMultiCandidateMajority(t *testing.T) {
	majority := `{"select":[{"type":"metric","name":"row_count"}],"limit":10}`
	dissent := `{"select":[{"type":"metric","name":"row_count"}],"limit":5}`
	srv := stubLLMServer(t, []string{majority, majority, dissent})
	defer srv.Close()

	cfg := config.AIConfig{
		Provider:            "openai",
		BaseURL:             srv.URL,
		APIKey:              "x",
		Model:               "test",
		MaxRetries:          2,
		MultiCandidateCount: 3,
	}
	svc := NewService(cfg, query.NewValidator(1000))

	model := &semantic.SemanticModel{
		ID:           "m",
		DatasourceID: "d",
		Name:         "public.orders",
		Metrics:      []semantic.Metric{{Name: "row_count", Aggregation: "count", Expression: "*"}},
	}

	resp, err := svc.ProcessQuestion(context.Background(), "kaç sipariş var", model)
	if err != nil {
		t.Fatalf("ProcessQuestion error = %v, want nil", err)
	}
	if resp.LogicalQuery == nil || resp.LogicalQuery.Limit != 10 {
		t.Fatalf("expected majority limit=10, got %+v", resp.LogicalQuery)
	}
	foundVote := false
	for _, w := range resp.Warnings {
		if w == "self-consistency: 2/3 candidates agreed" {
			foundVote = true
			break
		}
	}
	if !foundVote {
		t.Errorf("expected self-consistency warning, got warnings=%v", resp.Warnings)
	}
}

// TestProcessQuestionMultiCandidateNoMajorityFallsBack ensures a tied vote
// (no strict majority) falls through to the standard single-shot + retry path
// rather than returning a low-confidence guess.
func TestProcessQuestionMultiCandidateNoMajorityFallsBack(t *testing.T) {
	a := `{"select":[{"type":"metric","name":"row_count"}],"limit":10}`
	b := `{"select":[{"type":"metric","name":"row_count"}],"limit":20}`
	c := `{"select":[{"type":"metric","name":"row_count"}],"limit":30}`
	// 3 multi-candidate samples, all different → no winner.
	// Then the retry path opens with one more call which we make succeed.
	srv := stubLLMServer(t, []string{a, b, c, a})
	defer srv.Close()

	cfg := config.AIConfig{
		Provider:            "openai",
		BaseURL:             srv.URL,
		APIKey:              "x",
		Model:               "test",
		MaxRetries:          1,
		MultiCandidateCount: 3,
	}
	svc := NewService(cfg, query.NewValidator(1000))

	model := &semantic.SemanticModel{
		ID:           "m",
		DatasourceID: "d",
		Name:         "public.orders",
		Metrics:      []semantic.Metric{{Name: "row_count", Aggregation: "count", Expression: "*"}},
	}

	resp, err := svc.ProcessQuestion(context.Background(), "kaç sipariş var", model)
	if err != nil {
		t.Fatalf("ProcessQuestion error = %v, want nil", err)
	}
	if resp.LogicalQuery == nil {
		t.Fatalf("expected fallback path to produce a query, got nil")
	}
	for _, w := range resp.Warnings {
		if w == "self-consistency: 1/3 candidates agreed" || w == "self-consistency: 0/3 candidates agreed" {
			t.Errorf("did not expect self-consistency warning when no majority; got %q", w)
		}
	}
}

func TestComputeConfidence(t *testing.T) {
	cases := []struct {
		name        string
		validations int
		retries     int
		want        float64
	}{
		{"clean", 0, 0, 0.9},
		{"one validation error", 1, 0, 0.75},
		{"one retry", 0, 1, 0.8},
		{"clamped at zero", 10, 10, 0},
	}
	for _, c := range cases {
		got := computeConfidence(c.validations, c.retries)
		if got != c.want {
			t.Errorf("computeConfidence(%d, %d) = %v, want %v", c.validations, c.retries, got, c.want)
		}
	}
}
