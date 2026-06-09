package handlers

import (
	"testing"

	"github.com/biqly/biqly/internal/config"
)

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
	cfg := config.AmbiguityConfig{CheckEnabled: true, TieredEnabled: true}
	pc := &ProcessContext{clarificationRound: maxClarificationRounds}

	var tiers []string
	opts := ambiguityProcessOptions(cfg, pc, nil, func(tier string) { tiers = append(tiers, tier) })
	if len(opts) != 0 {
		t.Fatalf("opts = %d, want 0 when cap reached", len(opts))
	}
	if len(tiers) != 1 || tiers[0] != "3" {
		t.Fatalf("tiers = %v, want [3]", tiers)
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
