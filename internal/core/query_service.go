// Package core orchestrates semantic query compilation, execution, and permission enforcement.
package core

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"regexp"
	"strings"

	"github.com/biqly/biqly/internal/audit"
	"github.com/biqly/biqly/internal/datasource"
	"github.com/biqly/biqly/internal/metadata"
	"github.com/biqly/biqly/internal/platform/observability"
	"github.com/biqly/biqly/internal/query"
	"github.com/biqly/biqly/internal/security"
	"github.com/biqly/biqly/internal/semantic"
	pkgmetadata "github.com/biqly/biqly/pkg/metadata"
)

const (
	QueryStatusSuccess = "success"
	QueryStatusFailed  = "failed"
)

type ModelLoader interface {
	GetPublishedFullModel(ctx context.Context, modelID string) (*semantic.SemanticModel, error)
}

// CompositeModelLoader resolves a composite semantic model into the merged
// SemanticModel that the compiler consumes. Optional dependency: when nil,
// LogicalQueries referencing composite_id are rejected.
type CompositeModelLoader interface {
	GetPublishedResolvedComposite(ctx context.Context, compositeID string) (*semantic.SemanticModel, error)
}

type DatasourceLoader interface {
	GetDatasource(ctx context.Context, datasourceID string) (*metadata.Datasource, error)
}

type DriverGetter interface {
	Get(typeName string) (datasource.Driver, error)
}

type HistoryRecorder interface {
	CreateQueryHistory(ctx context.Context, entry *query.HistoryEntry) error
}

// ColumnTypeLoader resolves synced column metadata so inline (ad-hoc) model
// joins can be checked for SQL type compatibility. Optional: nil disables the
// check.
type ColumnTypeLoader interface {
	ListColumns(ctx context.Context, datasourceID, schemaName, tableName string) ([]metadata.Column, error)
}

type QueryServiceDeps struct {
	Models      ModelLoader
	Composites  CompositeModelLoader
	Datasources DatasourceLoader
	Drivers     DriverGetter
	Validator   *query.Validator
	Executor    *query.Executor
	History     HistoryRecorder
	Logger      *slog.Logger
	// Encryptor decrypts datasource DSNs when stored encrypted; nil means plaintext only.
	Encryptor *security.Encryption
	// Pools caches *sql.DB handles across query executions. When nil the
	// service falls back to opening a fresh pool per query (legacy behavior).
	Pools *datasource.PoolCache
	// PIIPolicies resolves per-user PII masking configs and RLS row filters.
	// Nil disables both.
	PIIPolicies *PIIPolicyService
	// Audit records query execution events with the applied policy decisions.
	// Nil disables audit.
	Audit *audit.Logger
	// Identity resolves the calling user for history and audit attribution.
	Identity IdentityResolver
	// ColumnTypes resolves synced column data types to validate inline model
	// join column compatibility. Nil disables the check (fail open).
	ColumnTypes ColumnTypeLoader
}

type QueryService struct {
	models      ModelLoader
	composites  CompositeModelLoader
	datasources DatasourceLoader
	drivers     DriverGetter
	validator   *query.Validator
	executor    *query.Executor
	history     HistoryRecorder
	logger      *slog.Logger
	encryptor   *security.Encryption
	pools       *datasource.PoolCache
	piiPolicies *PIIPolicyService
	audit       *audit.Logger
	identity    IdentityResolver
	columnTypes ColumnTypeLoader
}

type CompileResult struct {
	LogicalQuery query.LogicalQuery      `json:"logical_query"`
	Model        *semantic.SemanticModel `json:"semantic_model"`
	Datasource   *metadata.Datasource    `json:"-"`
	Driver       datasource.Driver       `json:"-"`
	Compiled     *query.CompiledQuery    `json:"compiled_query"`
}

type RunResult struct {
	CompileResult
	Result *query.Result `json:"result,omitempty"`
}

