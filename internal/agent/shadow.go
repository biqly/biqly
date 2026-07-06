package agent

import (
	"context"
	"fmt"
	"time"

	"github.com/bytedance/sonic"
)

// ShadowCategory is a bounded, enum-like classification of how a shadow
// (agent) run compared to the legacy run it shadowed. Its fixed value set
// makes it safe to use as a metric label — unlike SQL or question text,
// which must never be a label (unbounded cardinality) and are kept out of
// ShadowComparison.Categories entirely.
type ShadowCategory string

const (
	ShadowCategoryMatch                 ShadowCategory = "match"
	ShadowCategoryResultMismatch        ShadowCategory = "result_mismatch"
	ShadowCategoryQueryMismatch         ShadowCategory = "query_mismatch"
	ShadowCategoryLatencyRegression     ShadowCategory = "latency_regression"
	ShadowCategoryClarificationMismatch ShadowCategory = "clarification_mismatch"
	ShadowCategoryPolicyOutcomeMismatch ShadowCategory = "policy_outcome_mismatch"
	ShadowCategoryAgentOnlyFailure      ShadowCategory = "agent_only_failure"
	ShadowCategoryLegacyOnlyFailure     ShadowCategory = "legacy_only_failure"
	ShadowCategoryBothFailed            ShadowCategory = "both_failed"
)

// latencyRegressionFactor and latencyRegressionFloor bound what counts as a
// latency regression: the agent must be both meaningfully slower in
// relative terms AND slow in absolute terms, so noise on already-fast
// (sub-2s) runs never gets flagged.
const (
	latencyRegressionFactor = 2
	latencyRegressionFloor  = 2 * time.Second
)

// ShadowOutcome is one side's (legacy or agent) normalized outcome for a
// shadowed run. NormalizedQuery/NormalizedSQL/ResultFingerprint are compared
// for equality only — CompareShadowOutcomes never places raw query/SQL/result
// text into ShadowComparison.Categories; callers decide their own retention
// policy for whatever they put in Detail.
type ShadowOutcome struct {
	Succeeded          bool
	NormalizedQuery    string
	NormalizedSQL      string
	ResultFingerprint  string
	RowCount           int
	Latency            time.Duration
	ClarificationAsked bool
	PolicyDenied       bool
	PolicyReasonCode   string
}

// ShadowComparison is the result of comparing a legacy and an agent outcome
// for the same job. Detail is caller-persisted data (e.g. a JSONB column),
// not a metric — it is fine for it to carry more context than Categories.
type ShadowComparison struct {
	Categories []ShadowCategory
	Detail     map[string]any
}

// CompareShadowOutcomes classifies how agent compares to legacy for the same
// job. When both sides fail, only ShadowCategoryBothFailed is reported —
// further comparison is meaningless. Otherwise every applicable mismatch
// category is reported (a comparison can carry more than one).
func CompareShadowOutcomes(legacy, agent ShadowOutcome) ShadowComparison {
	detail := map[string]any{}

	if !legacy.Succeeded && !agent.Succeeded {
		return ShadowComparison{Categories: []ShadowCategory{ShadowCategoryBothFailed}, Detail: detail}
	}

	var categories []ShadowCategory
	switch {
	case !legacy.Succeeded:
		categories = append(categories, ShadowCategoryLegacyOnlyFailure)
	case !agent.Succeeded:
		categories = append(categories, ShadowCategoryAgentOnlyFailure)
	default:
		categories = compareSucceededOutcomes(legacy, agent, detail)
	}

	if len(categories) == 0 {
		categories = []ShadowCategory{ShadowCategoryMatch}
	}
	return ShadowComparison{Categories: categories, Detail: detail}
}

func compareSucceededOutcomes(legacy, agent ShadowOutcome, detail map[string]any) []ShadowCategory {
	var categories []ShadowCategory
	if legacy.ClarificationAsked != agent.ClarificationAsked {
		categories = append(categories, ShadowCategoryClarificationMismatch)
	}
	if legacy.PolicyDenied != agent.PolicyDenied {
		categories = append(categories, ShadowCategoryPolicyOutcomeMismatch)
		if agent.PolicyReasonCode != "" {
			detail["agent_policy_reason_code"] = agent.PolicyReasonCode
		}
	}
	if legacy.NormalizedQuery != agent.NormalizedQuery || legacy.NormalizedSQL != agent.NormalizedSQL {
		categories = append(categories, ShadowCategoryQueryMismatch)
	}
	if legacy.ResultFingerprint != agent.ResultFingerprint || legacy.RowCount != agent.RowCount {
		categories = append(categories, ShadowCategoryResultMismatch)
	}
	if agent.Latency > legacy.Latency*latencyRegressionFactor && agent.Latency > latencyRegressionFloor {
		categories = append(categories, ShadowCategoryLatencyRegression)
	}
	return categories
}

// ShadowComparisonStore persists shadow comparison outcomes, one row per
// category (agent_shadow_comparisons, migration 065a). Decoupled from
// internal/metadata so this package stays free of a direct DB dependency.
type ShadowComparisonStore interface {
	RecordShadowComparison(ctx context.Context, jobID, legacyRunID, agentRunID string, category ShadowCategory, detail []byte) error
}

// ShadowEvaluator computes and persists shadow comparisons between a
// legacy run and the agent run that shadowed it.
type ShadowEvaluator struct {
	store ShadowComparisonStore
}

// NewShadowEvaluator builds a ShadowEvaluator backed by store.
func NewShadowEvaluator(store ShadowComparisonStore) *ShadowEvaluator {
	return &ShadowEvaluator{store: store}
}

// Evaluate compares legacy and agent, then persists one row per resulting
// category. Returns the comparison even if persistence fails, so a caller
// can still log/alert on it.
func (e *ShadowEvaluator) Evaluate(
	ctx context.Context, jobID, legacyRunID, agentRunID string, legacy, agent ShadowOutcome,
) (ShadowComparison, error) {
	comparison := CompareShadowOutcomes(legacy, agent)
	detailJSON, err := sonic.Marshal(comparison.Detail)
	if err != nil {
		return comparison, fmt.Errorf("encode shadow comparison detail: %w", err)
	}
	for _, category := range comparison.Categories {
		if err := e.store.RecordShadowComparison(ctx, jobID, legacyRunID, agentRunID, category, detailJSON); err != nil {
			return comparison, fmt.Errorf("record shadow comparison: %w", err)
		}
	}
	return comparison, nil
}
