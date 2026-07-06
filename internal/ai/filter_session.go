package ai

import (
	"fmt"
	"github.com/bytedance/sonic"
	"regexp"
	"strings"

	promptpkg "github.com/biqly/biqly/internal/ai/prompt"
	"github.com/biqly/biqly/internal/query"
)

// FollowUpIntent classifies how the current question relates to active filters
// from the prior successful turn in a conversation.
type FollowUpIntent int

const (
	IntentUnknown FollowUpIntent = iota
	IntentNewQuery
	IntentRefine
	IntentReplaceFilters
)

func (i FollowUpIntent) String() string {
	switch i {
	case IntentUnknown:
		return "unknown"
	case IntentNewQuery:
		return "new_query"
	case IntentRefine:
		return "refine"
	case IntentReplaceFilters:
		return "replace_filters"
	default:
		return "unknown"
	}
}

// FilterSessionState holds filters that remain active across follow-up turns
// until the user starts a new query or explicitly changes the time scope.
type FilterSessionState struct {
	Filters           []query.Filter
	Having            []query.Filter
	LastQuestion      string
	LastResultSummary string
	RankingDimension  string
	RankingMetric     string
}

// FilterSessionFromPriorTurns extracts the most recent successful LogicalQuery
// from conversation history and returns its filters as the active session.
func FilterSessionFromPriorTurns(turns []promptpkg.ConversationTurn) *FilterSessionState {
	for i := len(turns) - 1; i >= 0; i-- {
		lq := parseLogicalQueryFromTurn(turns[i])
		if lq == nil || len(lq.Filters) == 0 && len(lq.Having) == 0 && len(lq.GroupBy) == 0 {
			continue
		}
		rankDim, rankMetric := rankingShapeFromLogicalQuery(lq)
		return &FilterSessionState{
			Filters:           cloneFilters(lq.Filters),
			Having:            cloneFilters(lq.Having),
			LastQuestion:      strings.TrimSpace(turns[i].Question),
			LastResultSummary: strings.TrimSpace(turns[i].ResultSummary),
			RankingDimension:  rankDim,
			RankingMetric:     rankMetric,
		}
	}
	return nil
}

func rankingShapeFromLogicalQuery(lq *query.LogicalQuery) (dimension, metric string) {
	if lq == nil {
		return "", ""
	}
	if len(lq.GroupBy) > 0 {
		dimension = strings.TrimSpace(lq.GroupBy[0].Field)
	}
	if dimension == "" {
		for _, item := range lq.Select {
			if item.Type == query.SelectTypeDimension && strings.TrimSpace(item.Name) != "" {
				dimension = strings.TrimSpace(item.Name)
				break
			}
		}
	}
	for _, item := range lq.Select {
		if item.Type == query.SelectTypeMetric && strings.TrimSpace(item.Name) != "" {
			metric = strings.TrimSpace(item.Name)
			break
		}
	}
	if metric == "" {
		metric = "count"
	}
	return dimension, metric
}

func parseLogicalQueryFromTurn(t promptpkg.ConversationTurn) *query.LogicalQuery {
	raw := strings.TrimSpace(t.LogicalQuery)
	if raw == "" {
		return nil
	}
	var lq query.LogicalQuery
	if err := sonic.ConfigStd.Unmarshal([]byte(raw), &lq); err != nil {
		return nil
	}
	return &lq
}

func cloneFilters(in []query.Filter) []query.Filter {
	if len(in) == 0 {
		return nil
	}
	out := make([]query.Filter, len(in))
	copy(out, in)
	return out
}

var (
	replaceFilterPatterns []*regexp.Regexp
	newQueryPatterns      []*regexp.Regexp
	refinePatterns        []*regexp.Regexp
)

