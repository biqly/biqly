package metadata

import (
	"context"
	"testing"

	"github.com/biqly/biqly/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInstructionsCRUDAndActiveListing(t *testing.T) {
	db := testutil.OpenMetadataDB(t)
	ctx := context.Background()
	repo := NewRepository(db)

	const datasourceID = "00000000-0000-0000-0000-0000000059b1"
	testutil.EnsureMetadataTestDatasource(ctx, t, db, datasourceID, "instruction-test")
	_, err := db.ExecContext(ctx, `DELETE FROM ai_instructions WHERE datasource_id = $1::uuid`, datasourceID)
	require.NoError(t, err)

	activeID, err := repo.InsertInstruction(ctx, InstructionInsert{
		DatasourceID: datasourceID,
		Title:        "Fiscal year",
		BodyMD:       "The fiscal year starts in April.",
	})
	require.NoError(t, err)
	require.NotEmpty(t, activeID)

	inactiveID, err := repo.InsertInstruction(ctx, InstructionInsert{
		DatasourceID: datasourceID,
		Title:        "Deprecated rule",
		BodyMD:       "Do not use.",
	})
	require.NoError(t, err)
	require.NoError(t, repo.UpdateInstruction(ctx, inactiveID, InstructionUpdate{
		Title:    "Deprecated rule",
		BodyMD:   "Do not use.",
		IsActive: false,
	}))

	all, err := repo.ListInstructions(ctx, datasourceID)
	require.NoError(t, err)
	assert.Len(t, all, 2)

	active, err := repo.ListActiveInstructions(ctx, datasourceID)
	require.NoError(t, err)
	require.Len(t, active, 1)
	assert.Equal(t, activeID, active[0].ID)
	assert.Equal(t, "Fiscal year", active[0].Title)
	assert.Equal(t, "The fiscal year starts in April.", active[0].BodyMD)

	got, err := repo.GetInstruction(ctx, activeID)
	require.NoError(t, err)
	assert.True(t, got.IsActive)

	require.NoError(t, repo.DeleteInstruction(ctx, activeID))
	_, err = repo.GetInstruction(ctx, activeID)
	assert.ErrorIs(t, err, ErrInstructionNotFound)
}
