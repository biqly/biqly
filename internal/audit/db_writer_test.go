package audit

import (
	"context"
	"database/sql"
	"encoding/json"
	"testing"
	"time"

	"github.com/biqly/biqly/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestToNullUUID(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected sql.NullString
	}{
		{
			name:     "Valid UUID",
			input:    "4da3108c-02a8-4eb8-b9a5-1d0b30177724",
			expected: sql.NullString{String: "4da3108c-02a8-4eb8-b9a5-1d0b30177724", Valid: true},
		},
		{
			name:     "Invalid UUID length",
			input:    "short-uuid",
			expected: sql.NullString{Valid: false},
		},
		{
			name:     "Invalid UUID format",
			input:    "4da3108c-02a8-4eb8-b9a5-1d0b3017772g", // contains 'g'
			expected: sql.NullString{Valid: false},
		},
		{
			name:     "Empty string",
			input:    "",
			expected: sql.NullString{Valid: false},
		},
		{
			name:     "Unknown string caller",
			input:    "unknown",
			expected: sql.NullString{Valid: false},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := toNullUUID(tt.input)
			assert.Equal(t, tt.expected.Valid, got.Valid)
			if tt.expected.Valid {
				assert.Equal(t, tt.expected.String, got.String)
			}
		})
	}
}

func TestNewDBWriter_NilDB(t *testing.T) {
	w := NewDBWriter(context.Background(), nil, nil)
	assert.Nil(t, w)
}

func openTestDBPool(t *testing.T) *sql.DB {
	t.Helper()
	return testutil.OpenMetadataDB(t)
}

func TestDBWriter_Integration(t *testing.T) {
	db := openTestDBPool(t)
	ctx := context.Background()

	// Ensure clean start
	_, err := db.ExecContext(ctx, "DELETE FROM audit_events WHERE event_type = $1", "test_db_writer_integration")
	require.NoError(t, err)

	w := NewDBWriter(ctx, db, nil)
	require.NotNil(t, w)
	t.Cleanup(func() { require.NoError(t, w.Close()) })

	event := Event{
		UserID:       "4da3108c-02a8-4eb8-b9a5-1d0b30177724",
		EventType:    "test_db_writer_integration",
		DatasourceID: "5fa3108c-02a8-4eb8-b9a5-1d0b30177725",
		ModelID:      "6fa3108c-02a8-4eb8-b9a5-1d0b30177726",
		Details:      map[string]any{"key": "val"},
		Timestamp:    time.Now().UTC(),
	}

	w.Write(event)

	// Wait for the background worker to write
	time.Sleep(200 * time.Millisecond)

	var count int
	err = db.QueryRowContext(ctx, "SELECT COUNT(*) FROM audit_events WHERE event_type = $1", "test_db_writer_integration").Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, 1, count)

	var userID, datasourceID, modelID string
	var detailsStr string
	err = db.QueryRowContext(ctx, `
		SELECT user_id, datasource_id, model_id, details::text 
		FROM audit_events 
		WHERE event_type = $1 
		LIMIT 1
	`, "test_db_writer_integration").Scan(&userID, &datasourceID, &modelID, &detailsStr)
	require.NoError(t, err)

	assert.Equal(t, event.UserID, userID)
	assert.Equal(t, event.DatasourceID, datasourceID)
	assert.Equal(t, event.ModelID, modelID)

	var detailsMap map[string]any
	err = json.Unmarshal([]byte(detailsStr), &detailsMap)
	require.NoError(t, err)
	assert.Equal(t, "val", detailsMap["key"])
}

func TestDBWriter_CloseFlushes(t *testing.T) {
	db := openTestDBPool(t)
	ctx := context.Background()

	// Ensure clean start
	_, err := db.ExecContext(ctx, "DELETE FROM audit_events WHERE event_type = $1", "test_close_flushes")
	require.NoError(t, err)

	w := NewDBWriter(ctx, db, nil)
	require.NotNil(t, w)

	event := Event{
		UserID:    "4da3108c-02a8-4eb8-b9a5-1d0b30177724",
		EventType: "test_close_flushes",
		Timestamp: time.Now().UTC(),
	}

	w.Write(event)

	// Close immediately, which should trigger a flush of any remaining events in the channel
	err = w.Close()
	require.NoError(t, err)

	var count int
	err = db.QueryRowContext(ctx, "SELECT COUNT(*) FROM audit_events WHERE event_type = $1", "test_close_flushes").Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, 1, count)
}
