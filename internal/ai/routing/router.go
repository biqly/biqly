package routing

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"

	"github.com/biqly/biqly/internal/ai/lingua"
	"github.com/biqly/biqly/internal/i18n"
	"github.com/biqly/biqly/internal/metadata"
	"github.com/biqly/biqly/internal/semantic"
)

const (
	autoModelPrefix       = "auto:"
	maxAutoSelectedTables = 6
	maxExpandedAutoTables = 12 // after scoring: add FK bridge tables so picks form one component
	nameResolverMaxHops   = 3  // max FK hops to follow when resolving "<entity> name" questions
	minRouteConfidence    = 0.35
	// Limits for auto-generated semantic models (avoids multi-hundred-column explosion in LLM prompts).
	maxRankedColumnsPerTable = 24
	minColumnsBeforeRanking  = 12
)

var ErrTableScopeInvalid = errors.New("table scope invalid")

// ErrTypeScopeEmpty means both include_base_tables and include_views were false.
var ErrTypeScopeEmpty = errors.New("at least one of include_base_tables or include_views must be true")

// MetadataReader provides datasource metadata needed by TableRouter.
type MetadataReader interface {
	ListTables(ctx context.Context, datasourceID, schemaName string) ([]metadata.Table, error)
	ListColumns(ctx context.Context, datasourceID, schemaName, tableName string) ([]metadata.Column, error)
	ListRelations(ctx context.Context, datasourceID string) ([]metadata.Relation, error)
}

// EmbeddingReader returns previously-computed table and column embeddings for a
// datasource. Metadata without embeddings is simply absent from the result; the
// router falls back to keyword-only/table-wide context when vectors are missing.
type EmbeddingReader interface {
	ListTableEmbeddings(ctx context.Context, datasourceID string) ([]metadata.TableEmbedding, error)
	ListColumnEmbeddings(ctx context.Context, datasourceID string) ([]metadata.ColumnEmbedding, error)
}

// MetadataTranslator overlays entity_translations onto metadata rows for routing.
type MetadataTranslator interface {
	ApplyTableTranslations(ctx context.Context, tables []metadata.Table, loc i18n.Locale) error
	ApplyColumnTranslations(ctx context.Context, cols []metadata.Column, loc i18n.Locale) error
}

// TableRouter selects relevant tables and builds a synthetic semantic model.
// When both an Embedder and EmbeddingReader are configured (and embeddings
// have been precomputed for the datasource), scoring blends keyword overlap
// with cosine similarity between the question and each table embedding.
type TableRouter struct {
	reader          MetadataReader
	translator      MetadataTranslator
	embedder        Embedder
	embeddingReader EmbeddingReader
	embeddingWeight float64
	limits          Limits
	timeGrains      TimeGrainStore
}

// NewTableRouter creates a metadata-backed table router with no embeddings.
// Equivalent to NewTableRouterWithEmbeddings(reader, no embedder, no embedding reader, 0).
func NewTableRouter(reader MetadataReader) *TableRouter {
	return &TableRouter{
		reader:     reader,
		timeGrains: NewStaticTimeGrainStore(),
	}
}

// NewTableRouterWithEmbeddings creates a router that, when an Embedder and
// EmbeddingReader are both present, computes the question's embedding once
// per request and blends cosine-similarity into the scoring loop. weight
// controls the magnitude of that contribution (0 disables it).
func NewTableRouterWithEmbeddings(reader MetadataReader, embedder Embedder, embeddingReader EmbeddingReader, weight float64) *TableRouter {
	return &TableRouter{
		reader:          reader,
		embedder:        embedder,
		embeddingReader: embeddingReader,
		embeddingWeight: weight,
		limits:          DefaultRoutingLimits(),
		timeGrains:      NewStaticTimeGrainStore(),
	}
}

// SetTimeGrainStore configures the database-backed time grain store.
func (r *TableRouter) SetTimeGrainStore(store TimeGrainStore) {
	r.timeGrains = store
}

// SetMetadataTranslator enables localized descriptions for keyword routing and
// is required for Turkish embedding refresh (entity_translations).
func (r *TableRouter) SetMetadataTranslator(t MetadataTranslator) {
	r.translator = t
}

