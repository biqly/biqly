package query

import (
	"encoding/json"
	"strconv"

	"github.com/biqly/biqly/internal/semantic"
	"github.com/google/uuid"
)

// BuildQueryHistoryEntry maps an executed or failed query into a HistoryEntry for persistence.
// It sets Fingerprint from the logical query, datasource, and semantic model version.
func BuildQueryHistoryEntry(
	lq LogicalQuery,
	model *semantic.SemanticModel,
	cq *CompiledQuery,
	result *QueryResult,
	status string,
	queryErr error,
) (*HistoryEntry, error) {
	lq.EnsureVersion()
	entry := &HistoryEntry{
		DatasourceID: lq.DatasourceID,
		ModelID:      HistoryModelID(model),
		LogicalQuery: lq,
		Status:       status,
		Fingerprint: ComputeFingerprint(FingerprintInputs{
			LogicalQuery:   lq,
			DatasourceID:   lq.DatasourceID,
			ContextVersion: semanticModelVersionForFingerprint(model),
		}),
	}
	if cq != nil {
		entry.CompiledSQL = &cq.SQL
		sqlArgs, err := MarshalSQLArgs(cq.Args)
		if err != nil {
			return nil, err
		}
		entry.SQLArgs = sqlArgs
	}
	if result != nil {
		rowCount := result.Stats.RowCount
		durationMs := int(result.Stats.DurationMs)
		entry.RowCount = &rowCount
		entry.DurationMs = &durationMs
	}
	if queryErr != nil {
		msg := queryErr.Error()
		entry.ErrorMessage = &msg
	}
	return entry, nil
}

// HistoryModelID returns the model ID for storage when it is a valid UUID; otherwise nil.
// Non-UUID identifiers (e.g. transient names) are omitted so history rows stay FK-safe.
func HistoryModelID(model *semantic.SemanticModel) *string {
	if model == nil {
		return nil
	}
	if _, err := uuid.Parse(model.ID); err != nil {
		return nil
	}
	return &model.ID
}

// MarshalSQLArgs JSON-encodes SQL arguments for history storage.
func MarshalSQLArgs(args []any) (*string, error) {
	if args == nil {
		return nil, nil
	}
	b, err := json.Marshal(args)
	if err != nil {
		return nil, err
	}
	s := string(b)
	return &s, nil
}

func semanticModelVersionForFingerprint(model *semantic.SemanticModel) string {
	if model == nil {
		return ""
	}
	return strconv.Itoa(model.Version)
}
