package handlers

import (
	"encoding/json"
	"testing"
)

func TestPriorTurnsForPromptMapsResultSummary(t *testing.T) {
	turns := priorTurnsForPrompt([]priorTurnPayload{
		{
			Question:      "geçtiğimiz ay en çok hangi gün tweet atılmıştır?",
			LogicalQuery:  json.RawMessage(`{"select":[{"name":"tweet_count"}]}`),
			Note:          "executed",
			ResultSummary: "May 20, 2026: 2,932 tweets",
		},
	})

	if len(turns) != 1 {
		t.Fatalf("expected one turn, got %d", len(turns))
	}
	if turns[0].ResultSummary != "May 20, 2026: 2,932 tweets" {
		t.Fatalf("expected result summary to map through, got %q", turns[0].ResultSummary)
	}
}
