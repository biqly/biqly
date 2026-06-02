package eval

import (
	"context"
	"testing"

	ambiguitypkg "github.com/biqly/biqly/internal/ai/ambiguity"
)

func TestAmbiguityCases(t *testing.T) {
	for _, testCase := range AmbiguityCases() {
		t.Run(testCase.ID, func(t *testing.T) {
			got := ambiguitypkg.Analyze(context.Background(), testCase.Question, testCase.Model, testCase.Glossary, 0)
			if got.IsAmbiguous != testCase.ExpectedAmbiguous {
				t.Fatalf("Analyze(%q).IsAmbiguous = %t, want %t; result = %#v", testCase.Question, got.IsAmbiguous, testCase.ExpectedAmbiguous, got)
			}
		})
	}
}
