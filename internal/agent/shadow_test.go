package agent

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func succeededOutcome() ShadowOutcome {
	return ShadowOutcome{
		Succeeded:         true,
		NormalizedQuery:   `{"metrics":["revenue"]}`,
		NormalizedSQL:     "SELECT sum(revenue) FROM orders",
		ResultFingerprint: "fp-1",
		RowCount:          1,
		Latency:           500 * time.Millisecond,
	}
}

func TestCompareShadowOutcomesMatch(t *testing.T) {
	legacy := succeededOutcome()
	agent := succeededOutcome()
	got := CompareShadowOutcomes(legacy, agent)
	assert.Equal(t, []ShadowCategory{ShadowCategoryMatch}, got.Categories)
}

func TestCompareShadowOutcomesBothFailed(t *testing.T) {
	got := CompareShadowOutcomes(ShadowOutcome{Succeeded: false}, ShadowOutcome{Succeeded: false})
	assert.Equal(t, []ShadowCategory{ShadowCategoryBothFailed}, got.Categories)
}

func TestCompareShadowOutcomesLegacyOnlyFailure(t *testing.T) {
	got := CompareShadowOutcomes(ShadowOutcome{Succeeded: false}, succeededOutcome())
	assert.Equal(t, []ShadowCategory{ShadowCategoryLegacyOnlyFailure}, got.Categories)
}

func TestCompareShadowOutcomesAgentOnlyFailure(t *testing.T) {
	got := CompareShadowOutcomes(succeededOutcome(), ShadowOutcome{Succeeded: false})
	assert.Equal(t, []ShadowCategory{ShadowCategoryAgentOnlyFailure}, got.Categories)
}

func TestCompareShadowOutcomesQueryMismatch(t *testing.T) {
	legacy := succeededOutcome()
	agent := succeededOutcome()
	agent.NormalizedSQL = "SELECT sum(revenue) FROM orders WHERE region = 'EU'"
	got := CompareShadowOutcomes(legacy, agent)
	assert.Contains(t, got.Categories, ShadowCategoryQueryMismatch)
}

func TestCompareShadowOutcomesResultMismatch(t *testing.T) {
	legacy := succeededOutcome()
	agent := succeededOutcome()
	agent.ResultFingerprint = "fp-different"
	got := CompareShadowOutcomes(legacy, agent)
	assert.Contains(t, got.Categories, ShadowCategoryResultMismatch)
}

func TestCompareShadowOutcomesClarificationMismatch(t *testing.T) {
	legacy := succeededOutcome()
	agent := succeededOutcome()
	agent.ClarificationAsked = true
	got := CompareShadowOutcomes(legacy, agent)
	assert.Contains(t, got.Categories, ShadowCategoryClarificationMismatch)
}

func TestCompareShadowOutcomesPolicyOutcomeMismatch(t *testing.T) {
	legacy := succeededOutcome()
	agent := succeededOutcome()
	agent.PolicyDenied = true
	agent.PolicyReasonCode = "hidden_column_denied"
	got := CompareShadowOutcomes(legacy, agent)
	assert.Contains(t, got.Categories, ShadowCategoryPolicyOutcomeMismatch)
	assert.Equal(t, "hidden_column_denied", got.Detail["agent_policy_reason_code"])
}

func TestCompareShadowOutcomesLatencyRegression(t *testing.T) {
	legacy := succeededOutcome()
	legacy.Latency = 500 * time.Millisecond
	agent := succeededOutcome()
	agent.Latency = 5 * time.Second // > 2x legacy AND > the 2s floor
	got := CompareShadowOutcomes(legacy, agent)
	assert.Contains(t, got.Categories, ShadowCategoryLatencyRegression)
}

func TestCompareShadowOutcomesNoRegressionBelowAbsoluteFloor(t *testing.T) {
	legacy := succeededOutcome()
	legacy.Latency = 10 * time.Millisecond
	agent := succeededOutcome()
	agent.Latency = 100 * time.Millisecond // 10x relative, but well under the 2s floor
	got := CompareShadowOutcomes(legacy, agent)
	assert.NotContains(t, got.Categories, ShadowCategoryLatencyRegression)
}

func TestCompareShadowOutcomesNeverPlacesSQLOrQueryTextInCategories(t *testing.T) {
	legacy := succeededOutcome()
	agent := succeededOutcome()
	agent.NormalizedSQL = "SELECT secret_column FROM users"
	got := CompareShadowOutcomes(legacy, agent)
	for _, c := range got.Categories {
		assert.NotContains(t, string(c), "SELECT", "categories must stay a bounded enum, never raw SQL")
		assert.NotContains(t, string(c), "secret_column")
	}
}

func TestCompareShadowOutcomesReportsMultipleSimultaneousCategories(t *testing.T) {
	legacy := succeededOutcome()
	agent := succeededOutcome()
	agent.NormalizedSQL = "SELECT sum(revenue) FROM orders WHERE region = 'EU'"
	agent.ResultFingerprint = "fp-different"
	got := CompareShadowOutcomes(legacy, agent)
	assert.Contains(t, got.Categories, ShadowCategoryQueryMismatch)
	assert.Contains(t, got.Categories, ShadowCategoryResultMismatch)
	assert.Len(t, got.Categories, 2)
}

type fakeShadowComparisonStore struct {
	calls      int
	categories []ShadowCategory
	failWith   error
}

func (f *fakeShadowComparisonStore) RecordShadowComparison(_ context.Context, _, _, _ string, category ShadowCategory, _ []byte) error {
	f.calls++
	f.categories = append(f.categories, category)
	return f.failWith
}

func TestShadowEvaluatorPersistsOneRowPerCategory(t *testing.T) {
	store := &fakeShadowComparisonStore{}
	eval := NewShadowEvaluator(store)

	legacy := succeededOutcome()
	agent := succeededOutcome()
	agent.NormalizedSQL = "different sql"
	agent.ResultFingerprint = "different"

	comparison, err := eval.Evaluate(context.Background(), "job-1", "legacy-run-1", "agent-run-1", legacy, agent)
	require.NoError(t, err)
	assert.Len(t, comparison.Categories, 2)
	assert.Equal(t, 2, store.calls)
	assert.ElementsMatch(t, comparison.Categories, store.categories)
}

func TestShadowEvaluatorPersistsSingleMatchRow(t *testing.T) {
	store := &fakeShadowComparisonStore{}
	eval := NewShadowEvaluator(store)

	_, err := eval.Evaluate(context.Background(), "job-1", "legacy-run-1", "agent-run-1", succeededOutcome(), succeededOutcome())
	require.NoError(t, err)
	assert.Equal(t, 1, store.calls)
	assert.Equal(t, []ShadowCategory{ShadowCategoryMatch}, store.categories)
}

func TestShadowEvaluatorPropagatesStoreError(t *testing.T) {
	store := &fakeShadowComparisonStore{failWith: errors.New("db down")}
	eval := NewShadowEvaluator(store)

	_, err := eval.Evaluate(context.Background(), "job-1", "legacy-run-1", "agent-run-1", succeededOutcome(), succeededOutcome())
	assert.Error(t, err)
}
