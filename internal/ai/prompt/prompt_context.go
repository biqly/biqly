package prompt

const (
	maxFewShotCompact  = 2
	maxFewShotStandard = 5
	maxFewShotExpanded = 12

	maxPriorTurnsCompact  = 2
	maxPriorTurnsStandard = 5
	maxPriorTurnsExpanded = 8

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
