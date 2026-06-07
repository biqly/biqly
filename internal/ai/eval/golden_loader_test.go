package eval

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/biqly/biqly/internal/query"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGoldenLoader(t *testing.T) {
	tempDir := t.TempDir()

	// 1. Initially empty directory
	cases, err := LoadGoldenCasesFromDir(tempDir)
	require.NoError(t, err)
	assert.Empty(t, cases)

	// 2. Save a case
	expectedQuery := query.LogicalQuery{
		Select: []query.SelectItem{{Type: "metric", Name: "row_count"}},
		Limit:  50,
	}
	err = SaveGoldenCaseToDir(tempDir, "test-case-1", "how many rows?", "orders", expectedQuery)
	require.NoError(t, err)

	// Verify file was created
	assert.FileExists(t, filepath.Join(tempDir, "test-case-1.json"))

	// 3. Load the case
	cases, err = LoadGoldenCasesFromDir(tempDir)
	require.NoError(t, err)
	require.Len(t, cases, 1)

	c := cases[0]
	assert.Equal(t, "test-case-1", c.ID)
	assert.Equal(t, "how many rows?", c.Question)
	assert.Equal(t, "orders", c.Model.ID)
	assert.Equal(t, expectedQuery.Limit, c.Expected.Limit)
	assert.Equal(t, expectedQuery.Select[0].Name, c.Expected.Select[0].Name)

	// 4. Save second case
	err = SaveGoldenCaseToDir(tempDir, "test-case-2", "another question", "orders", expectedQuery)
	require.NoError(t, err)

	// Load both
	cases, err = LoadGoldenCasesFromDir(tempDir)
	require.NoError(t, err)
	assert.Len(t, cases, 2)

	// 5. Delete first case
	err = DeleteGoldenCaseFromDir(tempDir, "test-case-1")
	require.NoError(t, err)

	// Verify deletion
	assert.NoFileExists(t, filepath.Join(tempDir, "test-case-1.json"))

	cases, err = LoadGoldenCasesFromDir(tempDir)
	require.NoError(t, err)
	assert.Len(t, cases, 1)
	assert.Equal(t, "test-case-2", cases[0].ID)

	// 6. Delete non-existent case (should return error)
	err = DeleteGoldenCaseFromDir(tempDir, "non-existent")
	assert.Error(t, err)
}

func TestGoldenLoader_RejectsUnsafeIDs(t *testing.T) {
	tempDir := t.TempDir()
	expectedQuery := query.LogicalQuery{
		Select: []query.SelectItem{{Type: "metric", Name: "row_count"}},
		Limit:  50,
	}

	unsafeIDs := []string{"", "..", "../escape", `foo\bar`, "foo/bar"}
	for _, id := range unsafeIDs {
		err := SaveGoldenCaseToDir(tempDir, id, "question", "orders", expectedQuery)
		require.Error(t, err, "save should reject id %q", id)

		err = DeleteGoldenCaseFromDir(tempDir, id)
		require.Error(t, err, "delete should reject id %q", id)
	}
}

func TestGoldenLoader_CorruptJSON(t *testing.T) {
	tempDir := t.TempDir()

	// Write invalid JSON file
	err := os.WriteFile(filepath.Join(tempDir, "corrupt.json"), []byte("{invalid-json"), 0600)
	require.NoError(t, err)

	_, err = LoadGoldenCasesFromDir(tempDir)
	assert.Error(t, err)
}
