package query

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"sort"
)

// FingerprintInputs collects the pieces that uniquely identify a query run
// for audit grouping, cache lookup, and replay/eval. ContextVersion ties the
// fingerprint to the semantic model snapshot that produced the result so that
// publishing a new version naturally invalidates cached fingerprints.
// PermissionScope distinguishes runs of the same LogicalQuery under different
// row filters or allowed-model sets.
type FingerprintInputs struct {
	LogicalQuery    *LogicalQuery
	DatasourceID    string
	ContextVersion  string
	PermissionScope string
}

// ComputeFingerprint produces a stable hex SHA-256 over a canonical projection
// of the inputs. The projection:
//   - drops LogicalQuery.Version (a fingerprint must be stable across schema
//     bumps that don't change semantics; bumps that DO change semantics live
//     in CurrentLogicalQueryVersion and force callers to rehash anyway);
//   - sorts Filters/GroupBy/Having entries where order is semantically
//     irrelevant, so callers that build the same query in a different order
//     still collide;
//   - leaves Select/OrderBy in caller order because position carries meaning.
//
// For composite models the fingerprint also pins CompositeID; ContextVersion
// is expected to encode the resolved composite snapshot version (which in turn
// rolls forward whenever any component model is republished), so a component
// change naturally invalidates cached composite fingerprints.
func ComputeFingerprint(in FingerprintInputs) string {
	type canonical struct {
		DatasourceID    string       `json:"datasource_id"`
		ModelID         string       `json:"model_id"`
		CompositeID     string       `json:"composite_id,omitempty"`
		ContextVersion  string       `json:"context_version,omitempty"`
		PermissionScope string       `json:"permission_scope,omitempty"`
		Select          []SelectItem `json:"select"`
		Filters         []Filter     `json:"filters"`
		Having          []Filter     `json:"having"`
		GroupBy         []GroupBy    `json:"group_by"`
		OrderBy         []OrderBy    `json:"order_by"`
		Limit           int          `json:"limit"`
		Offset          int          `json:"offset"`
		CTEs            []CTE        `json:"ctes,omitempty"`
	}

	c := canonical{
		DatasourceID:    in.DatasourceID,
		ModelID:         in.LogicalQuery.ModelID,
		CompositeID:     in.LogicalQuery.CompositeID,
		ContextVersion:  in.ContextVersion,
		PermissionScope: in.PermissionScope,
		Select:          in.LogicalQuery.Select,
		Filters:         sortedFilters(in.LogicalQuery.Filters),
		Having:          sortedFilters(in.LogicalQuery.Having),
		GroupBy:         sortedGroupBy(in.LogicalQuery.GroupBy),
		OrderBy:         in.LogicalQuery.OrderBy,
		Limit:           in.LogicalQuery.Limit,
		Offset:          in.LogicalQuery.Offset,
		CTEs:            in.LogicalQuery.CTEs,
	}

	raw, err := json.Marshal(c)
	if err != nil {
		return ""
	}
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}

func sortedFilters(in []Filter) []Filter {
	if len(in) == 0 {
		return nil
	}
	out := make([]Filter, len(in))
	copy(out, in)
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Field != out[j].Field {
			return out[i].Field < out[j].Field
		}
		return out[i].Operator < out[j].Operator
	})
	return out
}

func sortedGroupBy(in []GroupBy) []GroupBy {
	if len(in) == 0 {
		return nil
	}
	out := make([]GroupBy, len(in))
	copy(out, in)
	sort.SliceStable(out, func(i, j int) bool { return out[i].Field < out[j].Field })
	return out
}
