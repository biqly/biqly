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
	lq *LogicalQuery,
	model *semantic.SemanticModel,
	cq *CompiledQuery,
	result *Result,
	status string,
	queryErr error,
) (*HistoryEntry, error) {
	lq.EnsureVersion()
	fingerprint, err := ComputeFingerprint(FingerprintInputs{
		LogicalQuery:   lq,
		DatasourceID:   lq.DatasourceID,
		ContextVersion: semanticModelVersionForFingerprint(model),
	})
	if err != nil {
		return nil, err
	}
	entry := &HistoryEntry{
		DatasourceID: lq.DatasourceID,
		ModelID:      HistoryModelID(model),
		LogicalQuery: *lq,
		Status:       status,
		Fingerprint:  fingerprint,
	}
	if cq != nil {
		entry.CompiledSQL = new(cq.SQL)
		sqlArgs, err := MarshalSQLArgs(cq.Args)
		if err != nil {
			return nil, err
		}
		entry.SQLArgs = sqlArgs
	}
	if result != nil {
		entry.RowCount = new(result.Stats.RowCount)
		entry.DurationMs = new(int(result.Stats.DurationMs))
	}
	if queryErr != nil {
		entry.ErrorMessage = new(queryErr.Error())
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
	return new(model.ID)
}

// MarshalSQLArgs JSON-encodes SQL arguments for history storage.
func MarshalSQLArgs(args []any) (*string, error) {
	if args == nil {
		return nil, nil //nolint:nilnil // nil args serialize as SQL NULL
	}
	b, err := json.Marshal(args)
	if err != nil {
		return nil, err
	}
	return new(string(b)), nil
}

func semanticModelVersionForFingerprint(model *semantic.SemanticModel) string {
	if model == nil {
		return ""
	}
	return strconv.Itoa(model.Version)
}
