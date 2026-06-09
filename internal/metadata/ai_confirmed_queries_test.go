package metadata

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestQuestionHashNormalizesCaseAndWhitespace(t *testing.T) {
	a := QuestionHash("  Total Revenue  ")
	b := QuestionHash("total revenue")
	assert.Equal(t, a, b)
}

func TestSemanticModelHashIncludesVersion(t *testing.T) {
	assert.Equal(t, "model-1@3", SemanticModelHash("model-1", 3))
	assert.NotEqual(t, SemanticModelHash("model-1", 2), SemanticModelHash("model-1", 3))
}
