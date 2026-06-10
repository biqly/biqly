package handlers

import (
	"context"
	"database/sql/driver"
	"testing"
	"time"

	"github.com/biqly/biqly/internal/ai/routing"
	"github.com/biqly/biqly/internal/app"
	"github.com/biqly/biqly/internal/config"
	"github.com/biqly/biqly/internal/metadata"
)

type tierMetricsStub struct {
	tiers []string
}

func (*tierMetricsStub) RecordAIRequest(int64, bool, int, bool)  {}
func (*tierMetricsStub) RecordAIStep(string, int64)              {}
func (*tierMetricsStub) RecordLLMRequest(int64, int, int, int64) {}
func (*tierMetricsStub) RecordAmbiguityAnalysis(int64, string, bool) {
}
func (m *tierMetricsStub) RecordAmbiguityTier(tier string)  { m.tiers = append(m.tiers, tier) }
func (*tierMetricsStub) RecordAmbiguityClarified()          {}
func (*tierMetricsStub) RecordAmbiguityRoundCapReached()    {}
func (*tierMetricsStub) RecordAIRepair(bool, int, []string) {}
func (*tierMetricsStub) RecordMemoryStoreConfirmed()        {}
func (*tierMetricsStub) RecordMemoryStoreRecall(int)        {}
func (*tierMetricsStub) RecordMemoryRecallFeedback(bool, string) {
}
func (*tierMetricsStub) RecordEnrichContextGaps(int)               {}
func (*tierMetricsStub) RecordEnrichContextApplied(int)            {}
func (*tierMetricsStub) RecordEnrichContextApplyErrors(int)        {}
func (*tierMetricsStub) RecordMemoryStoreConfirmedEmbeddingError() {}
func (*tierMetricsStub) RecordAmbiguityClarificationRound(int)     {}
func (*tierMetricsStub) RecordAmbiguityResolution(string)          {}
func (*tierMetricsStub) RecordFeedbackSubmitted(string)            {}

func TestTierZeroClarificationIfNeeded(t *testing.T) {
	db, state := setupMockDB(t)
	repo := metadata.NewRepository(db)
	metrics := &tierMetricsStub{}
	h := &AIHandler{
		deps:    (&app.Dependencies{MetaRepo: repo}).AIDeps(),
		metrics: metrics,
	}
	ctx := context.Background()
	req := aiQueryRequest{DatasourceID: "ds-1", Question: "sales?"}
	route := &routing.TableRoutingResult{NeedsClarification: true, Candidates: []routing.TableCandidate{{Table: "orders"}}}

	state.queries = []queryMock{
		{
			Pattern: "INSERT INTO ai_query_history",
			Cols:    []string{"id", "created_at"},
			Rows:    [][]driver.Value{{"aqh-1", time.Now()}},
		},
	}

	resp, ok := h.tierZeroClarificationIfNeeded(ctx, req, nil, route)
	if !ok || resp == nil {
		t.Fatal("expected tier-0 clarification response")
	}
	if resp.Clarification == nil || !resp.Clarification.NeedsClarification {
		t.Fatal("expected NeedsClarification on response")
	}
	if len(metrics.tiers) != 1 || metrics.tiers[0] != "0" {
		t.Fatalf("tiers = %v, want [0]", metrics.tiers)
	}

	if resp, ok := h.tierZeroClarificationIfNeeded(ctx, req, nil, nil); ok || resp != nil {
		t.Fatalf("nil route: resp=%v ok=%v, want nil/false", resp, ok)
	}
	if resp, ok := h.tierZeroClarificationIfNeeded(ctx, req, nil, &routing.TableRoutingResult{}); ok || resp != nil {
		t.Fatalf("no clarification route: resp=%v ok=%v, want nil/false", resp, ok)
	}
}

func TestShouldUseLLMAmbiguityTier(t *testing.T) {
	tiered := config.AmbiguityConfig{
		CheckEnabled:          true,
		LLMEnabled:            true,
		TieredEnabled:         true,
		MaxLLMTierPerQuestion: 1,
	}
	legacy := config.AmbiguityConfig{
		CheckEnabled:  true,
		LLMEnabled:    true,
		TieredEnabled: false,
	}
	disabledLLM := tiered
	disabledLLM.LLMEnabled = false

	tests := []struct {
		name string
		cfg  config.AmbiguityConfig
		pc   *ProcessContext
		want bool
	}{
		{name: "tiered first round", cfg: tiered, pc: &ProcessContext{clarificationRound: 0}, want: true},
		{name: "tiered second round", cfg: tiered, pc: &ProcessContext{clarificationRound: 1}, want: false},
		{name: "legacy always when enabled", cfg: legacy, pc: &ProcessContext{clarificationRound: 3}, want: true},
		{name: "llm disabled", cfg: disabledLLM, pc: &ProcessContext{clarificationRound: 0}, want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.pc.ShouldUseLLMAmbiguityTier(tt.cfg); got != tt.want {
				t.Errorf("ShouldUseLLMAmbiguityTier() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestAmbiguityProcessOptionsCapRecordsTier3(t *testing.T) {
	cfg := config.AmbiguityConfig{CheckEnabled: true, TieredEnabled: true, LLMEnabled: true}
	pc := &ProcessContext{clarificationRound: maxClarificationRounds}

	var tiers []string
	opts := ambiguityProcessOptions(cfg, pc, nil, func(tier string) { tiers = append(tiers, tier) })
	if len(opts) == 0 {
		t.Fatal("opts empty, want interactive tier options at cap boundary")
	}
	if len(tiers) != 1 || tiers[0] != "3" {
		t.Fatalf("tiers = %v, want [3]", tiers)
	}
}

func TestAmbiguityProcessOptionsPastCapBypassesCheck(t *testing.T) {
	cfg := config.AmbiguityConfig{CheckEnabled: true, TieredEnabled: true}
	pc := &ProcessContext{clarificationRound: maxClarificationRounds + 1}

	opts := ambiguityProcessOptions(cfg, pc, nil, func(tier string) { t.Fatalf("unexpected tier %q", tier) })
	if len(opts) != 0 {
		t.Fatalf("opts = %d, want 0 when past interactive tier", len(opts))
	}
}

func TestAmbiguityProcessOptionsTieredReturnsOptions(t *testing.T) {
	cfg := config.AmbiguityConfig{
		CheckEnabled:          true,
		LLMEnabled:            true,
		TieredEnabled:         true,
		MaxLLMTierPerQuestion: 1,
	}
	pc := &ProcessContext{clarificationRound: 0}
	if got := len(ambiguityProcessOptions(cfg, pc, nil, nil)); got < 5 {
		t.Fatalf("opts len = %d, want at least 5 for tiered ambiguity", got)
	}
	if pcRound1 := (&ProcessContext{clarificationRound: 1}); pcRound1.ShouldUseLLMAmbiguityTier(cfg) {
		t.Fatal("round 1 should not use LLM tier when max is 1")
	}
}
