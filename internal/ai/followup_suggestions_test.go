package ai

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestValidateSuggestedFollowUpsKeepsValidSuggestions(t *testing.T) {
	candidates := []SuggestedFollowUp{
		{
			ID:       "top-hours",
			Label:    "Compare the busiest 5 hours",
			Question: "Group yesterday's tweets by hour and compare the busiest 5 hour windows.",
			Reason:   "Current result found a single busiest hour.",
			Kind:     SuggestedFollowUpComparison,
			Requires: []string{"created_at_ts"},
		},
	}

	got := ValidateSuggestedFollowUps(candidates, []string{"created_at_ts", "author_id"}, nil)

	require.Equal(t, candidates, got)
}

func TestValidateSuggestedFollowUpsDropsUnknownKind(t *testing.T) {
	candidates := []SuggestedFollowUp{
		{
			ID:       "weird-kind",
			Label:    "Do something",
			Question: "Do something weird?",
			Kind:     SuggestedFollowUpKind("mystery"),
		},
	}

	got := ValidateSuggestedFollowUps(candidates, nil, nil)

	require.Empty(t, got)
}

func TestValidateSuggestedFollowUpsDropsUnknownRequiredField(t *testing.T) {
	candidates := []SuggestedFollowUp{
		{
			ID:       "missing-field",
			Label:    "Break down by region",
			Question: "Break the result down by region?",
			Kind:     SuggestedFollowUpBreakdown,
			Requires: []string{"region"},
		},
	}

	got := ValidateSuggestedFollowUps(candidates, []string{"created_at_ts"}, nil)

	require.Empty(t, got)
}

func TestValidateSuggestedFollowUpsDropsPriorQuestionDuplicate(t *testing.T) {
	candidates := []SuggestedFollowUp{
		{
			ID:       "dup",
			Label:    "Busiest hour",
			Question: "  What was the BUSIEST hour?  ",
			Kind:     SuggestedFollowUpExplain,
		},
	}
	priorQuestions := []string{"what was the busiest hour?"}

	got := ValidateSuggestedFollowUps(candidates, nil, priorQuestions)

	require.Empty(t, got)
}

func TestValidateSuggestedFollowUpsDropsEmptyLabelOrQuestion(t *testing.T) {
	candidates := []SuggestedFollowUp{
		{
			ID:       "blank-label",
			Label:    "   ",
			Question: "What happened yesterday?",
			Kind:     SuggestedFollowUpExplain,
		},
		{
			ID:       "blank-question",
			Label:    "Yesterday recap",
			Question: "   ",
			Kind:     SuggestedFollowUpExplain,
		},
	}

	got := ValidateSuggestedFollowUps(candidates, nil, nil)

	require.Empty(t, got)
}

func TestValidateSuggestedFollowUpsDropsLabelDuplicate(t *testing.T) {
	candidates := []SuggestedFollowUp{
		{
			ID:       "first",
			Label:    "Busiest hour breakdown",
			Question: "What was the busiest hour yesterday?",
			Kind:     SuggestedFollowUpExplain,
		},
		{
			ID:       "second",
			Label:    "  BUSIEST hour BREAKDOWN  ",
			Question: "Which day had the most activity overall?",
			Kind:     SuggestedFollowUpExplain,
		},
	}

	got := ValidateSuggestedFollowUps(candidates, nil, nil)

	require.Len(t, got, 1)
	require.Equal(t, "first", got[0].ID)
}

func TestValidateSuggestedFollowUpsDropsSubstringSimilarQuestion(t *testing.T) {
	candidates := []SuggestedFollowUp{
		{
			ID:       "first",
			Label:    "Busiest hour",
			Question: "What was the busiest hour yesterday for tweets?",
			Kind:     SuggestedFollowUpExplain,
		},
		{
			ID:       "second",
			Label:    "Busiest hour again",
			Question: "Yesterday, what was the busiest hour yesterday for tweets, exactly?",
			Kind:     SuggestedFollowUpExplain,
		},
	}

	got := ValidateSuggestedFollowUps(candidates, nil, nil)

	require.Len(t, got, 1)
	require.Equal(t, "first", got[0].ID)
}

func TestValidateSuggestedFollowUpsKeepsShortSubstringOverlap(t *testing.T) {
	candidates := []SuggestedFollowUp{
		{
			ID:       "first",
			Label:    "Trend",
			Question: "What was the busiest hour yesterday for tweets?",
			Kind:     SuggestedFollowUpTrend,
		},
		{
			ID:       "second",
			Label:    "Trend line",
			Question: "Show a trend of tweet volume by day over the last month?",
			Kind:     SuggestedFollowUpTrend,
		},
	}

	got := ValidateSuggestedFollowUps(candidates, nil, nil)

	require.Len(t, got, 2)
}