func NewQueryService(deps *QueryServiceDeps) *QueryService {
	return &QueryService{
		models:      deps.Models,
		composites:  deps.Composites,
		datasources: deps.Datasources,
		drivers:     deps.Drivers,
		validator:   deps.Validator,
		executor:    deps.Executor,
		history:     deps.History,
		logger:      deps.Logger,
		encryptor:   deps.Encryptor,
		pools:       deps.Pools,
		piiPolicies: deps.PIIPolicies,
		audit:       deps.Audit,
		identity:    deps.Identity,
		columnTypes: deps.ColumnTypes,
	}
}

// openPool returns a *sql.DB to use for executing compiled. When a PoolCache
// is configured the handle is cached and owned by the cache (do NOT close it
// after use). Otherwise a fresh pool is opened and the returned cleanup func
// closes it.
func (s *QueryService) openPool(ctx context.Context, driver datasource.Driver, datasourceID, dsn string) (*sql.DB, func(), error) {
	if s.pools != nil {
		db, err := s.pools.Get(ctx, driver, datasourceID, dsn)
		if err != nil {
			return nil, nil, err
		}
		return db, func() {}, nil
	}
	db, err := driver.Open(ctx, dsn)
	if err != nil {
		return nil, nil, err
	}
	return db, func() { _ = db.Close() }, nil
}

func (s *QueryService) Compile(ctx context.Context, lq *query.LogicalQuery) (*CompileResult, *ServiceError) {
	return s.CompileWithModel(ctx, lq, nil)
}

// CompileWithModel is Compile with an optional inline semantic model. When
// inline is non-nil it is used directly instead of loading the model
// referenced by lq.ModelID/lq.CompositeID from the catalog — required for
// synthetic auto-routing models ("auto:" ID prefix) that exist only in the
// caller's memory.
func (s *QueryService) CompileWithModel(ctx context.Context, lq *query.LogicalQuery, inline *semantic.SemanticModel) (*CompileResult, *ServiceError) {
	lq.EnsureVersion()
	loaded, se := s.loadContext(ctx, lq, inline)
	if se != nil {
		return nil, se
	}
	// Repair + EnsureGroupBySelected used to run here AND inside
	// CompileWithContext, which double-walked the LogicalQuery for every
	// /api/query/run. CompileWithContext is the single normalization point.
	compiled, se := s.CompileWithContext(ctx, &loaded.LogicalQuery, loaded.Model, loaded.Driver)
	if se != nil {
		return nil, se
	}
	loaded.Compiled = compiled
	if se := enforceFunctionBlocklist(loaded); se != nil {
		return nil, se
	}
	return loaded, nil
}

func enforceFunctionBlocklist(compiled *CompileResult) *ServiceError {
	custom, err := pkgmetadata.ParseDatasourceFunctionBlocklist(compiled.Datasource.Config)
	if err != nil {
		return ToServiceError(fmt.Errorf("parse datasource function blocklist: %w", err))
	}
	checker, err := security.NewReadOnlyCheckerWithAdditionalDeniedFunctions(custom)
	if err != nil {
		return ToServiceError(fmt.Errorf("invalid datasource function blocklist: %w", err))
	}
	if err := checker.Check(compiled.Compiled.SQL); err != nil {
		return ToServiceError(fmt.Errorf("function blocklist: %w", err))
	}
	return nil
}

