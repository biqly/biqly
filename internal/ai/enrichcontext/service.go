package enrichcontext

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/biqly/biqly/internal/ai"
	providerpkg "github.com/biqly/biqly/internal/ai/provider"
	"github.com/biqly/biqly/internal/core"
	"github.com/biqly/biqly/internal/datasource"
	"github.com/biqly/biqly/internal/metadata"
	"github.com/biqly/biqly/internal/security"
	"github.com/biqly/biqly/internal/semantic"
)

// Service detects context gaps and applies approved enrichments.
type Service struct {
	meta      *metadata.Repository
	semantic  *semantic.Repository
	client    providerpkg.Provider
	driverReg *datasource.Registry
	poolCache *datasource.PoolCache
	encryptor *security.Encryption
}

// NewService wires dependencies for enrich-context analysis.
func NewService(
	meta *metadata.Repository,
	semanticRepo *semantic.Repository,
	client providerpkg.Provider,
	driverReg *datasource.Registry,
	poolCache *datasource.PoolCache,
	encryptor *security.Encryption,
) *Service {
	return &Service{
		meta:      meta,
		semantic:  semanticRepo,
		client:    client,
		driverReg: driverReg,
		poolCache: poolCache,
		encryptor: encryptor,
	}
}

// Analyze loads model context, detects gaps, and optionally asks the LLM for suggestions.
func (s *Service) Analyze(ctx context.Context, req AnalyzeRequest) (*AnalyzeResult, error) {
	if req.DatasourceID == "" || req.ModelID == "" {
		return nil, errors.New("datasource_id and model_id are required")
	}
	model, err := s.semantic.GetFullModel(ctx, req.ModelID)
	if err != nil {
		return nil, fmt.Errorf("load model: %w", err)
	}
	if model.DatasourceID != req.DatasourceID {
		return nil, errors.New("model does not belong to datasource")
	}

	glossary, err := s.meta.ListBusinessGlossary(ctx, req.DatasourceID, req.ModelID)
	if err != nil {
		return nil, fmt.Errorf("list glossary: %w", err)
	}
	columns, err := s.meta.ListColumns(ctx, req.DatasourceID, "", "")
	if err != nil {
		return nil, fmt.Errorf("list columns: %w", err)
	}

	gaps := detectGaps(model, glossary, columns)
	result := &AnalyzeResult{
		DatasourceID: req.DatasourceID,
		ModelID:      req.ModelID,
		ModelName:    model.Name,
		Gaps:         gaps,
	}

	if req.Suggest {
		sampleSummary, sampleRows, sampleErr := s.sampleBaseTable(ctx, model)
		if sampleErr != nil {
			sampleSummary = ""
		}
		result.SampleRows = sampleRows
		suggestions, suggestErr := suggestForGaps(ctx, s.client, model, gaps, sampleSummary)
		if suggestErr != nil {
			return result, fmt.Errorf("suggest enrichments: %w", suggestErr)
		}
		result.Suggestions = suggestions
	}
	return result, nil
}

// Apply writes user-approved enrichment values.
func (s *Service) Apply(ctx context.Context, req ApplyRequest) (*ApplyResult, error) {
	if req.DatasourceID == "" || req.ModelID == "" {
		return nil, errors.New("datasource_id and model_id are required")
	}
	if len(req.Items) == 0 {
		return nil, errors.New("items are required")
	}
	model, err := s.semantic.GetFullModel(ctx, req.ModelID)
	if err != nil {
		return nil, fmt.Errorf("load model: %w", err)
	}
	if model.DatasourceID != req.DatasourceID {
		return nil, errors.New("model does not belong to datasource")
	}
	glossary, err := s.meta.ListBusinessGlossary(ctx, req.DatasourceID, req.ModelID)
	if err != nil {
		return nil, fmt.Errorf("list glossary: %w", err)
	}
	applied, skipped, errs := applyItems(ctx, model, glossary, s.meta, s.semantic, req.Items)
	return &ApplyResult{Applied: applied, Skipped: skipped, Errors: errs}, nil
}

func (s *Service) sampleBaseTable(ctx context.Context, model *semantic.SemanticModel) (summary string, rowCount int, err error) {
	if s.driverReg == nil || s.encryptor == nil {
		return "", 0, nil
	}
	ds, err := s.meta.GetDatasource(ctx, model.DatasourceID)
	if err != nil {
		return "", 0, err
	}
	driver, err := s.driverReg.Get(ds.Type)
	if err != nil {
		return "", 0, fmt.Errorf("%w: %w", core.ErrLoadDriver, err)
	}
	cols, err := s.meta.ListColumns(ctx, ds.ID, model.BaseSchema, model.BaseTable)
	if err != nil {
		return "", 0, err
	}
	if len(cols) == 0 {
		return "", 0, nil
	}
	dsn, err := metadata.RuntimeDSN(ds, s.encryptor)
	if err != nil {
		return "", 0, err
	}
	var db *sql.DB
	if s.poolCache != nil {
		db, err = s.poolCache.Get(ctx, driver, ds.ID, dsn)
	} else {
		db, err = driver.Open(ctx, dsn)
		if err == nil {
			defer func() { _ = db.Close() }()
		}
	}
	if err != nil {
		return "", 0, fmt.Errorf("%w: %w", core.ErrConnection, err)
	}
	rows, err := ai.FetchTableSample(ctx, db, driver.Dialect(), cols, model.BaseSchema, model.BaseTable, 5, 120)
	if err != nil {
		return "", 0, err
	}
	if len(rows) == 0 {
		return "no sample rows", 0, nil
	}
	var b strings.Builder
	fmt.Fprintf(&b, "table %s.%s sample (%d rows)\n", model.BaseSchema, model.BaseTable, len(rows))
	for i, row := range rows {
		if i >= 3 {
			break
		}
		fmt.Fprintf(&b, "row %d: %v\n", i+1, row)
	}
	return b.String(), len(rows), nil
}