func TestValidateSuggestedFollowUpsCapsAtThree(t *testing.T) {
	candidates := []SuggestedFollowUp{
		{ID: "c1", Label: "One", Question: "Question one?", Kind: SuggestedFollowUpTrend},
		{ID: "c2", Label: "Two", Question: "Question two?", Kind: SuggestedFollowUpTrend},
		{ID: "c3", Label: "Three", Question: "Question three?", Kind: SuggestedFollowUpTrend},
		{ID: "c4", Label: "Four", Question: "Question four?", Kind: SuggestedFollowUpTrend},
	}

	got := ValidateSuggestedFollowUps(candidates, nil, nil)

	require.Len(t, got, maxSuggestedFollowUps)
	require.Equal(t, []string{"c1", "c2", "c3"}, []string{got[0].ID, got[1].ID, got[2].ID})
}

func TestRewriteFollowUpsWithAINilProviderReturnsNilNoError(t *testing.T) {
	got, err := RewriteFollowUpsWithAI(context.Background(), nil, "how many tweets?", []SuggestedFollowUp{
		{ID: "c1", Label: "One", Question: "Question one?", Kind: SuggestedFollowUpTrend},
	}, []string{"created_at_ts"}, nil)

	require.NoError(t, err)
	require.Nil(t, got)
}

func TestRewriteFollowUpsWithAINoCandidatesReturnsNilNoError(t *testing.T) {
	provider := &stubAnswerProvider{content: "should not be called"}

	got, err := RewriteFollowUpsWithAI(context.Background(), provider, "how many tweets?", nil, []string{"created_at_ts"}, nil)

	require.NoError(t, err)
	require.Nil(t, got)
	require.Equal(t, 0, provider.calls)
}

func TestRewriteFollowUpsWithAIParsesValidResponse(t *testing.T) {
	provider := &stubAnswerProvider{content: `{"suggestions": [
		{"id": "rewritten-1", "label": "Rewritten label", "question": "Rewritten question?", "reason": "why", "kind": "comparison", "requires": ["created_at_ts"]}
	]}`}
	candidates := []SuggestedFollowUp{
		{ID: "c1", Label: "One", Question: "Question one?", Kind: SuggestedFollowUpTrend},
	}

	got, err := RewriteFollowUpsWithAI(context.Background(), provider, "how many tweets?", candidates, []string{"created_at_ts"}, []string{"prior question?"})

	require.NoError(t, err)
	require.Len(t, got, 1)
	require.Equal(t, "rewritten-1", got[0].ID)
	require.Equal(t, "Rewritten label", got[0].Label)
	require.Equal(t, "Rewritten question?", got[0].Question)
	require.Equal(t, SuggestedFollowUpComparison, got[0].Kind)
	require.Equal(t, 1, provider.calls)
	require.Contains(t, provider.lastPrompt, "how many tweets?")
	require.Contains(t, provider.lastPrompt, "created_at_ts")
	require.Contains(t, provider.lastPrompt, "prior question?")
}

func TestRewriteFollowUpsWithAIInvalidJSONReturnsError(t *testing.T) {
	provider := &stubAnswerProvider{content: "not json at all"}
	candidates := []SuggestedFollowUp{
		{ID: "c1", Label: "One", Question: "Question one?", Kind: SuggestedFollowUpTrend},
	}

	got, err := RewriteFollowUpsWithAI(context.Background(), provider, "how many tweets?", candidates, nil, nil)

	require.Error(t, err)
	require.Nil(t, got)
}

func TestRewriteFollowUpsWithAIProviderErrorReturnsError(t *testing.T) {
	provider := &stubAnswerProvider{err: errors.New("boom")}
	candidates := []SuggestedFollowUp{
		{ID: "c1", Label: "One", Question: "Question one?", Kind: SuggestedFollowUpTrend},
	}

	got, err := RewriteFollowUpsWithAI(context.Background(), provider, "how many tweets?", candidates, nil, nil)

	require.Error(t, err)
	require.Nil(t, got)
}

// TestRewriteFollowUpsWithAIDoesNotValidate confirms RewriteFollowUpsWithAI
// does not itself call ValidateSuggestedFollowUps: a suggestion whose
// Requires references a field outside availableFields must still come back
// in the raw, unvalidated result. Validation is the caller's responsibility
// (see attachSuggestedFollowUps), so that a single AI failure path can fall
// back to the deterministic suggestions instead of surfacing a partially
// invalid AI result.
func TestRewriteFollowUpsWithAIDoesNotValidate(t *testing.T) {
	provider := &stubAnswerProvider{content: `{"suggestions": [
		{"id": "bad", "label": "Bad", "question": "Bad question?", "kind": "comparison", "requires": ["field_not_available"]}
	]}`}
	candidates := []SuggestedFollowUp{
		{ID: "c1", Label: "One", Question: "Question one?", Kind: SuggestedFollowUpTrend},
	}

	got, err := RewriteFollowUpsWithAI(context.Background(), provider, "how many tweets?", candidates, []string{"created_at_ts"}, nil)

	require.NoError(t, err)
	require.Len(t, got, 1)
	require.Equal(t, "bad", got[0].ID)
	require.Equal(t, []string{"field_not_available"}, got[0].Requires)
}
