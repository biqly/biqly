package routing

import (
	"sort"
	"strings"

	"github.com/biqly/biqly/internal/semantic"
)

// minCompositeMatchScore is the minimum keyword overlap score a composite must
// reach before it is considered a viable cross-domain routing target.
const minCompositeMatchScore = 1.0

// CompositeCandidate is a scored composite model evaluated against a question.
type CompositeCandidate struct {
	CompositeID     string   `json:"composite_id"`
	Name            string   `json:"name"`
	Score           float64  `json:"score"`
	ComponentModels []string `json:"component_models,omitempty"`
}

// CompositeMatcher scores published composite models against a natural language
// question so the AI service can prefer a cross-domain composite when the
// question clearly spans multiple component domains. It uses the same token
// scoring as the table router so behaviour stays consistent.
type CompositeMatcher struct{}

// NewCompositeMatcher constructs a CompositeMatcher.
func NewCompositeMatcher() *CompositeMatcher {
	return &CompositeMatcher{}
}

// Match scores each published composite against the question and returns the
// candidates sorted by descending score. Only composites scoring at or above
// minCompositeMatchScore are returned. Drafts (Status != "published") are
// skipped so unpublished work never affects routing.
func (m *CompositeMatcher) Match(question string, composites []semantic.CompositeModel) []CompositeCandidate {
	questionTokens := tokenSet(question)
	var out []CompositeCandidate
	for i := range composites {
		c := composites[i]
		if c.Status != "published" {
			continue
		}
		score := scoreComposite(questionTokens, &c)
		if score < minCompositeMatchScore {
			continue
		}
		out = append(out, CompositeCandidate{
			CompositeID:     c.ID,
			Name:            c.Name,
			Score:           score,
			ComponentModels: componentModelIDs(&c),
		})
	}
	sort.SliceStable(out, func(i, j int) bool {
		return out[i].Score > out[j].Score
	})
	return out
}

// Best returns the highest-scoring composite candidate, or nil when none clear
// the match threshold.
func (m *CompositeMatcher) Best(question string, composites []semantic.CompositeModel) *CompositeCandidate {
	candidates := m.Match(question, composites)
	if len(candidates) == 0 {
		return nil
	}
	best := candidates[0]
	return &best
}

// scoreComposite accumulates keyword overlap across the composite's name,
// label, description and component aliases. A composite that matches multiple
// component domains scores higher, which is exactly the cross-domain signal we
// want to reward.
func scoreComposite(questionTokens map[string]bool, c *semantic.CompositeModel) float64 {
	score := weightedTokenScore(questionTokens, c.Name, 1.0)
	if c.Label != nil {
		score += weightedTokenScore(questionTokens, *c.Label, 1.0)
	}
	if c.Description != nil {
		score += weightedTokenScore(questionTokens, *c.Description, 0.5)
	}
	matchedComponents := 0
	for _, comp := range c.Components {
		if weightedTokenScore(questionTokens, comp.Alias, 1.0) > 0 {
			matchedComponents++
		}
	}
	// Reward questions that reference two or more component domains — the
	// defining characteristic of a cross-domain composite query.
	if matchedComponents >= 2 {
		score += float64(matchedComponents)
	}
	return score
}

func componentModelIDs(c *semantic.CompositeModel) []string {
	ids := make([]string, 0, len(c.Components))
	for _, comp := range c.Components {
		if strings.TrimSpace(comp.ModelID) != "" {
			ids = append(ids, comp.ModelID)
		}
	}
	return ids
}
