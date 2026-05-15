package ai

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

// contextTierForAttempt maps the retry loop index to a progressive context tier.
func contextTierForAttempt(attempt int) int {
	if attempt <= 0 {
		return 0
	}
	if attempt == 1 {
		return 1
	}
	return 2
}

type tieredProcessOptions struct {
	fewShot      []FewShotExample
	samples      []TableSample
	priorTurns   []ConversationTurn
	deniedFields []string
	glossary     []GlossaryEntry
}

func applyContextTier(base processOptions, tier int) tieredProcessOptions {
	return tieredProcessOptions{
		fewShot:      tailFewShot(base.fewShot, fewShotCap(tier)),
		samples:      base.samples,
		priorTurns:   tailPriorTurns(base.priorTurns, priorTurnsCap(tier)),
		deniedFields: base.deniedFields,
		glossary:     tailGlossary(base.glossary, glossaryCap(tier)),
	}
}

func fewShotCap(tier int) int {
	switch tier {
	case 0:
		return maxFewShotCompact
	case 1:
		return maxFewShotStandard
	default:
		return maxFewShotExpanded
	}
}

func priorTurnsCap(tier int) int {
	switch tier {
	case 0:
		return maxPriorTurnsCompact
	case 1:
		return maxPriorTurnsStandard
	default:
		return maxPriorTurnsExpanded
	}
}

func glossaryCap(tier int) int {
	switch tier {
	case 0:
		return maxGlossaryCompact
	case 1:
		return maxGlossaryStandard
	default:
		return maxGlossaryExpanded
	}
}

func tailFewShot(examples []FewShotExample, max int) []FewShotExample {
	if len(examples) == 0 || max <= 0 {
		return nil
	}
	if len(examples) <= max {
		return examples
	}
	return append([]FewShotExample(nil), examples[len(examples)-max:]...)
}

func tailPriorTurns(turns []ConversationTurn, max int) []ConversationTurn {
	if len(turns) == 0 || max <= 0 {
		return nil
	}
	if len(turns) <= max {
		return turns
	}
	return append([]ConversationTurn(nil), turns[len(turns)-max:]...)
}

func tailGlossary(entries []GlossaryEntry, max int) []GlossaryEntry {
	if len(entries) == 0 || max <= 0 {
		return nil
	}
	if len(entries) <= max {
		return entries
	}
	return append([]GlossaryEntry(nil), entries[:max]...)
}
