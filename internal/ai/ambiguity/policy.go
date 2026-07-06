package ambiguity

// ClarificationTieEpsilon is the minimum confidence gap between the top two
// surviving interpretations of an ambiguous term for the leader to count as a
// "clear best". At or below this gap the term is a genuine toss-up and must be
// clarified rather than auto-resolved to a default.
const ClarificationTieEpsilon = 0.15

// ChosenDefault pairs an ambiguous term with the interpretation the policy
// selected as its safe default.
type ChosenDefault struct {
	Term           string
	Interpretation Interpretation
}

// Decision is the outcome of the clarification policy for one analysis.
type Decision struct {
	// Clarify is true when the user must be asked to disambiguate; false when
	// every ambiguous term has a safe default the pipeline can proceed with.
	Clarify bool
	// Chosen lists the default interpretation for each ambiguous term, in
	// Result.Ambiguities order. Populated only when Clarify is false.
	Chosen []ChosenDefault
}

// Decide applies the clarification policy on top of the analyzer output. It
// clarifies ONLY when a term is a genuine toss-up (its top two interpretations
// sit within tieEpsilon of each other) — the deterministic detectors assign
// most interpretations equal confidence, so equal-confidence collisions
// (glossary/temporal/scope/exact-synonym) still clarify, while a materially
// stronger interpretation (e.g. an exact synonym match beating a fuzzy one)
// proceeds with that interpretation as the default plus a caveat.
//
// force makes the policy always proceed with the defaults, choosing the
// highest-confidence interpretation of every term even on a tie. It backs the
// first-class "skip" action, which must never dead-end.
//
// A non-positive tieEpsilon falls back to ClarificationTieEpsilon.
func Decide(result Result, tieEpsilon float64, force bool) Decision {
	if !result.IsAmbiguous || len(result.Ambiguities) == 0 {
		return Decision{Clarify: false}
	}
	if tieEpsilon <= 0 {
		tieEpsilon = ClarificationTieEpsilon
	}
	chosen := make([]ChosenDefault, 0, len(result.Ambiguities))
	for _, item := range result.Ambiguities {
		best, isClear := clearBestInterpretation(item, tieEpsilon)
		if !isClear && !force {
			// Genuine toss-up (rule: ties/segment with no default) — ask.
			return Decision{Clarify: true}
		}
		chosen = append(chosen, ChosenDefault{Term: item.Term, Interpretation: best})
	}
	return Decision{Clarify: false, Chosen: chosen}
}

// clearBestInterpretation returns the highest-confidence interpretation and
// whether it is materially ahead of the runner-up by at least tieEpsilon.
func clearBestInterpretation(item Item, tieEpsilon float64) (Interpretation, bool) {
	if len(item.Interpretations) == 0 {
		return Interpretation{}, false
	}
	topIdx := 0
	for i := 1; i < len(item.Interpretations); i++ {
		if item.Interpretations[i].Confidence > item.Interpretations[topIdx].Confidence {
			topIdx = i
		}
	}
	top := item.Interpretations[topIdx]
	if len(item.Interpretations) == 1 {
		return top, true
	}

	secondSet := false
	var second float64
	for i, interp := range item.Interpretations {
		if i == topIdx {
			continue
		}
		if !secondSet || interp.Confidence > second {
			second = interp.Confidence
			secondSet = true
		}
	}
	return top, top.Confidence-second >= tieEpsilon
}

// ApplyDefaults rewrites the question by replacing each chosen term with its
// interpretation's label, mirroring how ResolveChoice applies a user selection.
// Terms that no longer appear in the question are left untouched.
func ApplyDefaults(question string, chosen []ChosenDefault) string {
	for _, c := range chosen {
		if rewritten, ok := replaceTermWithInterpretation(question, c.Term, c.Interpretation); ok {
			question = rewritten
		}
	}
	return question
}