// SetRoutingLimits overrides auto-model caps (zero fields use defaults).
func (r *TableRouter) SetRoutingLimits(limits Limits) {
	r.limits = limits.withDefaults()
}

// TableCandidate is a scored table candidate returned by automatic routing.
type TableCandidate struct {
	Table          string  `json:"table"`
	Score          float64 `json:"score"`
	TotalScore     float64 `json:"total_score"`
	KeywordScore   float64 `json:"keyword_score"`
	EmbeddingScore float64 `json:"embedding_score"`
	Selected       bool    `json:"selected"`
	Description    string  `json:"description,omitempty"`
	RejectedReason string  `json:"rejected_reason,omitempty"`
}

// TableRoutingResult describes the table-routing decision for an AI query.
type TableRoutingResult struct {
	SelectedModels     []string           `json:"selected_models,omitempty"`
	SelectedTables     []string           `json:"selected_tables,omitempty"`
	SelectedDimensions []string           `json:"selected_dimensions,omitempty"`
	SelectedMetrics    []string           `json:"selected_metrics,omitempty"`
	JoinPaths          []string           `json:"join_paths,omitempty"`
	Candidates         []TableCandidate   `json:"candidates,omitempty"`
	Confidence         float64            `json:"confidence"`
	NeedsClarification bool               `json:"needs_clarification"`
	Manual             bool               `json:"manual"`
	ContextSource      string             `json:"context_source,omitempty"`
	ContextKey         string             `json:"context_key,omitempty"`
	ContextUpdatedAt   *time.Time         `json:"context_updated_at,omitempty"`
	Debug              *TableRoutingDebug `json:"debug,omitempty"`
	// RankingMethod tells the frontend which signal drove this decision:
	// "manual" when the user supplied a scope, "hybrid" when both keyword
	// and embedding similarity contributed, otherwise "keyword".
	RankingMethod string `json:"ranking_method,omitempty"`
	// CompositeID is set when the question was routed to a published composite
	// semantic model instead of a single model or auto-generated raw tables.
	CompositeID string `json:"composite_id,omitempty"`
	// ComponentModels lists the model IDs that make up the selected composite,
	// populated only when CompositeID is set.
	ComponentModels []string `json:"component_models,omitempty"`
}

type tableBundle struct {
	table          metadata.Table
	score          float64
	keywordScore   float64
	embeddingScore float64
}

// TableRoutingDebug carries explainability details for route decisions. It is
// intentionally compact so it can be returned in regular API responses.
type TableRoutingDebug struct {
	QuestionLocale       string   `json:"question_locale,omitempty"`
	RelationExpansion    []string `json:"relation_expansion,omitempty"`
	BridgeTables         []string `json:"bridge_tables,omitempty"`
	EliminatedCandidates []string `json:"eliminated_candidates,omitempty"`
	SchemaPartitions     []string `json:"schema_partitions,omitempty"`
}

func (r *TableRoutingResult) ensureDebug() *TableRoutingDebug {
	if r.Debug == nil {
		r.Debug = &TableRoutingDebug{}
	}
	return r.Debug
}

type embeddingSignals struct {
	tableBoost   map[string]float64
	columnScores map[string]float64
}

