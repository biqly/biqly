package audit

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMarshal(t *testing.T) {
	now := time.Now().UTC()
	event := Event{
		ID:           "test_id",
		UserID:       "user_123",
		EventType:    EventQueryExecuted,
		DatasourceID: "ds_456",
		ModelID:      "model_789",
		Details:      map[string]any{"query": "SELECT 1"},
		Timestamp:    now,
	}

	data, err := Marshal(event)
	require.NoError(t, err)

	var decoded Event
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)

	assert.Equal(t, event.ID, decoded.ID)
	assert.Equal(t, event.UserID, decoded.UserID)
	assert.Equal(t, event.EventType, decoded.EventType)
	assert.Equal(t, event.DatasourceID, decoded.DatasourceID)
	assert.Equal(t, event.ModelID, decoded.ModelID)
	assert.Equal(t, event.Details["query"], decoded.Details["query"])
	assert.True(t, event.Timestamp.Equal(decoded.Timestamp))
}

func TestLogger(t *testing.T) {
	tests := []struct {
		name          string
		eventType     EventType
		expectedLevel string
	}{
		{
			name:          "QueryExecuted uses Info level",
			eventType:     EventQueryExecuted,
			expectedLevel: "INFO",
		},
		{
			name:          "QueryFailed uses Error level",
			eventType:     EventQueryFailed,
			expectedLevel: "ERROR",
		},
		{
			name:          "PermissionDeny uses Error level",
			eventType:     EventPermissionDeny,
			expectedLevel: "ERROR",
		},
		{
			name:          "DatasourceSync uses Info level",
			eventType:     EventDatasourceSync,
			expectedLevel: "INFO",
		},
		{
			name:          "AIGenerated uses Info level",
			eventType:     EventAIGenerated,
			expectedLevel: "INFO",
		},
		{
			name:          "InternalRequest uses Info level",
			eventType:     EventInternalRequest,
			expectedLevel: "INFO",
		},
		{
			name:          "Unknown EventType uses Info level",
			eventType:     EventType("unknown"),
			expectedLevel: "INFO",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			jsonHandler := slog.NewJSONHandler(&buf, nil)
			slogger := slog.New(jsonHandler)
			auditLogger := NewLogger(slogger)

			event := Event{
				ID:           "evt_1",
				UserID:       "usr_1",
				EventType:    tt.eventType,
				DatasourceID: "ds_1",
				ModelID:      "mdl_1",
				Details:      map[string]any{"foo": "bar"},
				Timestamp:    time.Now(),
			}

			auditLogger.Log(context.Background(), event)

			logOutput := buf.Bytes()
			require.NotEmpty(t, logOutput)

			var logMap map[string]any
			err := json.Unmarshal(logOutput, &logMap)
			require.NoError(t, err)

			assert.Equal(t, tt.expectedLevel, logMap["level"])
			assert.Equal(t, string(tt.eventType), logMap["event_type"])
			assert.Equal(t, "usr_1", logMap["user_id"])
			assert.Equal(t, "ds_1", logMap["datasource_id"])
			assert.Equal(t, "mdl_1", logMap["model_id"])

			details, ok := logMap["details"].(map[string]any)
			require.True(t, ok)
			assert.Equal(t, "bar", details["foo"])
		})
	}
}