func (s *QueryService) CompileWithContext(ctx context.Context, lq *query.LogicalQuery, model *semantic.SemanticModel, driver datasource.Driver) (*query.CompiledQuery, *ServiceError) {
	if driver != nil {
		ctx = observability.WithDBSystem(ctx, driver.Type())
	}
	query.RepairMisnamedCalendarGrainDimensions(lq, dimensionNames(model))
	lq.EnsureGroupBySelected()
	if err := s.validator.Validate(lq, model); err != nil {
		return nil, ToServiceError(err)
	}
	// Per-user PII masking + RLS row filters; errors fail the query rather
	// than run unmasked/unfiltered.
	piiConfig, rowFilters, err := s.piiPolicies.QueryPolicy(ctx, lq.DatasourceID)
	if err != nil {
		return nil, ToServiceError(fmt.Errorf("resolve security policy: %w", err))
	}
	compiled, err := query.NewCompiler(driver.Dialect()).CompileWithPermissions(ctx, lq, model, rowFilters, piiConfig)
	if err != nil {
		if _, ok := errors.AsType[query.ValidationErrors](err); ok {
			return nil, ToServiceError(err)
		}
		return nil, ToServiceError(fmt.Errorf("compile: %w", err))
	}
	return compiled, nil
}

func (s *QueryService) Run(ctx context.Context, lq *query.LogicalQuery) (*RunResult, *ServiceError) {
	return s.RunWithModel(ctx, lq, nil)
}

// RunWithModel is Run with an optional inline semantic model; see
// CompileWithModel for when to pass a non-nil model.
func (s *QueryService) RunWithModel(ctx context.Context, lq *query.LogicalQuery, inline *semantic.SemanticModel) (*RunResult, *ServiceError) {
	lq.EnsureVersion()
	compiled, se := s.CompileWithModel(ctx, lq, inline)
	if se != nil {
		return nil, se
	}
	dsn, err := metadata.RuntimeDSN(compiled.Datasource, s.encryptor)
	if err != nil {
		return nil, ToServiceError(fmt.Errorf("%w: %w", ErrLoadDatasource, err))
	}
	db, cleanup, err := s.openPool(ctx, compiled.Driver, compiled.Datasource.ID, dsn)
	if err != nil {
		return nil, ToServiceError(fmt.Errorf("%w: %w", ErrConnection, err))
	}
	defer cleanup()

	if fp, fpErr := query.LogicalQueryFingerprint(&compiled.LogicalQuery, compiled.Model); fpErr == nil {
		ctx = observability.WithQueryFingerprint(ctx, fp)
	}
	if compiled.Driver != nil {
		ctx = observability.WithDBSystem(ctx, compiled.Driver.Type())
	}
	readOnlyTx := compiled.Driver != nil && compiled.Driver.SupportsReadOnlyTx()
	result, err := s.executor.Execute(ctx, db, compiled.Compiled, readOnlyTx)
	if err != nil {
		entry := s.recordHistory(ctx, &compiled.LogicalQuery, compiled.Model, compiled.Compiled, nil, QueryStatusFailed, err)
		s.auditQueryExecution(ctx, compiled, nil, entry, err)
		return nil, ToServiceError(fmt.Errorf("%w: %w", ErrQueryExecution, err))
	}
	// Compute total count for pagination when the query has a LIMIT.
	if compiled.Compiled != nil {
		count, countErr := computeTotalCount(ctx, db, compiled.Compiled)
		if countErr == nil {
			result.Stats.TotalCount = count
		} else {
			slog.Debug("total count query skipped", "error", countErr)
		}
	}
	query.EnrichResult(result, lq, compiled.Model)
	entry := s.recordHistory(ctx, &compiled.LogicalQuery, compiled.Model, compiled.Compiled, result, QueryStatusSuccess, nil)
	s.auditQueryExecution(ctx, compiled, result, entry, nil)
	return &RunResult{CompileResult: *compiled, Result: result}, nil
}

