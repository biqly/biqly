package ai

import (
	"strings"
	"testing"

	"github.com/biqly/biqly/internal/ai/prompt"
	"github.com/biqly/biqly/internal/query"
	"github.com/bytedance/sonic"
	"github.com/stretchr/testify/require"
)

func TestFilterSessionFromPriorTurns(t *testing.T) {
	prevLQ := query.LogicalQuery{
		Filters: []query.Filter{
			{Field: "order_date", Operator: query.OpBetween, Value: []any{"2025-04-01", "2025-04-30"}},
		},
	}
	raw, err := sonic.ConfigStd.Marshal(prevLQ)
	require.NoError(t, err)
	turns := []prompt.ConversationTurn{
		{
			Question:      "geçen ay satışlar",
			LogicalQuery:  string(raw),
			ResultSummary: "May 20, 2026: 2,932 tweets",
		},
	}
	sess := FilterSessionFromPriorTurns(turns)
	if sess == nil || len(sess.Filters) != 1 {
		t.Fatalf("expected session with 1 filter, got %+v", sess)
	}
	if sess.Filters[0].Field != "order_date" {
		t.Errorf("field = %q, want order_date", sess.Filters[0].Field)
	}
	if sess.LastResultSummary != "May 20, 2026: 2,932 tweets" {
		t.Errorf("LastResultSummary = %q", sess.LastResultSummary)
	}
}

func TestClassifyFollowUpIntent(t *testing.T) {
	sess := &FilterSessionState{
		Filters: []query.Filter{{Field: "order_date", Operator: query.OpGte, Value: "2025-04-01"}},
	}
	cases := []struct {
		q    string
		want FollowUpIntent
	}{
		{"bölgeye göre grupla", IntentRefine},
		{"şimdi kategoriye göre", IntentRefine},
		{"peki o gün en çok hangi yazar tweet atmıştır", IntentRefine},
		{"for the date in the previous result, which author tweeted most", IntentRefine},
		{"bu ay satışlar", IntentReplaceFilters},
		{"geçen hafta", IntentReplaceFilters},
		{"yeni soru: tüm müşteriler", IntentNewQuery},
		{"tüm zamanlar ciro", IntentNewQuery},
	}
	for _, c := range cases {
		got := ClassifyFollowUpIntent(c.q, sess)
		if got != c.want {
			t.Errorf("ClassifyFollowUpIntent(%q) = %v, want %v", c.q, got, c.want)
		}
	}
}

func TestActiveFilterInstructionsIncludesPreviousAnswerContext(t *testing.T) {
	sess := &FilterSessionState{
		Filters:           []query.Filter{{Field: "tweet_day", Operator: query.OpEq, Value: "2026-05-20"}},
		LastQuestion:      "geçtiğimiz ay en çok hangi gün tweet atılmıştır?",
		LastResultSummary: "May 20, 2026: 2,932 tweets",
	}

	got := ActiveFilterInstructions(sess, IntentRefine)

	if !strings.Contains(got, "## Previous Answer Context") {
		t.Fatalf("expected previous answer context block, got:\n%s", got)
	}
	if !strings.Contains(got, "May 20, 2026: 2,932 tweets") {
		t.Errorf("expected result summary in instructions, got:\n%s", got)
	}
	if !strings.Contains(got, "o gün") || !strings.Contains(got, "that day") {
		t.Errorf("expected reference-resolution guidance, got:\n%s", got)
	}
}

func TestApplyFilterSessionInheritsMissingFilters(t *testing.T) {
	sess := &FilterSessionState{
		Filters: []query.Filter{
			{Field: "order_date", Operator: query.OpBetween, Value: []any{"2025-04-01", "2025-04-30"}},
		},
	}
	lq := &query.LogicalQuery{
		Select:  []query.SelectItem{{Type: query.SelectTypeDimension, Name: "region"}},
		GroupBy: []query.GroupBy{{Field: "region"}},
	}
	notes := ApplyFilterSession(lq, sess, IntentRefine)
	if len(lq.Filters) != 1 {
		t.Fatalf("expected 1 inherited filter, got %+v", lq.Filters)
	}
	if len(notes) == 0 {
		t.Error("expected inheritance note")
	}
}

func TestApplyFilterSessionSkipsOnReplaceIntent(t *testing.T) {
	sess := &FilterSessionState{
		Filters: []query.Filter{{Field: "order_date", Operator: query.OpGte, Value: "x"}},
	}
	lq := &query.LogicalQuery{
		Select: []query.SelectItem{{Type: query.SelectTypeMetric, Name: "revenue"}},
	}
	notes := ApplyFilterSession(lq, sess, IntentReplaceFilters)
	if len(lq.Filters) != 0 {
		t.Fatalf("expected no merge on replace intent, got %+v", lq.Filters)
	}
	if len(notes) != 0 {
		t.Errorf("expected no notes, got %v", notes)
	}
}

func TestApplyFilterSessionDoesNotDuplicate(t *testing.T) {
	sess := &FilterSessionState{
		Filters: []query.Filter{{Field: "order_date", Operator: query.OpGte, Value: "2025-04-01"}},
	}
	lq := &query.LogicalQuery{
		Select: []query.SelectItem{{Type: query.SelectTypeMetric, Name: "revenue"}},
		Filters: []query.Filter{
			{Field: "order_date", Operator: query.OpGte, Value: "2025-05-01"},
		},
	}
	notes := ApplyFilterSession(lq, sess, IntentRefine)
	if len(lq.Filters) != 1 {
		t.Fatalf("expected single filter (no duplicate field), got %+v", lq.Filters)
	}
	if len(notes) != 0 {
		t.Errorf("expected no inheritance when field already present, got %v", notes)
	}
}
