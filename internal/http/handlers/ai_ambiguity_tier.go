package handlers

import (
	"context"

	"github.com/biqly/biqly/internal/ai"
	"github.com/biqly/biqly/internal/ai/routing"
	"github.com/biqly/biqly/internal/config"
	"github.com/biqly/biqly/internal/semantic"
)

// tierZeroClarificationIfNeeded handles table-routing ambiguity tier-0 short-circuit.
// Returns (response, true) when routing needs clarification; (nil, false) otherwise.
func (h *AIHandler) tierZeroClarificationIfNeeded(
	ctx context.Context,
	req aiQueryRequest,
	model *semantic.SemanticModel,
	routeResult *routing.TableRoutingResult,
) (*ai.Response, bool) {
	if routeResult == nil || !routeResult.NeedsClarification {
		return nil, false
	}
	if h.metrics != nil {
		h.metrics.RecordAmbiguityTier("0")
	}
	resp := clarificationResponse(routeResult)
	return h.observeAIRequest(ctx, req, model, routeResult, resp, 0, nil), true
}

func ambiguityProcessOptions(cfg config.AmbiguityConfig, pc *ProcessContext, observer ai.AmbiguityAnalysisObserver, tierRecorder func(tier string)) []ai.ProcessOption {
	// Skip is first-class: run the ambiguity check regardless of round and let
	// the policy proceed with the top interpretation of every ambiguous term.
	if pc != nil && pc.ClarificationSkip {
		opts := []ai.ProcessOption{
			ai.WithAmbiguityCheck(true),
			ai.WithClarificationSkip(true),
			ai.WithAmbiguityConfidenceThreshold(cfg.ConfidenceThreshold),
		}
		if observer != nil {
			opts = append(opts, ai.WithAmbiguityAnalysisObserver(observer))
		}
		return opts
	}
	if pc != nil && pc.ShouldUseInteractiveTier(cfg) {
		if tierRecorder != nil {
			tierRecorder("3")
		}
		opts := []ai.ProcessOption{
			ai.WithAmbiguityCheck(true),
			ai.WithAmbiguityInteractiveTier(true),
			ai.WithAmbiguityConfidenceThreshold(cfg.ConfidenceThreshold),
			ai.WithLLMAmbiguityCheck(true),
			ai.WithClarifyPolicy(cfg.ClarifyPolicyEnabled),
		}
		if observer != nil {
			opts = append(opts, ai.WithAmbiguityAnalysisObserver(observer))
		}
		if tierRecorder != nil {
			opts = append(opts, ai.WithAmbiguityTierObserver(tierRecorder))
		}
		return opts
	}
	if pc != nil && pc.AmbiguityCapReached(cfg) {
		return nil
	}
	if pc == nil || !pc.ShouldCheckAmbiguity(cfg) {
		return nil
	}

	opts := []ai.ProcessOption{
		ai.WithAmbiguityCheck(true),
		ai.WithAmbiguityConfidenceThreshold(cfg.ConfidenceThreshold),
		ai.WithAmbiguityMaxOptions(cfg.MaxOptions),
		ai.WithClarifyPolicy(cfg.ClarifyPolicyEnabled),
	}
	if cfg.TieredEnabled {
		opts = append(opts, ai.WithAmbiguitySynonymOnly(true))
	}
	opts = append(opts, ai.WithLLMAmbiguityCheck(pc.ShouldUseLLMAmbiguityTier(cfg)))
	if observer != nil {
		opts = append(opts, ai.WithAmbiguityAnalysisObserver(observer))
	}
	if tierRecorder != nil {
		opts = append(opts, ai.WithAmbiguityTierObserver(tierRecorder))
	}
	return opts
}