func init() {
	replaceFilterPatterns = compileFollowUpPatterns([]string{
		`(?i)\b(geçen|önceki|bu|son)\s+(ay|hafta|yıl|çeyrek|quarter|month|week|year)\b`,
		`(?i)\b(last|this|previous|next)\s+(month|week|year|quarter)\b`,
		`(?i)\bson\s+\d+\s+(gün|günler|day|days|hafta|weeks?)\b`,
		`(?i)\b(past|last)\s+\d+\s+(days?|weeks?|months?)\b`,
		`(?i)\b(20\d{2})\b`,
		`(?i)\b(tarih|date|period|dönem)\s*(değiş|change|olarak|instead)\b`,
		`(?i)\b(artık|instead|yerine)\b`,
	})
	newQueryPatterns = compileFollowUpPatterns([]string{
		`(?i)\b(yeni\s+soru|baştan|sıfırdan|from\s+scratch|new\s+question)\b`,
		`(?i)\b(tüm\s+zamanlar|all\s+time|without\s+filter|filtresiz)\b`,
		`(?i)\b(farklı\s+(tablo|model|konu)|different\s+(table|topic))\b`,
	})
	refinePatterns = compileFollowUpPatterns([]string{
		`(?i)\b(göre|grupla|group\s+by|break\s+down|sırala|sort\s+by|order\s+by)\b`,
		`(?i)\b(bölge|region|kategori|category|müşteri|customer)\s*(göre|bazında|by)\b`,
		`(?i)\b(şimdi|now|also|ayrıca|ekle|add|sadece\s+grupla)\b`,
		`(?i)\b(o\s+(gün|tarih|şirket|müşteri)|that\s+(day|date|company|customer)|previous\s+result|previous\s+answer)\b`,
		`(?i)\b(top\s+\d+|ilk\s+\d+|limit)\b`,
		`(?i)^(bölgeye|regiona|kategoriye)\s+göre`,
	})
}

func compileFollowUpPatterns(src []string) []*regexp.Regexp {
	out := make([]*regexp.Regexp, len(src))
	for i, pattern := range src {
		out[i] = regexp.MustCompile(pattern)
	}
	return out
}

// ClassifyFollowUpIntent decides whether the current question should inherit
// filters from the session, replace them, or start fresh.
func ClassifyFollowUpIntent(question string, session *FilterSessionState) FollowUpIntent {
	if session == nil || (len(session.Filters) == 0 && len(session.Having) == 0) {
		return IntentNewQuery
	}
	q := strings.TrimSpace(question)
	if q == "" {
		return IntentNewQuery
	}
	for _, re := range newQueryPatterns {
		if re.MatchString(q) {
			return IntentNewQuery
		}
	}
	for _, re := range replaceFilterPatterns {
		if re.MatchString(q) {
			return IntentReplaceFilters
		}
	}
	for _, re := range refinePatterns {
		if re.MatchString(q) {
			return IntentRefine
		}
	}
	words := len(strings.Fields(q))
	if words <= 8 {
		return IntentRefine
	}
	return IntentNewQuery
}

// ApplyFilterSession merges active session filters into lq when intent is
// IntentRefine and the model omitted filters present in the session.
// Returns human-readable notes for warnings / prompt context.
func ApplyFilterSession(lq *query.LogicalQuery, session *FilterSessionState, intent FollowUpIntent) []string {
	if lq == nil || session == nil || intent != IntentRefine {
		return nil
	}
	var notes []string
	inheritable := session.Filters
	if owned := formulaMeasureFilterFields(lq); len(owned) > 0 {
		// A comparison formula owns its measure-filter fields (the period axis:
		// left day=5 vs right day=4). Re-applying a prior turn's value for those
		// fields as a top-level WHERE would zero out the other side of the
		// comparison, so exclude them from inheritance.
		inheritable = make([]query.Filter, 0, len(session.Filters))
		for _, f := range session.Filters {
			if _, ok := owned[f.Field]; ok {
				continue
			}
			inheritable = append(inheritable, f)
		}
	}
	added := mergeMissingFilters(&lq.Filters, inheritable)
	if len(added) > 0 {
		notes = append(notes, fmt.Sprintf("inherited %d filter(s) from prior turn: %s", len(added), filterSummary(added)))
	}
	addedHaving := mergeMissingFilters(&lq.Having, session.Having)
	if len(addedHaving) > 0 {
		notes = append(notes, fmt.Sprintf("inherited %d having filter(s) from prior turn: %s", len(addedHaving), filterSummary(addedHaving)))
	}
	return notes
}

