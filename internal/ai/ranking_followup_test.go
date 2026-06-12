package ai

import (
	"context"
	"strings"
	"testing"

	"github.com/biqly/biqly/internal/i18n"
	"github.com/biqly/biqly/internal/query"
	"github.com/stretchr/testify/require"
)

func TestParseTopNFollowUp(t *testing.T) {
	t.Parallel()
	cases := []struct {
		q      string
		wantN  int
		wantOK bool
	}{
		{"peki ilk 10 dakiler? yanlarında da tweet atma sayıları olsun", 10, true},
		{"top 5 authors", 5, true},
		{"ilk 10 dakika içinde kaç tweet", 0, false},
		{"son 10 dk", 0, false},
		{"bugün kaç tweet", 0, false},
	}
	for _, tc := range cases {
		n, ok := parseTopNFollowUp(tc.q)
		if ok != tc.wantOK || n != tc.wantN {
			t.Errorf("parseTopNFollowUp(%q) = (%d, %v), want (%d, %v)", tc.q, n, ok, tc.wantN, tc.wantOK)
		}
	}
}

func TestRankingFollowUpInstructions(t *testing.T) {
	sess := &FilterSessionState{
		RankingDimension: "author_name",
		RankingMetric:    "count",
	}
	got := RankingFollowUpInstructions(sess, IntentRefine, "peki ilk 10 dakiler?")
	if !strings.Contains(got, "author_name") || !strings.Contains(got, "LIMIT 10") {
		t.Fatalf("expected ranking instructions, got:\n%s", got)
	}
	if !strings.Contains(got, "dakiler") {
		t.Fatalf("expected dakiler guidance, got:\n%s", got)
	}
}

func TestApplyRankingFollowUpPostCheckFixesMinuteMisread(t *testing.T) {
	ctx := i18n.WithLocale(context.Background(), i18n.LocaleTR)
	sess := &FilterSessionState{
		Filters: []query.Filter{
			{Field: "created_at_ts_2_day", Operator: query.OpEq, Value: "2026-06-12"},
		},
		RankingDimension: "author_name",
		RankingMetric:    "count",
	}
	resp := &AIResponse{
		Result: &AIResult{
			Confidence: 0.9,
			LogicalQuery: &query.LogicalQuery{
				Select: []query.SelectItem{{Type: query.SelectTypeMetric, Name: "count"}},
				Filters: []query.Filter{
					{Field: "created_at_ts_2_day", Operator: query.OpEq, Value: "2026-06-12"},
					{Field: "created_at_ts", Operator: query.OpBetween, Value: []any{"2026-06-12T00:00:00", "2026-06-12T00:10:00"}},
				},
				Limit: 1,
			},
		},
	}

	applyRankingFollowUpPostCheck(ctx, "peki ilk 10 dakiler?", sess, IntentRefine, resp)

	lq := resp.Result.LogicalQuery
	require.Equal(t, 10, lq.Limit)
	require.Len(t, lq.GroupBy, 1)
	require.Equal(t, "author_name", lq.GroupBy[0].Field)
	for _, f := range lq.Filters {
		require.NotEqual(t, query.OpBetween, f.Operator, "spurious short time filter should be removed")
	}
	require.NotEmpty(t, resp.Result.Warnings)
}

func TestRankingShapeFromPriorAuthorQuery(t *testing.T) {
	lq := &query.LogicalQuery{
		Select: []query.SelectItem{
			{Type: query.SelectTypeDimension, Name: "author_name"},
			{Type: query.SelectTypeMetric, Name: "count"},
		},
		GroupBy: []query.GroupBy{{Field: "author_name"}},
	}
	dim, metric := rankingShapeFromLogicalQuery(lq)
	require.Equal(t, "author_name", dim)
	require.Equal(t, "count", metric)
}