// Route selects tables for a question and returns a semantic model over them.
// includeBaseTables / includeViews restrict which metadata objects participate in routing
// (BASE TABLE vs VIEW). When both are true, behavior matches an unscoped datasource.
func (r *TableRouter) Route(
	ctx context.Context,
	datasourceID string,
	question string,
	tableScope []string,
	includeBaseTables bool,
	includeViews bool,
) (model *semantic.SemanticModel, result *TableRoutingResult, err error) {
	ctx, span := otel.Tracer("biqly/ai").Start(ctx, "ai.TableRoute")
	defer func() {
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
		}
		span.End()
	}()
	span.SetAttributes(
		attribute.String("datasource.id", datasourceID),
		attribute.Bool("ai.embedding_enabled", r.embedder != nil && r.embeddingReader != nil && r.embeddingWeight > 0),
	)

	tables, columns, relations, early, err := r.routeLoadAndFilter(ctx, datasourceID, tableScope, includeBaseTables, includeViews)
	if err != nil {
		if early != nil {
			return nil, early, err
		}
		return nil, nil, err
	}
	if early != nil {
		return nil, early, nil
	}

	tables, loc, columnsByTable, embedSignals, schemaPartitions, err := r.routePrepareSelection(
		ctx, datasourceID, question, tables, columns, relations, tableScope,
	)
	if err != nil {
		return nil, nil, err
	}

	selected, result, err := r.selectTables(tables, columnsByTable, question, tableScope, embedSignals.tableBoost)
	if err != nil {
		return nil, result, err
	}
	if result == nil {
		return nil, nil, errors.New("select tables: missing routing result")
	}
	r.routeAnnotateResult(result, loc, schemaPartitions, embedSignals)
	if result.NeedsClarification {
		return nil, result, nil
	}

	selected = r.routeExpandSelection(selected, tables, columnsByTable, relations, question, tableScope, result)
	model, result, err = r.routeFinalize(ctx, datasourceID, selected, relations, columnsByTable, question, embedSignals, result)
	if result != nil {
		span.SetAttributes(
			attribute.Float64("ai.route.confidence", result.Confidence),
			attribute.String("ai.ranking_method", result.RankingMethod),
		)
	}
	return model, result, err
}

func (r *TableRouter) routeLoadAndFilter(
	ctx context.Context,
	datasourceID string,
	tableScope []string,
	includeBaseTables bool,
	includeViews bool,
) (tables []metadata.Table, columns []metadata.Column, relations []metadata.Relation, early *TableRoutingResult, err error) {
	if !includeBaseTables && !includeViews {
		return nil, nil, nil, nil, ErrTypeScopeEmpty
	}
	tables, columns, relations, err = r.loadRoutingMetadata(ctx, datasourceID)
	if err != nil {
		return nil, nil, nil, nil, err
	}
	if err := validateManualScopeAgainstTypeScope(indexTables(tables), tableScope, includeBaseTables, includeViews); err != nil {
		return nil, nil, nil, &TableRoutingResult{Manual: true, NeedsClarification: true}, err
	}
	tables = filterTablesByTypeScope(tables, includeBaseTables, includeViews)
	columns = filterColumnsForTables(columns, tables)
	if len(tables) == 0 {
		return nil, nil, nil, &TableRoutingResult{NeedsClarification: true, Confidence: 0}, nil
	}
	return tables, columns, relations, nil, nil
}

func (r *TableRouter) routePrepareSelection(
	ctx context.Context,
	datasourceID string,
	question string,
	tables []metadata.Table,
	columns []metadata.Column,
	relations []metadata.Relation,
	tableScope []string,
) (filteredTables []metadata.Table, loc i18n.Locale, columnsByTable map[string][]metadata.Column, embedSignals embeddingSignals, schemaPartitions []string, err error) {
	loc = lingua.DetectQuestionLocale(question)
	if err := r.applyTranslations(ctx, tables, columns, loc); err != nil {
		return nil, "", nil, embeddingSignals{}, nil, err
	}
	columnsByTable = groupColumnsByTable(columns, len(tables))
	embedSignals = r.embeddingSignals(ctx, datasourceID, question, loc)
	filteredTables = tables
	if len(nonEmptyScope(tableScope)) == 0 {
		filteredTables, schemaPartitions = filterTablesBySchemaCluster(tables, columnsByTable, relations, question, embedSignals.tableBoost)
	}
	return filteredTables, loc, columnsByTable, embedSignals, schemaPartitions, nil
}

func (*TableRouter) routeAnnotateResult(result *TableRoutingResult, loc i18n.Locale, schemaPartitions []string, embedSignals embeddingSignals) {
	result.ensureDebug().QuestionLocale = string(loc)
	if len(schemaPartitions) > 0 {
		result.ensureDebug().SchemaPartitions = schemaPartitions
	}
	if result.RankingMethod != "" {
		return
	}
	switch {
	case result.Manual:
		result.RankingMethod = "manual"
	case len(embedSignals.tableBoost) > 0:
		result.RankingMethod = "hybrid"
	default:
		result.RankingMethod = "keyword"
	}
}

