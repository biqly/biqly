// Package query provides LogicalQuery types, SQL compilation, and execution.
package query

//revive:disable:exported // alias shim — canonical docs live in pkg/query

import pkgquery "github.com/biqly/biqly/pkg/query"

// Result and the sibling aliases below re-export pkg/query data structures
// so existing callers continue to import the legacy "internal/query" path.
// Methods declared on the canonical types in pkg/query
// (ValidationError.Error, ValidationErrors.Error) remain reachable through
// the aliases.
type (
	Result           = pkgquery.Result
	ResultColumn     = pkgquery.ResultColumn
	PivotHint        = pkgquery.PivotHint
	Anomaly          = pkgquery.Anomaly
	Stats            = pkgquery.Stats
	CompiledQuery    = pkgquery.CompiledQuery
	HistoryEntry     = pkgquery.HistoryEntry
	ValidationError  = pkgquery.ValidationError
	ValidationErrors = pkgquery.ValidationErrors //nolint:errname // re-export of pkg/query.ValidationErrors
)

// QueryResult is an alias for backward compatibility.
//
// Deprecated: Use Result instead.
type QueryResult = pkgquery.Result

// QueryStats is an alias for backward compatibility.
//
// Deprecated: Use Stats instead.
type QueryStats = pkgquery.Stats

// QueryHistoryEntry is an alias for backward compatibility.
//
// Deprecated: Use HistoryEntry instead.
type QueryHistoryEntry = pkgquery.HistoryEntry

// Re-exported semantic-type identifiers.
const (
	SemanticTypeDimension = pkgquery.SemanticTypeDimension
	SemanticTypeMetric    = pkgquery.SemanticTypeMetric
)

// Re-exported result-column format identifiers.
const (
	FormatNumber   = pkgquery.FormatNumber
	FormatCurrency = pkgquery.FormatCurrency
	FormatPercent  = pkgquery.FormatPercent
	FormatDate     = pkgquery.FormatDate
	FormatDateTime = pkgquery.FormatDateTime
	FormatText     = pkgquery.FormatText
)

// Re-exported chart suggestion identifiers.
const (
	ChartBar    = pkgquery.ChartBar
	ChartLine   = pkgquery.ChartLine
	ChartTable  = pkgquery.ChartTable
	ChartNumber = pkgquery.ChartNumber
	ChartPie    = pkgquery.ChartPie
)
