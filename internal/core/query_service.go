package core

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"

	"github.com/biqly/biqly/internal/datasource"
	"github.com/biqly/biqly/internal/metadata"
	"github.com/biqly/biqly/internal/query"
	"github.com/biqly/biqly/internal/security"
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
	// Encryptor decrypts datasource DSNs when stored encrypted; nil means plaintext only.
	Encryptor *security.Encryption
}

type QueryService struct {
	models      ModelLoader
	datasources DatasourceLoader
	drivers     DriverGetter
	validator   *query.Validator
	executor    *query.Executor
	history     HistoryRecorder
	logger      *slog.Logger
	encryptor   *security.Encryption
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
		encryptor:   deps.Encryptor,
	}
}

func (s *QueryService) Compile(ctx context.Context, lq query.LogicalQuery) (*CompileResult, *ServiceError) {
	lq.EnsureVersion()
	loaded, se := s.loadContext(ctx, lq)
	if se != nil {
		return nil, se
	}
	query.RepairMisnamedCalendarGrainDimensions(&loaded.LogicalQuery, dimensionNames(loaded.Model))
	loaded.LogicalQuery.EnsureGroupBySelected()
	compiled, se := s.CompileWithContext(ctx, loaded.LogicalQuery, loaded.Model, loaded.Driver)
	if se != nil {
		return nil, se
	}
	loaded.Compiled = compiled
	return loaded, nil
}

func (s *QueryService) CompileWithContext(ctx context.Context, lq query.LogicalQuery, model *semantic.SemanticModel, driver datasource.Driver) (*query.CompiledQuery, *ServiceError) {
	query.RepairMisnamedCalendarGrainDimensions(&lq, dimensionNames(model))
	lq.EnsureGroupBySelected()
	if err := s.validator.Validate(lq, model); err != nil {
		return nil, ToServiceError(err)
	}
	compiled, err := query.NewCompiler(driver.Dialect()).Compile(ctx, lq, model)
	if err != nil {
		var valErrs query.ValidationErrors
		if errors.As(err, &valErrs) {
			return nil, ToServiceError(err)
		}
		return nil, ToServiceError(fmt.Errorf("compile: %w", err))
	}
	return compiled, nil
}

func (s *QueryService) Run(ctx context.Context, lq query.LogicalQuery) (*RunResult, *ServiceError) {
	lq.EnsureVersion()
	compiled, se := s.Compile(ctx, lq)
	if se != nil {
		return nil, se
	}
	dsn, err := metadata.RuntimeDSN(compiled.Datasource, s.encryptor)
	if err != nil {
		return nil, ToServiceError(fmt.Errorf("%w: %v", ErrLoadDatasource, err))
	}
	db, err := compiled.Driver.Open(ctx, dsn)
	if err != nil {
		return nil, ToServiceError(fmt.Errorf("%w: %v", ErrConnection, err))
	}
	defer func() { _ = db.Close() }()

	result, err := s.executor.Execute(ctx, db, compiled.Compiled)
	if err != nil {
		s.recordHistory(ctx, compiled.LogicalQuery, compiled.Model, compiled.Compiled, nil, QueryStatusFailed, err)
		return nil, ToServiceError(fmt.Errorf("%w: %v", ErrQueryExecution, err))
	}
	query.EnrichResult(result, lq, compiled.Model)
	s.recordHistory(ctx, compiled.LogicalQuery, compiled.Model, compiled.Compiled, result, QueryStatusSuccess, nil)
	return &RunResult{CompileResult: *compiled, Result: result}, nil
}

func (s *QueryService) DryRun(ctx context.Context, db *sql.DB, lq query.LogicalQuery, model *semantic.SemanticModel, driver datasource.Driver) *ServiceError {
	compiled, se := s.CompileWithContext(ctx, lq, model, driver)
	if se != nil {
		return se
	}
	explain := driver.Dialect().ExplainSQL(compiled.SQL)
	if explain == "" {
		return nil
	}
	rows, err := db.QueryContext(ctx, explain, compiled.Args...)
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
	for _, d := range model.Dimensions {
		out = append(out, d.Name)
	}
	return out
}

func (s *QueryService) loadContext(ctx context.Context, lq query.LogicalQuery) (*CompileResult, *ServiceError) {
	if lq.ModelID == "" {
		return nil, ToServiceError(ErrModelIDRequired)
	}
	if lq.DatasourceID == "" {
		return nil, ToServiceError(ErrDatasourceIDRequired)
	}
	model, err := s.models.GetPublishedFullModel(ctx, lq.ModelID)
	if err != nil {
		return nil, ToServiceError(fmt.Errorf("%w: %v", ErrLoadSemanticModel, err))
	}
	ds, err := s.datasources.GetDatasource(ctx, lq.DatasourceID)
	if err != nil {
		return nil, ToServiceError(fmt.Errorf("%w: %v", ErrLoadDatasource, err))
	}
	driver, err := s.drivers.Get(ds.Type)
	if err != nil {
		return nil, ToServiceError(fmt.Errorf("%w: %v", ErrLoadDriver, err))
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
	entry, err := query.BuildQueryHistoryEntry(lq, model, cq, result, status, queryErr)
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
