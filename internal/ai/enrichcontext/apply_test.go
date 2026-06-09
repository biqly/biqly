package enrichcontext

import (
	"context"
	"testing"

	"github.com/biqly/biqly/internal/metadata"
	"github.com/biqly/biqly/internal/semantic"
	"github.com/stretchr/testify/require"
)

func TestApplyOne_GlossaryRequiresRow(t *testing.T) {
	model := &semantic.SemanticModel{ID: "m1"}
	err := applyOne(
		context.Background(),
		model,
		map[string]metadata.BusinessGlossaryRow{},
		map[string]semantic.Dimension{},
		map[string]semantic.Metric{},
		nil,
		nil,
		"glossary:missing",
		"definition",
	)
	require.Error(t, err)
}