func (*TableRouter) routeExpandSelection(
	selected []tableBundle,
	tables []metadata.Table,
	columnsByTable map[string][]metadata.Column,
	relations []metadata.Relation,
	question string,
	tableScope []string,
	result *TableRoutingResult,
) []tableBundle {
	if len(selected) == 0 || result.Manual || len(nonEmptyScope(tableScope)) > 0 {
		return selected
	}
	questionTokens := tokenSet(question)
	tblIdx := indexTables(tables)
	selected = appendEntityResolverTables(selected, columnsByTable, relations, tblIdx, questionTokens, maxExpandedAutoTables, nameResolverMaxHops)
	selected = appendQuestionEntityTables(selected, tables, relations, tblIdx, questionTokens, maxExpandedAutoTables, nameResolverMaxHops)
	beforeBridge := bundleKeySet(selected)
	selected = expandSelectedWithJoinBridges(selected, relations, tblIdx, maxExpandedAutoTables)
	result.ensureDebug().BridgeTables = addedBundleLabels(beforeBridge, selected)
	return selected
}

func (r *TableRouter) routeFinalize(
	ctx context.Context,
	datasourceID string,
	selected []tableBundle,
	relations []metadata.Relation,
	columnsByTable map[string][]metadata.Column,
	question string,
	embedSignals embeddingSignals,
	result *TableRoutingResult,
) (*semantic.SemanticModel, *TableRoutingResult, error) {
	connected, joinPaths := connectSelectedTables(selected, relations)
	if result.Manual && len(connected) != len(selected) {
		result.NeedsClarification = true
		return nil, result, fmt.Errorf("%w: selected tables must have direct metadata relations", ErrTableScopeInvalid)
	}
	if len(connected) == 0 {
		result.NeedsClarification = true
		return nil, result, nil
	}
	result.SelectedTables = bundleLabels(connected)
	result.JoinPaths = joinPaths
	result.ensureDebug().RelationExpansion = joinPaths
	markSelectedCandidates(result.Candidates, result.SelectedTables)
	result.ensureDebug().EliminatedCandidates = eliminatedCandidateLabels(result.Candidates)

	limits := r.limits.withDefaults()
	questionTokens := tokenSet(question)
	columnsForModel := rankColumnsForSemanticModel(connected, columnsByTable, relations, questionTokens, embedSignals.columnScores, limits.MaxColumnsPerTable)
	model := buildSemanticModel(datasourceID, connected, columnsForModel, relations, limits, r.loadTimeGrains(ctx))
	if !result.Manual {
		pruneAutoSemanticModel(model, question, limits, embedSignals.columnScores)
	}
	contextSource := "auto"
	if result.Manual {
		contextSource = "manual"
	}
	applyModelContextToRouting(result, model, contextSource, nil)
	return model, result, nil
}

// loadRoutingMetadata fetches tables, columns, and relations for a datasource.
// The three queries are independent, so they run concurrently — DB roundtrips
// dominate this stage and parallelism roughly cuts cold-cache latency by 3x.
func (r *TableRouter) loadRoutingMetadata(ctx context.Context, datasourceID string) (
	tables []metadata.Table,
	columns []metadata.Column,
	relations []metadata.Relation,
	err error,
) {
	var (
		tablesErr error
		colsErr   error
		relErr    error
	)
	var listWG sync.WaitGroup
	listWG.Add(3)
	go func() {
		defer listWG.Done()
		tables, tablesErr = r.reader.ListTables(ctx, datasourceID, "")
	}()
	go func() {
		defer listWG.Done()
		columns, colsErr = r.reader.ListColumns(ctx, datasourceID, "", "")
	}()
	go func() {
		defer listWG.Done()
		relations, relErr = r.reader.ListRelations(ctx, datasourceID)
	}()
	listWG.Wait()
	switch {
	case tablesErr != nil:
		return nil, nil, nil, fmt.Errorf("list tables: %w", tablesErr)
	case colsErr != nil:
		return nil, nil, nil, fmt.Errorf("list columns: %w", colsErr)
	case relErr != nil:
		return nil, nil, nil, fmt.Errorf("list relations: %w", relErr)
	}
	return tables, columns, relations, nil
}