// formulaMeasureFilterFields collects the filter fields referenced inside
// formula measure refs (both sides). A formula select item encodes its own
// slice of the data per side; those fields must not be re-imposed globally.
func formulaMeasureFilterFields(lq *query.LogicalQuery) map[string]struct{} {
	owned := make(map[string]struct{})
	for i := range lq.Select {
		spec := lq.Select[i].Formula
		if lq.Select[i].Type != query.SelectTypeFormula || spec == nil {
			continue
		}
		for _, f := range spec.Left.Filters {
			owned[f.Field] = struct{}{}
		}
		for _, f := range spec.Right.Filters {
			owned[f.Field] = struct{}{}
		}
	}
	return owned
}

func mergeMissingFilters(target *[]query.Filter, inherited []query.Filter) []query.Filter {
	if len(inherited) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(*target))
	for _, f := range *target {
		seen[filterKey(f)] = struct{}{}
	}
	var added []query.Filter
	for _, f := range inherited {
		k := filterKey(f)
		if _, ok := seen[k]; ok {
			continue
		}
		*target = append(*target, f)
		added = append(added, f)
		seen[k] = struct{}{}
	}
	return added
}

func filterKey(f query.Filter) string {
	return f.Field + "|" + f.Operator
}

func filterSummary(filters []query.Filter) string {
	parts := make([]string, 0, len(filters))
	for _, f := range filters {
		parts = append(parts, fmt.Sprintf("%s %s %v", f.Field, f.Operator, f.Value))
	}
	return strings.Join(parts, "; ")
}

// ActiveFilterInstructions returns prompt text describing filters the model
// must keep unless the user changes the time scope or starts a new question.
func ActiveFilterInstructions(session *FilterSessionState, intent FollowUpIntent) string {
	if session == nil || intent != IntentRefine {
		return ""
	}
	if len(session.Filters) == 0 && len(session.Having) == 0 {
		return ""
	}
	var b strings.Builder
	if session.LastResultSummary != "" {
		_, _ = b.WriteString("\n\n## Previous Answer Context\n")
		if session.LastQuestion != "" {
			_, err := fmt.Fprintf(&b, "The previous question %q yielded:\n", session.LastQuestion)
			if err != nil {
				return ""
			}
		}
		_, err := fmt.Fprintf(&b, "Result: %s\n", session.LastResultSummary)
		if err != nil {
			return ""
		}
		_, _ = b.WriteString("When the user says \"o gün\", \"that day\", \"o şirket\", or similar references, resolve them to the relevant value from this result.\n")
	}
	_, _ = b.WriteString("\n\n## Active Filters (carry forward)\n")
	_, _ = b.WriteString("The previous turn established these filters. Keep them in your LogicalQuery unless the user explicitly changes the date/period or asks for a completely new analysis.\n")
	if len(session.Filters) > 0 {
		_, _ = b.WriteString("WHERE filters to preserve:\n")
		for _, f := range session.Filters {
			_, err := fmt.Fprintf(&b, "- %s %s %v\n", f.Field, f.Operator, f.Value)
			if err != nil {
				return ""
			}
		}
	}
	if len(session.Having) > 0 {
		_, _ = b.WriteString("HAVING filters to preserve:\n")
		for _, f := range session.Having {
			_, err := fmt.Fprintf(&b, "- %s %s %v\n", f.Field, f.Operator, f.Value)
			if err != nil {
				return ""
			}
		}
	}
	return b.String()
}