// DryRunWithModel compiles a LogicalQuery and validates its generated SQL
// against the target datasource's EXPLAIN facility without executing it.
func (s *QueryService) DryRunWithModel(ctx context.Context, lq *query.LogicalQuery, inline *semantic.SemanticModel) (*CompileResult, *ServiceError) {
	compiled, se := s.CompileWithModel(ctx, lq, inline)
	if se != nil {
		return nil, se
	}
	explain, se := dryRunExplainSQL(compiled.Compiled, compiled.Driver)
	if se != nil || explain == "" {
		return compiled, se
	}
	dsn, err := metadata.RuntimeDSN(compiled.Datasource, s.encryptor)
	if err != nil {
		return nil, ToServiceError(fmt.Errorf("%w: %w", ErrLoadDatasource, err))
	}
	db, cleanup, err := s.openPool(ctx, compiled.Driver, compiled.Datasource.ID, dsn)
	if err != nil {
		return nil, ToServiceError(fmt.Errorf("%w: %w", ErrConnection, err))
	}
	defer cleanup()

	if se := runDryExplain(ctx, db, explain, compiled.Compiled.Args); se != nil {
		return nil, se
	}
	return compiled, nil
}

// DryRun validates a LogicalQuery against a caller-provided datasource
// connection. It is retained for AI validation paths that already own a
// connection and have resolved the semantic model and driver.
func (s *QueryService) DryRun(ctx context.Context, db *sql.DB, lq *query.LogicalQuery, model *semantic.SemanticModel, driver datasource.Driver) *ServiceError {
	compiled, se := s.CompileWithContext(ctx, lq, model, driver)
	if se != nil {
		return se
	}
	explain, se := dryRunExplainSQL(compiled, driver)
	if se != nil || explain == "" {
		return se
	}
	return runDryExplain(ctx, db, explain, compiled.Args)
}

func dryRunExplainSQL(compiled *query.CompiledQuery, driver datasource.Driver) (string, *ServiceError) {
	if err := security.NewReadOnlyChecker().Check(compiled.SQL); err != nil {
		return "", ToServiceError(fmt.Errorf("read-only check: %w", err))
	}
	explain := driver.Dialect().ExplainSQL(compiled.SQL)
	if explain == "" {
		return "", nil
	}
	return explain, nil
}

func runDryExplain(ctx context.Context, db *sql.DB, explain string, args []any) *ServiceError {
	rows, err := db.QueryContext(ctx, explain, args...)
	if err != nil {
		return ToServiceError(fmt.Errorf("explain: %w", err))
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		// Drain result set so the driver can reuse the connection.
	}
	if err := rows.Err(); err != nil {
		return ToServiceError(fmt.Errorf("explain: %w", err))
	}
	return nil
}

func dimensionNames(model *semantic.SemanticModel) []string {
	if model == nil {
		return nil
	}
	out := make([]string, 0, len(model.Dimensions))
	for i := range model.Dimensions {
		out = append(out, model.Dimensions[i].Name)
	}
	return out
}

func (s *QueryService) loadSemanticModel(ctx context.Context, lq *query.LogicalQuery) (*semantic.SemanticModel, *ServiceError) {
	if lq.CompositeID != "" {
		if s.composites == nil {
			return nil, ToServiceError(fmt.Errorf("%w: composite models not supported", ErrLoadSemanticModel))
		}
		model, err := s.composites.GetPublishedResolvedComposite(ctx, lq.CompositeID)
		if err != nil {
			return nil, ToServiceError(fmt.Errorf("%w: %w", ErrLoadSemanticModel, err))
		}
		return model, nil
	}
	if lq.ModelID == "" {
		return nil, ToServiceError(ErrModelIDRequired)
	}
	model, err := s.models.GetPublishedFullModel(ctx, lq.ModelID)
	if err != nil {
		return nil, ToServiceError(fmt.Errorf("%w: %w", ErrLoadSemanticModel, err))
	}
	return model, nil
}

