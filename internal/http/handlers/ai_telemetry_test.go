package handlers

import (
	"testing"

	"github.com/biqly/biqly/internal/ai"
	"github.com/biqly/biqly/internal/metadata"
	"github.com/biqly/biqly/internal/query"
)

func TestDeriveAIOutcome(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		resp *ai.Response
		want string
	}{
		{"nil", nil, metadata.AIOutcomeFailed},
		{"clarification", &ai.Response{
			Clarification: &ai.ClarificationResponse{
				NeedsClarification: true,
			},
		}, metadata.AIOutcomeClarification},
		{"failed no lq", &ai.Response{
			Result: &ai.AIResult{
				Confidence: 0.9,
			},
		}, metadata.AIOutcomeFailed},
		{"success", &ai.Response{
			Result: &ai.AIResult{
				LogicalQuery: &query.LogicalQuery{},
				Confidence:   0.8,
			},
		}, metadata.AIOutcomeSuccess},
		{"partial warnings", &ai.Response{
			Result: &ai.AIResult{
				LogicalQuery: &query.LogicalQuery{},
				Confidence:   0.9,
				Warnings:     []string{"x"},
			},
		}, metadata.AIOutcomePartial},
		{"partial low confidence", &ai.Response{
			Result: &ai.AIResult{
				LogicalQuery: &query.LogicalQuery{},
				Confidence:   0.5,
			},
		}, metadata.AIOutcomePartial},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := deriveAIOutcome(tc.resp); got != tc.want {
				t.Fatalf("deriveAIOutcome() = %q, want %q", got, tc.want)
			}
		})
	}
}
