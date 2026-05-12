package core

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"strconv"

	"github.com/biqly/biqly/internal/datasource"
	"github.com/biqly/biqly/internal/metadata"
	"github.com/biqly/biqly/internal/query"
	"github.com/biqly/biqly/internal/semantic"
)

const (
	QueryStatusSuccess = "success"
	QueryStatusFailed  = "failed"
)

type ModelLoader interface {
	GetPublishedFullModel(ctx context.Context, modelID string) (*semantic.SemanticModel, error)
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

type QueryServiceDeps struct {
	Models      ModelLoader
	Datasources DatasourceLoader
	Drivers     DriverGetter
	Validator   *query.Validator
	Executor    *query.Executor
	History     HistoryRecorder
	Logger      *slog.Logger
}

type QueryService struct {
	models      ModelLoader
	datasources DatasourceLoader
	drivers     DriverGetter
	validator   *query.Validator
	executor    *query.Executor
	history     HistoryRecorder
	logger      *slog.Logger
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
	Result *query.QueryResult `json:"result,omitempty"`
}

func NewQueryService(deps QueryServiceDeps) *QueryService {
	return &QueryService{
		models:      deps.Models,
		datasources: deps.Datasources,
		drivers:     deps.Drivers,
		validator:   deps.Validator,
		executor:    deps.Executor,
		history:     deps.History,
		logger:      deps.Logger,
	}
}

func (s *QueryService) Compile(ctx context.Context, lq query.LogicalQuery) (*CompileResult, error) {
	lq.EnsureVersion()
	loaded, err := s.loadContext(ctx, lq)
	if err != nil {
		return nil, err
	}
	compiled, err := s.CompileWithContext(ctx, lq, loaded.Model, loaded.Driver)
	if err != nil {
		return nil, err
	}
	loaded.Compiled = compiled
	return loaded, nil
}

func (s *QueryService) CompileWithContext(ctx context.Context, lq query.LogicalQuery, model *semantic.SemanticModel, driver datasource.Driver) (*query.CompiledQuery, error) {
	if err := s.validator.Validate(lq, model); err != nil {
		return nil, err
	}
	compiled, err := query.NewCompiler(driver.Dialect()).Compile(ctx, lq, model)
	if err != nil {
		return nil, fmt.Errorf("compile: %w", err)
	}
	return compiled, nil
}

func (s *QueryService) Run(ctx context.Context, lq query.LogicalQuery) (*RunResult, error) {
	lq.EnsureVersion()
	compiled, err := s.Compile(ctx, lq)
	if err != nil {
		return nil, err
	}
	db, err := compiled.Driver.Open(ctx, compiled.Datasource.DSNEncrypted)
	if err != nil {
		return nil, fmt.Errorf("connection: %w", err)
	}
	defer func() { _ = db.Close() }()

	result, err := s.executor.Execute(ctx, db, compiled.Compiled)
	if err != nil {
		s.recordHistory(ctx, lq, compiled.Model, compiled.Compiled, nil, QueryStatusFailed, err)
		return nil, fmt.Errorf("execute: %w", err)
	}
	query.EnrichResult(result, lq, compiled.Model)
	s.recordHistory(ctx, lq, compiled.Model, compiled.Compiled, result, QueryStatusSuccess, nil)
	return &RunResult{CompileResult: *compiled, Result: result}, nil
}

func (s *QueryService) DryRun(ctx context.Context, db *sql.DB, lq query.LogicalQuery, model *semantic.SemanticModel, driver datasource.Driver) error {
	compiled, err := s.CompileWithContext(ctx, lq, model, driver)
	if err != nil {
		return fmt.Errorf("compile: %w", err)
	}
	explain := driver.Dialect().ExplainSQL(compiled.SQL)
	if explain == "" {
		return nil
	}
	rows, err := db.QueryContext(ctx, explain, compiled.Args...)
	if err != nil {
		return fmt.Errorf("explain: %w", err)
	}
	_ = rows.Close()
	return nil
}

func (s *QueryService) loadContext(ctx context.Context, lq query.LogicalQuery) (*CompileResult, error) {
	if lq.ModelID == "" {
		return nil, fmt.Errorf("model_id is required")
	}
	if lq.DatasourceID == "" {
		return nil, fmt.Errorf("datasource_id is required")
	}
	model, err := s.models.GetPublishedFullModel(ctx, lq.ModelID)
	if err != nil {
		return nil, fmt.Errorf("load semantic model: %w", err)
	}
	ds, err := s.datasources.GetDatasource(ctx, lq.DatasourceID)
	if err != nil {
		return nil, fmt.Errorf("load datasource: %w", err)
	}
	driver, err := s.drivers.Get(ds.Type)
	if err != nil {
		return nil, fmt.Errorf("load driver: %w", err)
	}
	return &CompileResult{
		LogicalQuery: lq,
		Model:        model,
		Datasource:   ds,
		Driver:       driver,
	}, nil
}

func (s *QueryService) recordHistory(
	ctx context.Context,
	lq query.LogicalQuery,
	model *semantic.SemanticModel,
	cq *query.CompiledQuery,
	result *query.QueryResult,
	status string,
	queryErr error,
) {
	if s.history == nil {
		return
	}
	entry, err := BuildQueryHistoryEntry(lq, model, cq, result, status, queryErr)
	if err != nil {
		s.logError(ctx, "build query history failed", err)
		return
	}
	if err := s.history.CreateQueryHistory(ctx, entry); err != nil {
		s.logError(ctx, "create query history failed", err)
	}
}

func (s *QueryService) logError(ctx context.Context, msg string, err error) {
	if s.logger != nil {
		s.logger.ErrorContext(ctx, msg, "error", err)
		return
	}
	slog.ErrorContext(ctx, msg, "error", err)
}

func BuildQueryHistoryEntry(
	lq query.LogicalQuery,
	model *semantic.SemanticModel,
	cq *query.CompiledQuery,
	result *query.QueryResult,
	status string,
	queryErr error,
) (*query.HistoryEntry, error) {
	lq.EnsureVersion()
	entry := &query.HistoryEntry{
		DatasourceID: lq.DatasourceID,
		ModelID:      historyModelID(model),
		LogicalQuery: lq,
		Status:       status,
		Fingerprint: query.ComputeFingerprint(query.FingerprintInputs{
			LogicalQuery:   lq,
			DatasourceID:   lq.DatasourceID,
			ContextVersion: semanticContextVersion(model),
		}),
	}
	if cq != nil {
		entry.CompiledSQL = &cq.SQL
		sqlArgs, err := marshalSQLArgs(cq.Args)
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

func historyModelID(model *semantic.SemanticModel) *string {
	if model == nil || model.ID == "" {
		return nil
	}
	return &model.ID
}

// semanticContextVersion stamps the semantic model version onto a query
// fingerprint so re-publishing the model naturally invalidates any cached
// matches keyed by the previous schema.
func semanticContextVersion(model *semantic.SemanticModel) string {
	if model == nil {
		return ""
	}
	return strconv.Itoa(model.Version)
}

func marshalSQLArgs(args []any) (*string, error) {
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
