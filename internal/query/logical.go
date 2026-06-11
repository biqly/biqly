package query

//revive:disable:exported // alias shim — canonical docs live in pkg/logicalquery

import "github.com/biqly/biqly/pkg/logicalquery"

// LogicalQuery and the sibling aliases below let legacy callers keep using
// the `query.<Type>` import path while the canonical definitions live in
// pkg/logicalquery. Methods declared on the underlying types
// (LogicalQuery.EnsureVersion, LogicalQuery.EnsureGroupBySelected,
// CTE.Subquery) remain accessible through the aliases.
type (
	LogicalQuery   = logicalquery.LogicalQuery
	SubqueryBody   = logicalquery.SubqueryBody
	CTE            = logicalquery.CTE
	SelectItem     = logicalquery.SelectItem
	WindowSpec     = logicalquery.WindowSpec
	Filter         = logicalquery.Filter
	SubqueryFilter = logicalquery.SubqueryFilter
	CaseExpr       = logicalquery.CaseExpr
	CaseBranch     = logicalquery.CaseBranch
	CaseThen       = logicalquery.CaseThen
	GroupBy        = logicalquery.GroupBy
	OrderBy        = logicalquery.OrderBy
)

// CurrentLogicalQueryVersion is re-exported from pkg/logicalquery. Bumping
// requires a coordinated change there plus a migration plan for stored
// payloads.
const CurrentLogicalQueryVersion = logicalquery.CurrentLogicalQueryVersion

// Re-exported time grain identifiers.
const (
	TimeGrainDay     = logicalquery.TimeGrainDay
	TimeGrainWeek    = logicalquery.TimeGrainWeek
	TimeGrainMonth   = logicalquery.TimeGrainMonth
	TimeGrainQuarter = logicalquery.TimeGrainQuarter
	TimeGrainYear    = logicalquery.TimeGrainYear
)

// Re-exported filter operator identifiers.
const (
	OpEq         = logicalquery.OpEq
	OpNeq        = logicalquery.OpNeq
	OpGt         = logicalquery.OpGt
	OpGte        = logicalquery.OpGte
	OpLt         = logicalquery.OpLt
	OpLte        = logicalquery.OpLte
	OpIn         = logicalquery.OpIn
	OpNotIn      = logicalquery.OpNotIn
	OpContains   = logicalquery.OpContains
	OpStartsWith = logicalquery.OpStartsWith
	OpEndsWith   = logicalquery.OpEndsWith
	OpBetween    = logicalquery.OpBetween
	OpIsNull     = logicalquery.OpIsNull
	OpIsNotNull  = logicalquery.OpIsNotNull
	OpIsEmpty    = logicalquery.OpIsEmpty
	OpIsNotEmpty = logicalquery.OpIsNotEmpty
)

// Re-exported SELECT item kinds.
const (
	SelectTypeDimension = logicalquery.SelectTypeDimension
	SelectTypeMetric    = logicalquery.SelectTypeMetric
	SelectTypeWindow    = logicalquery.SelectTypeWindow
	SelectTypeCase      = logicalquery.SelectTypeCase
)

// Re-exported CASE-branch kinds.
const (
	CaseThenTypeDimension = logicalquery.CaseThenTypeDimension
	CaseThenTypeLiteral   = logicalquery.CaseThenTypeLiteral
)

// Re-exported ORDER directions.
const (
	OrderAsc  = logicalquery.OrderAsc
	OrderDesc = logicalquery.OrderDesc
)

// IsValidTimeGrain reports whether the supplied value matches a supported
// grain. Empty is considered valid (means "no bucketing").
func IsValidTimeGrain(grain string) bool {
	return logicalquery.IsValidTimeGrain(grain)
}