func (s *QueryService) loadContext(ctx context.Context, lq *query.LogicalQuery, inline *semantic.SemanticModel) (*CompileResult, *ServiceError) {
	if lq.DatasourceID == "" {
		return nil, ToServiceError(ErrDatasourceIDRequired)
	}
	model := inline
	if model == nil {
		var se *ServiceError
		model, se = s.loadSemanticModel(ctx, lq)
		if se != nil {
			return nil, se
		}
	} else if se := s.validateInlineJoinColumnTypes(ctx, lq.DatasourceID, inline); se != nil {
		// Catalog-stored models are validated at modeling time; inline (ad-hoc
		// Query Builder / auto-routing) models arrive unvetted, so their join
		// ON column types are checked here against synced column metadata.
		return nil, se
	}
	ds, err := s.datasources.GetDatasource(ctx, lq.DatasourceID)
	if err != nil {
		return nil, ToServiceError(fmt.Errorf("%w: %w", ErrLoadDatasource, err))
	}
	driver, err := s.drivers.Get(ds.Type)
	if err != nil {
		return nil, ToServiceError(fmt.Errorf("%w: %w", ErrLoadDriver, err))
	}
	return &CompileResult{
		LogicalQuery: *lq,
		Model:        model,
		Datasource:   ds,
		Driver:       driver,
	}, nil
}

// validateInlineJoinColumnTypes rejects inline-model joins whose ON columns
// have SQL-incompatible types (e.g. date = uuid) using synced column metadata.
// Missing metadata, lookup errors, or unknown types fail open so exotic
// dialects and unsynced tables never block a query.
func (s *QueryService) validateInlineJoinColumnTypes(ctx context.Context, datasourceID string, model *semantic.SemanticModel) *ServiceError {
	if s.columnTypes == nil || model == nil || len(model.Joins) == 0 {
		return nil
	}
	typesByTable := make(map[string]map[string]string, len(model.Joins)+1)
	lookup := func(schema, table, column string) (string, bool) {
		if table == "" || column == "" {
			return "", false
		}
		key := schema + "." + table
		types, cached := typesByTable[key]
		if !cached {
			types = map[string]string{}
			cols, err := s.columnTypes.ListColumns(ctx, datasourceID, schema, table)
			if err != nil {
				slog.DebugContext(ctx, "join type check: column metadata lookup failed (fail open)",
					"datasource_id", datasourceID, "schema", schema, "table", table, "err", err)
			} else {
				for _, c := range cols {
					types[c.ColumnName] = c.DataType
				}
			}
			typesByTable[key] = types
		}
		t, ok := types[column]
		return t, ok && t != ""
	}
	if errs := query.ValidateJoinColumnTypes(model, lookup); len(errs) > 0 {
		return ToServiceError(errs)
	}
	return nil
}

// recordHistory persists the query history entry and returns it (with the
// generated ID) for audit linkage; nil when persistence is disabled or failed.
func (s *QueryService) recordHistory(
	ctx context.Context,
	lq *query.LogicalQuery,
	model *semantic.SemanticModel,
	cq *query.CompiledQuery,
	result *query.Result,
	status string,
	queryErr error,
) *query.HistoryEntry {
	if s.history == nil {
		return nil
	}
	entry, err := query.BuildQueryHistoryEntry(lq, model, cq, result, status, queryErr)
	if err != nil {
		s.logError(ctx, "build query history failed", err)
		return nil
	}
	if uid := s.callerID(ctx); uid != "" {
		entry.UserID = new(uid)
	}
	if err := s.history.CreateQueryHistory(ctx, entry); err != nil {
		s.logError(ctx, "create query history failed", err)
		return nil
	}
	return entry
}

func (s *QueryService) callerID(ctx context.Context) string {
	if s.identity == nil {
		return ""
	}
	uid, _ := s.identity(ctx)
	return uid
}

