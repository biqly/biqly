package metadata

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPruneStaleMetadataDeletesGhostObjects(t *testing.T) {
	db, state := setupMockDB(t)
	repo := NewRepository(db)

	state.execs = []execMock{
		{Pattern: "DELETE FROM relations", RowsAffected: 1},
		{Pattern: "DELETE FROM columns", RowsAffected: 4},
		{Pattern: "DELETE FROM tables", RowsAffected: 2},
		{Pattern: "DELETE FROM schemas", RowsAffected: 0},
	}

	result, err := repo.PruneStaleMetadata(context.Background(), "ds-1", SyncSnapshotKeys{
		SchemaNames:         []string{"public"},
		TableKeys:           []string{"public.users"},
		ColumnKeys:          []string{"public.users.id"},
		RelationConstraints: []string{"fk_users"},
	})
	require.NoError(t, err)
	assert.Equal(t, 1, result.Relations)
	assert.Equal(t, 4, result.Columns)
	assert.Equal(t, 2, result.Tables)
	assert.Equal(t, 0, result.Schemas)
	assert.Equal(t, 7, result.Total())
}

func TestPruneStaleMetadataEmptyKeepListsStillExecute(t *testing.T) {
	db, state := setupMockDB(t)
	repo := NewRepository(db)
	state.execs = []execMock{
		{Pattern: "DELETE FROM relations", RowsAffected: 0},
		{Pattern: "DELETE FROM columns", RowsAffected: 0},
		{Pattern: "DELETE FROM tables", RowsAffected: 0},
		{Pattern: "DELETE FROM schemas", RowsAffected: 0},
	}
	// Nil slices must not panic or match NULL — they become empty arrays.
	_, err := repo.PruneStaleMetadata(context.Background(), "ds-1", SyncSnapshotKeys{})
	require.NoError(t, err)
}