// applyTranslations mutates tables/columns in place with locale-specific
// metadata translations when the locale uses them and a translator is wired.
// It is a no-op otherwise, so callers can group columns afterwards unconditionally.
func (r *TableRouter) applyTranslations(
	ctx context.Context,
	tables []metadata.Table,
	columns []metadata.Column,
	loc i18n.Locale,
) error {
	profile, _ := i18n.LocaleProfileFor(loc)
	if r.translator == nil || !profile.UsesMetadataTranslations {
		return nil
	}
	if err := r.translator.ApplyTableTranslations(ctx, tables, loc); err != nil {
		return fmt.Errorf("apply table translations: %w", err)
	}
	if err := r.translator.ApplyColumnTranslations(ctx, columns, loc); err != nil {
		return fmt.Errorf("apply column translations: %w", err)
	}
	return nil
}

// loadTimeGrains returns the configured time grains, falling back to the
// defaults when no provider is wired or the lookup fails.
func (r *TableRouter) loadTimeGrains(ctx context.Context) []metadata.TimeGrain {
	if r.timeGrains == nil {
		return DefaultTimeGrains
	}
	timeGrains, err := r.timeGrains.List(ctx)
	if err != nil {
		return DefaultTimeGrains
	}
	return timeGrains
}

func (*TableRouter) selectTables(
	tables []metadata.Table,
	columnsByTable map[string][]metadata.Column,
	question string,
	tableScope []string,
	embedBoost map[string]float64,
) ([]tableBundle, *TableRoutingResult, error) {
	if len(tables) == 0 {
		return nil, &TableRoutingResult{
			NeedsClarification: true,
			Confidence:         0,
		}, nil
	}

	if len(nonEmptyScope(tableScope)) > 0 {
		return selectManualTables(tables, tableScope)
	}

	return selectAutomaticTables(tables, columnsByTable, question, embedBoost)
}

// embeddingSignals returns table boosts and per-column similarity from one
// question embedding. Any failed piece falls back independently to current
// keyword/table-wide behavior.
func (r *TableRouter) embeddingSignals(ctx context.Context, datasourceID, question string, loc i18n.Locale) embeddingSignals {
	if r.embedder == nil || r.embeddingReader == nil || r.embeddingWeight <= 0 {
		return embeddingSignals{}
	}

	ctx, span := otel.Tracer("biqly/ai").Start(ctx, "ai.RouteEmbedding")
	defer span.End()
	span.SetAttributes(
		attribute.String("datasource.id", datasourceID),
		attribute.String("ai.embedding.model", r.embedder.Model()),
	)
	storedTables, tableErr := r.embeddingReader.ListTableEmbeddings(ctx, datasourceID)
	storedColumns, columnErr := r.embeddingReader.ListColumnEmbeddings(ctx, datasourceID)
	if (tableErr != nil || len(storedTables) == 0) && (columnErr != nil || len(storedColumns) == 0) {
		return embeddingSignals{}
	}
	qVecs, err := r.embedder.Embed(ctx, []string{question})
	if err != nil || len(qVecs) == 0 || len(qVecs[0]) == 0 {
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
		}
		return embeddingSignals{}
	}
	q := qVecs[0]
	baseModel := r.embedder.Model()
	signals := embeddingSignals{}
	if tableErr == nil && len(storedTables) > 0 {
		signals.tableBoost = make(map[string]float64, len(storedTables))
		for _, te := range storedTables {
			if !lingua.EmbeddingModelMatches(te.Model, baseModel, loc) {
				continue
			}
			sim := CosineSimilarity(q, te.Embedding)
			if sim <= 0 {
				continue
			}
			signals.tableBoost[tableKey(te.SchemaName, te.TableName)] = sim * r.embeddingWeight
		}
	}
	if columnErr == nil && len(storedColumns) > 0 {
		signals.columnScores = make(map[string]float64, len(storedColumns))
		for _, ce := range storedColumns {
			if !lingua.EmbeddingModelMatches(ce.Model, baseModel, loc) {
				continue
			}
			sim := CosineSimilarity(q, ce.Embedding)
			signals.columnScores[columnKey(ce.SchemaName, ce.TableName, ce.ColumnName)] = sim
		}
	}
	return signals
}