// auditQueryExecution records the query execution audit event carrying the
// policy decisions applied at compile time, so enforcement is provable per
// request. The write context is non-cancelable: the event must land even
// when the caller disconnects right after execution.
func (s *QueryService) auditQueryExecution(
	ctx context.Context,
	cr *CompileResult,
	result *query.Result,
	entry *query.HistoryEntry,
	queryErr error,
) {
	if s.audit == nil || cr == nil {
		return
	}
	details := map[string]any{
		"channel": audit.ChannelFromContext(ctx),
	}
	if entry != nil {
		details["history_id"] = entry.ID
		details["fingerprint"] = entry.Fingerprint
	}
	if result != nil {
		details["row_count"] = result.Stats.RowCount
		details["duration_ms"] = result.Stats.DurationMs
	}
	if cq := cr.Compiled; cq != nil && cq.Policy != nil {
		if len(cq.Policy.RowFilters) > 0 {
			details["row_filters"] = cq.Policy.RowFilters
		}
		if len(cq.Policy.MaskedColumns) > 0 {
			details["masked_columns"] = cq.Policy.MaskedColumns
		}
		if len(cq.Policy.HiddenColumns) > 0 {
			details["hidden_columns"] = cq.Policy.HiddenColumns
		}
	}
	eventType := audit.EventQueryExecuted
	if queryErr != nil {
		eventType = audit.EventQueryFailed
		details["error"] = queryErr.Error()
	}
	var modelID string
	if id := query.HistoryModelID(cr.Model); id != nil {
		modelID = *id
	}
	s.audit.Log(context.WithoutCancel(ctx), audit.Event{
		UserID:       s.callerID(ctx),
		EventType:    eventType,
		DatasourceID: cr.LogicalQuery.DatasourceID,
		ModelID:      modelID,
		Details:      details,
	})
}

func (s *QueryService) logError(ctx context.Context, msg string, err error) {
	if s.logger != nil {
		s.logger.ErrorContext(ctx, msg, "error", err)
		return
	}
	slog.ErrorContext(ctx, msg, "error", err)
}

// computeTotalCount runs a SELECT COUNT(*) variant of the compiled query,
// stripping ORDER BY, LIMIT, and OFFSET from the outermost clause so the
// result reflects the true total row count. Returns 0 if the count query
// itself fails (caller should treat it as unknown, not a hard error).
func computeTotalCount(ctx context.Context, db *sql.DB, cq *query.CompiledQuery) (int, error) {
	querySQL := stripPagination(cq.SQL)
	if querySQL == cq.SQL {
		// No LIMIT clause — no pagination needed.
		return 0, nil
	}
	// Wraps compiler output; values stay in cq.Args (parameterized).
	countSQL := "SELECT COUNT(*) FROM (" + querySQL + ") AS _cnt" // nosemgrep: go.lang.security.audit.database.string-formatted-query.string-formatted-query
	var count int
	err := db.QueryRowContext(ctx, countSQL, cq.Args...).Scan(&count) // nosemgrep: go.lang.security.audit.database.string-formatted-query.string-formatted-query
	if err != nil {
		return 0, fmt.Errorf("count query: %w", err)
	}
	return count, nil
}

// stripPagination strips the outermost ORDER BY, LIMIT, and OFFSET clauses
// from a SQL string, returning the modified SQL. If no LIMIT is found, the
// original string is returned unchanged.
func stripPagination(rawSQL string) string {
	// Normalise whitespace so the trailing-clause regexes can anchor to $.
	s := strings.TrimSpace(rawSQL)

	// Strip trailing OFFSET (may appear without LIMIT in some dialects).
	re := regexp.MustCompile(`(?i)\s+OFFSET\s+\d+$`)
	s = re.ReplaceAllString(s, "")

	// Strip trailing LIMIT [count] [OFFSET count].
	re = regexp.MustCompile(`(?i)\s+LIMIT\s+\d+(?:\s+OFFSET\s+\d+)?$`)
	hasLimit := s != re.ReplaceAllString(s, "")
	s = re.ReplaceAllString(s, "")

	if !hasLimit {
		return rawSQL // unchanged
	}

	// Strip trailing ORDER BY ... (safe for COUNT(*) wrapping).
	re = regexp.MustCompile(`(?i)\s+ORDER\s+BY\s+.+$`)
	s = re.ReplaceAllString(s, "")

	return s
}
