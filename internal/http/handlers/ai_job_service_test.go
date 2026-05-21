package handlers

import (
	"encoding/json"
	"testing"

	"github.com/biqly/biqly/internal/ai"
	"github.com/stretchr/testify/require"
)

func TestValidateAIJobRequestDescribe(t *testing.T) {
	valid, err := json.Marshal(ai.DescribeRequest{
		DatasourceID: "ds_1",
		Schema:       "public",
		Table:        "users",
	})
	require.NoError(t, err)
	require.NoError(t, validateAIJobRequest("describe", valid))

	missingTable, err := json.Marshal(ai.DescribeRequest{DatasourceID: "ds_1"})
	require.NoError(t, err)
	require.EqualError(t, validateAIJobRequest("describe", missingTable), "datasource_id and table are required")
}

func TestValidateAIJobRequestRejectsUnknownKind(t *testing.T) {
	err := validateAIJobRequest("unknown", json.RawMessage(`{}`))
	require.EqualError(t, err, "invalid kind")
}
