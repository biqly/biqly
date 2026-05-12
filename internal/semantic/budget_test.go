package semantic

import (
	"strings"
	"testing"
)

func TestEnforceBudgetPassesWithinLimits(t *testing.T) {
	model := SemanticModel{
		Dimensions: make([]Dimension, 5),
		Metrics:    make([]Metric, 3),
		Joins:      make([]Join, 2),
	}
	warnings := EnforceBudget(model, DefaultContextBudget(), 1000)
	if len(warnings) != 0 {
		t.Errorf("expected no warnings, got %v", warnings)
	}
}

func TestEnforceBudgetFlagsEachLimitIndependently(t *testing.T) {
	model := SemanticModel{
		Dimensions: make([]Dimension, 100), // over 80
		Metrics:    make([]Metric, 60),     // over 40
		Joins:      make([]Join, 50),       // over 30
	}
	warnings := EnforceBudget(model, DefaultContextBudget(), 30000) // over 20000

	want := []string{"max_dimensions", "max_metrics", "max_joins", "max_prompt_chars"}
	for _, needle := range want {
		found := false
		for _, w := range warnings {
			if strings.Contains(w, needle) {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected warning containing %q, got %v", needle, warnings)
		}
	}
}

func TestEnforceBudgetZeroLimitMeansUnlimited(t *testing.T) {
	model := SemanticModel{Dimensions: make([]Dimension, 1000)}
	budget := ContextBudget{} // all zero → unlimited
	warnings := EnforceBudget(model, budget, 1_000_000)
	if len(warnings) != 0 {
		t.Errorf("zero limits should disable checks, got %v", warnings)
	}
}

func TestEnforceBudgetSkipsPromptCheckWhenSizeUnknown(t *testing.T) {
	model := SemanticModel{}
	warnings := EnforceBudget(model, DefaultContextBudget(), 0)
	for _, w := range warnings {
		if strings.Contains(w, "max_prompt_chars") {
			t.Errorf("prompt budget should not fire when size is 0, got %v", warnings)
		}
	}
}
