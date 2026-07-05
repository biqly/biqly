package handlers

import (
	"net/http"
	"testing"

	"github.com/biqly/biqly/internal/ai"
)

func TestSuggestSavedQueryName(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"", ""},
		{"   ", ""},
		{"revenue by country", "Revenue by country"},
		{"what were monthly sales last year?", "What were monthly sales last year"},
		{
			"show me the total number of active users grouped by region and plan tier over the past quarter please",
			"Show me the total number of active users grouped by",
		},
		{"top 10 products!!!", "Top 10 products"},
	}
	for _, tc := range cases {
		if got := suggestSavedQueryName(tc.in); got != tc.want {
			t.Errorf("suggestSavedQueryName(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestBuildSavedQueryDraft_Success(t *testing.T) {
	lq := skillTemplateLQ()
	resp := &ai.Response{Result: &ai.AIResult{LogicalQuery: &lq}}

	draft, status := buildSavedQueryDraft("revenue by country", resp)
	if status != http.StatusOK {
		t.Fatalf("status = %d, want 200", status)
	}
	if draft.LogicalQuery == nil {
		t.Fatal("expected logical_query in draft")
	}
	if draft.Name != "Revenue by country" {
		t.Errorf("name = %q", draft.Name)
	}
	if draft.Description != "revenue by country" {
		t.Errorf("description = %q", draft.Description)
	}
	if draft.Parameters == nil || len(draft.Parameters) != 0 {
		t.Errorf("parameters = %#v, want empty non-nil slice", draft.Parameters)
	}
	if draft.Error != "" || draft.NeedsClarification {
		t.Errorf("unexpected error/clarification: %+v", draft)
	}
}

func TestBuildSavedQueryDraft_Clarification(t *testing.T) {
	resp := &ai.Response{
		Clarification: &ai.ClarificationResponse{
			NeedsClarification:    true,
			ClarificationQuestion: "which region do you mean?",
		},
	}
	draft, status := buildSavedQueryDraft("sales", resp)
	if status != http.StatusOK {
		t.Fatalf("status = %d, want 200", status)
	}
	if !draft.NeedsClarification {
		t.Fatal("expected needs_clarification")
	}
	if draft.LogicalQuery != nil {
		t.Error("clarification draft must not fabricate a logical_query")
	}
	if draft.Message != "which region do you mean?" {
		t.Errorf("message = %q", draft.Message)
	}
}

func TestBuildSavedQueryDraft_NoQueryReturnsWarning(t *testing.T) {
	resp := &ai.Response{Result: &ai.AIResult{Warnings: []string{"could not map question to any table"}}}
	draft, status := buildSavedQueryDraft("gibberish", resp)
	if status != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want 422", status)
	}
	if draft.LogicalQuery != nil {
		t.Error("empty-query draft must not fabricate a logical_query")
	}
	if draft.Error != "could not map question to any table" {
		t.Errorf("error = %q", draft.Error)
	}
}

func TestBuildSavedQueryDraft_NoQueryNoWarning(t *testing.T) {
	draft, status := buildSavedQueryDraft("gibberish", &ai.Response{Result: &ai.AIResult{}})
	if status != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want 422", status)
	}
	if draft.Error == "" {
		t.Error("expected a fallback error message")
	}
}
