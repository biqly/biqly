package query

import (
	"fmt"
	"strings"

	"github.com/biqly/biqly/internal/semantic"
)

// Join data type groups. Two columns are SQL-joinable when their raw data
// types normalize into the same group, or into one of the explicitly
// compatible cross-group pairs (integer<->decimal, date<->timestamp).
// This mirrors the frontend groups in frontend/src/utils/joinCompatibility.ts.
const (
	joinTypeGroupInteger   = "integer"
	joinTypeGroupDecimal   = "decimal"
	joinTypeGroupText      = "text"
	joinTypeGroupUUID      = "uuid"
	joinTypeGroupBoolean   = "boolean"
	joinTypeGroupTimestamp = "timestamp"
	joinTypeGroupDate      = "date"
	joinTypeGroupJSON      = "json"
)

// joinTypeGroupByRawType maps a normalized raw column type to its group.
var joinTypeGroupByRawType = map[string]string{
	// integers (incl. serial pseudo-types)
	"smallint": joinTypeGroupInteger, "int2": joinTypeGroupInteger,
	"integer": joinTypeGroupInteger, "int": joinTypeGroupInteger, "int4": joinTypeGroupInteger,
	"bigint": joinTypeGroupInteger, "int8": joinTypeGroupInteger,
	"serial": joinTypeGroupInteger, "serial4": joinTypeGroupInteger,
	"bigserial": joinTypeGroupInteger, "serial8": joinTypeGroupInteger,
	// arbitrary-precision / floating numerics
	"numeric": joinTypeGroupDecimal, "decimal": joinTypeGroupDecimal,
	"double precision": joinTypeGroupDecimal, "float": joinTypeGroupDecimal,
	"float4": joinTypeGroupDecimal, "float8": joinTypeGroupDecimal,
	"real": joinTypeGroupDecimal, "money": joinTypeGroupDecimal,
	// text-likes
	"text": joinTypeGroupText, "character varying": joinTypeGroupText,
	"varchar": joinTypeGroupText, "character": joinTypeGroupText, "char": joinTypeGroupText,
	"citext": joinTypeGroupText, "nvarchar": joinTypeGroupText, "nchar": joinTypeGroupText,
	"string": joinTypeGroupText,
	// uuid is its own group: not implicitly joinable to text
	"uuid": joinTypeGroupUUID, "uniqueidentifier": joinTypeGroupUUID,
	// booleans
	"boolean": joinTypeGroupBoolean, "bool": joinTypeGroupBoolean,
	// timestamps
	"timestamp": joinTypeGroupTimestamp, "timestamp without time zone": joinTypeGroupTimestamp,
	"timestamp with time zone": joinTypeGroupTimestamp, "timestamptz": joinTypeGroupTimestamp,
	"datetime": joinTypeGroupTimestamp,
	// dates
	"date": joinTypeGroupDate,
	// json is only joinable to itself
	"json": joinTypeGroupJSON, "jsonb": joinTypeGroupJSON,
}

// NormalizeJoinDataType maps a raw column data type (as synced into the
// metadata catalog) to its join compatibility group. It returns "" when the
// type is not recognized, which callers must treat as "unknown, fail open".
func NormalizeJoinDataType(dataType string) string {
	t := strings.ToLower(dataType)
	if idx := strings.IndexByte(t, '('); idx >= 0 {
		if end := strings.IndexByte(t[idx:], ')'); end >= 0 {
			t = t[:idx] + t[idx+end+1:]
		}
	}
	t = strings.Join(strings.Fields(t), " ")
	return joinTypeGroupByRawType[t]
}

// JoinDataTypesCompatible reports whether two raw column data types are
// SQL-joinable under the compatibility groups above. Unknown types fail open
// (compatible) so exotic dialect types never block a query.
func JoinDataTypesCompatible(left, right string) bool {
	l, r := NormalizeJoinDataType(left), NormalizeJoinDataType(right)
	if l == "" || r == "" {
		return true
	}
	if l == r {
		return true
	}
	switch {
	case l == joinTypeGroupInteger && r == joinTypeGroupDecimal,
		l == joinTypeGroupDecimal && r == joinTypeGroupInteger,
		l == joinTypeGroupDate && r == joinTypeGroupTimestamp,
		l == joinTypeGroupTimestamp && r == joinTypeGroupDate:
		return true
	}
	return false
}

// ColumnTypeLookup resolves the raw data type of schema.table.column.
// ok=false means the type is unknown (unsynced table, missing column) and the
// join must be allowed (fail open).
type ColumnTypeLookup func(schema, table, column string) (string, bool)

// ValidateJoinColumnTypes checks every join in the model whose ON columns both
// resolve to a known data type and rejects pairs that are not SQL-joinable
// (e.g. date = uuid). Joins with incomplete column info or unknown types are
// skipped.
func ValidateJoinColumnTypes(model *semantic.SemanticModel, typeFor ColumnTypeLookup) ValidationErrors {
	if model == nil || typeFor == nil {
		return nil
	}
	var errs ValidationErrors
	for _, j := range model.Joins {
		if j.FromTable == "" || j.FromColumn == "" || j.ToTable == "" || j.ToColumn == "" {
			continue
		}
		fromSchema := j.FromSchema
		if fromSchema == "" {
			fromSchema = model.BaseSchema
		}
		toSchema := j.ToSchema
		if toSchema == "" {
			toSchema = model.BaseSchema
		}
		fromType, fromOK := typeFor(fromSchema, j.FromTable, j.FromColumn)
		toType, toOK := typeFor(toSchema, j.ToTable, j.ToColumn)
		if !fromOK || !toOK {
			continue
		}
		if !JoinDataTypesCompatible(fromType, toType) {
			errs = append(errs, &ValidationError{
				Field: "model.joins",
				Code:  "INCOMPATIBLE_JOIN_COLUMN_TYPES",
				Message: fmt.Sprintf(
					"join column types are not compatible: %s.%s (%s) cannot be joined to %s.%s (%s)",
					j.FromTable, j.FromColumn, fromType, j.ToTable, j.ToColumn, toType,
				),
				Value: j.Name,
			})
		}
	}
	return errs
}
