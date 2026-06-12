package prompt

const (
	maxFewShotCompact  = 2
	maxFewShotStandard = 5
	maxFewShotExpanded = 12

	maxPriorTurnsCompact    = 2
	maxPriorTurnsStandard   = 5
	maxPriorTurnsExpanded   = 8
	priorTurnTokensCompact  = 150
	priorTurnTokensStandard = 250
	priorTurnTokensExpanded = 400

	maxGlossaryCompact  = 12
	maxGlossaryStandard = 25
	maxGlossaryExpanded = 40
)

// ContextTierForAttempt maps the retry loop index to a progressive context tier.
func ContextTierForAttempt(attempt int) int {
	if attempt <= 0 {
		return 0
	}
	if attempt == 1 {
		return 1
	}
	return 2
}

// FewShotCap returns the maximum number of few-shot examples for a context tier.
func FewShotCap(tier int) int {
	switch tier {
	case 0:
		return maxFewShotCompact
	case 1:
		return maxFewShotStandard
	default:
		return maxFewShotExpanded
	}
}

// PriorTurnsCap returns the maximum number of prior conversation turns for a context tier.
func PriorTurnsCap(tier int) int {
	switch tier {
	case 0:
		return maxPriorTurnsCompact
	case 1:
		return maxPriorTurnsStandard
	default:
		return maxPriorTurnsExpanded
	}
}

// PriorTurnsTokenBudget returns the approximate total token budget for prior
// conversation turns, including result summaries.
func PriorTurnsTokenBudget(tier int) int {
	switch tier {
	case 0:
		return priorTurnTokensCompact
	case 1:
		return priorTurnTokensStandard
	default:
		return priorTurnTokensExpanded
	}
}

// GlossaryCap returns the maximum number of glossary entries for a context tier.
func GlossaryCap(tier int) int {
	switch tier {
	case 0:
		return maxGlossaryCompact
	case 1:
		return maxGlossaryStandard
	default:
		return maxGlossaryExpanded
	}
}

// TailSlice returns the trailing max elements of items, or nil when empty.
func TailSlice[T any](items []T, maxItems int) []T {
	if len(items) == 0 || maxItems <= 0 {
		return nil
	}
	if len(items) <= maxItems {
		return items
	}
	return append([]T(nil), items[len(items)-maxItems:]...)
}

// TailPriorTurns returns the newest prior turns that fit both the count cap and
// the tier's rough token budget. The newest turn is kept even if it alone
// exceeds the budget so references to the immediate prior answer still work.
func TailPriorTurns(turns []ConversationTurn, tier int) []ConversationTurn {
	if len(turns) == 0 {
		return nil
	}
	maxItems := PriorTurnsCap(tier)
	if maxItems <= 0 {
		return nil
	}
	budget := PriorTurnsTokenBudget(tier)
	out := make([]ConversationTurn, 0, min(len(turns), maxItems))
	used := 0
	for i := len(turns) - 1; i >= 0 && len(out) < maxItems; i-- {
		cost := estimatePriorTurnTokens(turns[i])
		if len(out) > 0 && used+cost > budget {
			break
		}
		out = append(out, turns[i])
		used += cost
	}
	for i, j := 0, len(out)-1; i < j; i, j = i+1, j-1 {
		out[i], out[j] = out[j], out[i]
	}
	return out
}

func estimatePriorTurnTokens(turn ConversationTurn) int {
	return EstimateTokens(turn.Question) +
		EstimateTokens(turn.LogicalQuery) +
		EstimateTokens(turn.Note) +
		EstimateTokens(turn.ResultSummary) +
		12
}

// TailGlossary returns the leading maxItems glossary entries, or nil when empty.
func TailGlossary(entries []GlossaryEntry, maxItems int) []GlossaryEntry {
	if len(entries) == 0 || maxItems <= 0 {
		return nil
	}
	if len(entries) <= maxItems {
		return entries
	}
	return append([]GlossaryEntry(nil), entries[:maxItems]...)
}
