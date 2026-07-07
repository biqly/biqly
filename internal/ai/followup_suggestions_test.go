package ai

import (
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
