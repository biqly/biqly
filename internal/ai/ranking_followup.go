package ai

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/biqly/biqly/internal/i18n"
	"github.com/biqly/biqly/internal/query"
)

const defaultRankingMetric = "count"

var (
	topNFollowUpPattern = regexp.MustCompile(`(?i)\b(?:ilk|top|first|en\s+çok)\s+(\d+)\b`)
	minuteWindowPattern = regexp.MustCompile(`(?i)\b(?:\d+\s*)?(?:dakika|dk|minute|min(?:ute)?s?)\b`)
)

func parseTopNFollowUp(question string) (int, bool) {
	q := strings.TrimSpace(question)
	if q == "" {
		return 0, false
	}
	if minuteWindowPattern.MatchString(q) {
		return 0, false
	}
	m := topNFollowUpPattern.FindStringSubmatch(q)
	if len(m) < 2 {
		return 0, false
	}
	n, err := strconv.Atoi(m[1])
	if err != nil || n <= 0 || n > 1000 {
		return 0, false
	}
	return n, true
}

func rankingFollowUpApplies(question string, session *FilterSessionState, intent FollowUpIntent) (int, bool) {
	if session == nil || intent != IntentRefine || strings.TrimSpace(session.RankingDimension) == "" {
		return 0, false
	}
	return parseTopNFollowUp(question)
}

// RankingFollowUpInstructions adds prompt guidance when the user asks for a top-N
// ranking continuation of the previous grouped result (e.g. "ilk 10" after an
// author leaderboard). "dakiler" is colloquial for "ones/them", not "dakika".
func RankingFollowUpInstructions(session *FilterSessionState, intent FollowUpIntent, question string) string {
	n, ok := rankingFollowUpApplies(question, session, intent)
	if !ok {
		return ""
	}
	metric := strings.TrimSpace(session.RankingMetric)
	if metric == "" {
		metric = defaultRankingMetric
	}
	var b strings.Builder
	_, _ = b.WriteString("\n\n## Ranking Follow-up\n")
	_, _ = fmt.Fprintf(&b, "The user wants the top %d rows from the previous %s ranking.\n", n, session.RankingDimension)
	_, _ = fmt.Fprintf(&b, "Return %s (dimension) and %s (metric), GROUP BY %s, ORDER BY %s DESC, LIMIT %d.\n",
		session.RankingDimension, metric, session.RankingDimension, metric, n)
	_, _ = b.WriteString("Do NOT interpret \"ilk N dakiler\" / \"top N ones\" as a minute-level time window. ")
	_, _ = b.WriteString("Keep the active date filters; do not add sub-hour timestamp BETWEEN filters unless the user explicitly asks for minutes or hours.\n")
	return b.String()
}

// applyRankingFollowUpPostCheck repairs top-N ranking follow-ups when the model
// drops the ranking dimension or invents a short time window (e.g. first 10 minutes).
func applyRankingFollowUpPostCheck(
	ctx context.Context,
	question string,
	session *FilterSessionState,
	intent FollowUpIntent,
	resp *AIResponse,
) {
	if resp == nil || resp.Result == nil || resp.Result.LogicalQuery == nil {
		return
	}
	n, ok := rankingFollowUpApplies(question, session, intent)
	if !ok {
		return
	}
	lq := resp.Result.LogicalQuery
	metric := strings.TrimSpace(session.RankingMetric)
	if metric == "" {
		metric = defaultRankingMetric
	}

	removed := stripSpuriousShortTimeFilters(lq, session)
	fixed := enforceRankingLogicalQuery(lq, session.RankingDimension, metric, n)

	if removed || fixed {
		locale := i18n.FromContext(ctx)
		resp.Result.Warnings = append(resp.Result.Warnings, i18n.Tf(locale, "clarification.ranking_followup_corrected", map[string]any{
			"Dimension": session.RankingDimension,
			"Limit":     n,
		}))
	}
}

func stripSpuriousShortTimeFilters(lq *query.LogicalQuery, session *FilterSessionState) bool {
	if lq == nil || session == nil {
		return false
	}
	sessionKeys := make(map[string]struct{}, len(session.Filters))
	for _, f := range session.Filters {
		sessionKeys[filterKey(f)] = struct{}{}
	}
	kept := make([]query.Filter, 0, len(lq.Filters))
	removed := false
	for _, f := range lq.Filters {
		if f.Operator == query.OpBetween && isShortTimestampBetween(f) {
			if _, inherited := sessionKeys[filterKey(f)]; !inherited {
				removed = true
				continue
			}
		}
		kept = append(kept, f)
	}
	if removed {
		lq.Filters = kept
	}
	return removed
}

func isShortTimestampBetween(f query.Filter) bool {
	field := strings.ToLower(strings.TrimSpace(f.Field))
	if !strings.Contains(field, "created_at") && !strings.Contains(field, "_at") && !strings.Contains(field, "timestamp") {
		return false
	}
	if strings.HasSuffix(field, "_day") || strings.HasSuffix(field, "_date") {
		return false
	}
	vals, ok := f.Value.([]any)
	if !ok || len(vals) != 2 {
		return false
	}
	start, okStart := vals[0].(string)
	end, okEnd := vals[1].(string)
	if !okStart || !okEnd {
		return false
	}
	return strings.Contains(start, "T00:") || strings.Contains(end, "T00:") || len(start) > 10 || len(end) > 10
}

func enforceRankingLogicalQuery(lq *query.LogicalQuery, dimension, metric string, limit int) bool {
	if lq == nil || dimension == "" || limit <= 0 {
		return false
	}
	changed := false

	hasDim := false
	hasMetric := false
	for _, item := range lq.Select {
		if item.Type == query.SelectTypeDimension && item.Name == dimension {
			hasDim = true
		}
		if item.Type == query.SelectTypeMetric && item.Name == metric {
			hasMetric = true
		}
	}
	if !hasDim {
		lq.Select = append([]query.SelectItem{{Type: query.SelectTypeDimension, Name: dimension}}, lq.Select...)
		changed = true
	}
	if !hasMetric {
		lq.Select = append(lq.Select, query.SelectItem{Type: query.SelectTypeMetric, Name: metric})
		changed = true
	}

	hasGroup := false
	for _, gb := range lq.GroupBy {
		if gb.Field == dimension {
			hasGroup = true
			break
		}
	}
	if !hasGroup {
		lq.GroupBy = append(lq.GroupBy, query.GroupBy{Field: dimension})
		changed = true
	}

	hasOrder := false
	for _, ob := range lq.OrderBy {
		if ob.Field == metric && strings.EqualFold(ob.Direction, "desc") {
			hasOrder = true
			break
		}
	}
	if !hasOrder {
		lq.OrderBy = []query.OrderBy{{Field: metric, Direction: "desc"}}
		changed = true
	}

	if lq.Limit != limit {
		lq.Limit = limit
		changed = true
	}
	return changed
}
