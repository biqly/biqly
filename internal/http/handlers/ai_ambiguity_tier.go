package handlers

import (
	"github.com/biqly/biqly/internal/ai"
	"github.com/biqly/biqly/internal/config"
)

func ambiguityProcessOptions(cfg config.AmbiguityConfig, pc *ProcessContext, observer ai.AmbiguityAnalysisObserver, tierRecorder func(tier string)) []ai.ProcessOption {
	if pc != nil && pc.AmbiguityCapReached(cfg) {
		if tierRecorder != nil {
			tierRecorder("3")
		}
		return nil
	}
	if pc == nil || !pc.ShouldCheckAmbiguity(cfg) {
		return nil
	}

	opts := []ai.ProcessOption{
		ai.WithAmbiguityCheck(true),
		ai.WithAmbiguityConfidenceThreshold(cfg.ConfidenceThreshold),
		ai.WithAmbiguityMaxOptions(cfg.MaxOptions),
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
