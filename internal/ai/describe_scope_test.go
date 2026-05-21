package ai

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDescribeBatchScopeSchemas(t *testing.T) {
	schemas := DescribeBatchScopeSchemas([]DescribeBatchTable{
		{Schema: "public", Table: "a"},
		{Schema: "dbo", Table: "b"},
		{Schema: "public", Table: "c"},
	})
	require.Equal(t, []string{"dbo", "public"}, schemas)
}

func TestSchemasOverlap(t *testing.T) {
	require.True(t, SchemasOverlap([]string{"public"}, []string{"public", "dbo"}))
	require.False(t, SchemasOverlap([]string{"public"}, []string{"dbo"}))
}
