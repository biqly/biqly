package handlers

import (
	"context"
	"strings"

	"github.com/biqly/biqly/internal/ai"
	"github.com/biqly/biqly/internal/ai/ambiguity"
	"github.com/biqly/biqly/internal/ai/prompt"
	"github.com/biqly/biqly/internal/config"
	"github.com/biqly/biqly/internal/semantic"
)

const maxClarificationRounds = 2

// ProcessContext holds per-turn AI query processing state shared by sync HTTP and
// async job paths. ClarificationResolved is set only by Resolve.
type ProcessContext struct {
	Question              string
	ClarificationChoice   string
	ClarificationResolved bool
	DatasourceID          string
	clarificationRound    int
}

func buildProcessContext(req aiQueryRequest) *ProcessContext {
	return &ProcessContext{
		Question:            req.Question,
		ClarificationChoice: req.ClarificationChoice,
		DatasourceID:        req.DatasourceID,
		clarificationRound:  req.ClarificationRound,
	}
}

func (pc *ProcessContext) ApplyToRequest(req *aiQueryRequest) {
	if pc == nil || req == nil {
		return
	}
	req.Question = pc.Question
	req.ClarificationChoice = pc.ClarificationChoice
}

// Resolve applies the user's clarification choice to the question when present.
func (pc *ProcessContext) Resolve(ctx context.Context, model *semantic.SemanticModel, glossary []prompt.GlossaryEntry) error {
	if pc == nil || pc.ClarificationChoice == "" {
		return nil
	}
	question, err := ambiguity.Resolve(ctx, pc.Question, pc.ClarificationChoice, model, glossary)
	if err != nil {
		return err
	}
	pc.Question = question
	pc.ClarificationChoice = ""
	if ambiguity.HasRemaining(ctx, question, model, glossary) {
		pc.ClarificationResolved = false
		return nil
	}
	pc.ClarificationResolved = true
	return nil
}

func (pc *ProcessContext) ShouldCheckAmbiguity(cfg config.AmbiguityConfig) bool {
	if pc == nil {
		return cfg.CheckEnabled
	}
	if !cfg.CheckEnabled || pc.ClarificationResolved {
		return false
	}
	if pc.ShouldUseInteractiveTier(cfg) {
		return true
	}
	if pc.AmbiguityCapReached(cfg) {
		return false
	}
	return pc.clarificationRound < maxClarificationRounds
}

// ShouldUseInteractiveTier is Tier 3: one agent-driven LLM disambiguation pass after
// the user has exhausted normal clarification rounds without resolving.
func (pc *ProcessContext) ShouldUseInteractiveTier(cfg config.AmbiguityConfig) bool {
	if pc == nil {
		return false
	}
	return cfg.CheckEnabled && !pc.ClarificationResolved && pc.clarificationRound == maxClarificationRounds
}

func (pc *ProcessContext) AmbiguityCapReached(cfg config.AmbiguityConfig) bool {
	if pc == nil {
		return false
	}
	return cfg.CheckEnabled && !pc.ClarificationResolved && pc.clarificationRound > maxClarificationRounds
}

func (pc *ProcessContext) ShouldUseLLMAmbiguityTier(cfg config.AmbiguityConfig) bool {
	if !cfg.LLMEnabled {
		return false
	}
	if !cfg.TieredEnabled {
		return true
	}
	if cfg.MaxLLMTierPerQuestion <= 0 {
		return false
	}
	if pc == nil {
		return true
	}
	return pc.clarificationRound < cfg.MaxLLMTierPerQuestion
}

func (pc *ProcessContext) nextAmbiguityClarificationRound() int {
	if pc == nil {
		return 1
	}
	return pc.clarificationRound + 1
}

func isAmbiguityAnalyzerClarification(resp *ai.Response) bool {
	if resp == nil || resp.Clarification == nil || !resp.Clarification.NeedsClarification {
		return false
	}
	clar := resp.Clarification.Clarification
	return clar != nil && clar.Source == "ambiguity_analyzer"
}

func attachAmbiguityClarificationRound(pc *ProcessContext, resp *ai.Response) {
	if pc == nil || !isAmbiguityAnalyzerClarification(resp) {
		return
	}
	resp.Clarification.ClarificationRound = pc.nextAmbiguityClarificationRound()
}

func (h *AIHandler) resolveProcessContext(ctx context.Context, pc *ProcessContext, model *semantic.SemanticModel) error {
	if pc == nil || pc.ClarificationChoice == "" {
		return nil
	}
	choice := pc.ClarificationChoice
	glossary := h.loadGlossaryForAmbiguity(ctx, model)
	if err := pc.Resolve(ctx, model, glossary); err != nil {
		return err
	}
	if h.metrics != nil && strings.HasPrefix(choice, "ambiguity:") {
		h.metrics.RecordAmbiguityClarified()
	}
	return nil
}
