package ai

import (
	"context"
	"errors"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"
	"unicode"

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
	maxAutoModelDimensions   = 150
	maxAutoModelMetrics      = 120
	maxDateGrainExtras       = 48 // day/month/quarter/year variants per date columns (cap total)
	maxRankedColumnsPerTable = 24
	minColumnsBeforeRanking  = 12
)

type bundleColumn struct {
	bundle tableBundle
	col    metadata.Column
}

func columnPriority(c metadata.Column) int {
	// List human-readable dimensions before raw identifiers so prompts and models
	// default to names/titles rather than PK/FK columns.
	switch {
	case isDisplayNameColumn(c.ColumnName):
		return 0
	case c.IsPrimaryKey:
		return 3
	case c.IsForeignKey:
		return 2
	default:
		return 1
	}
}

// sortedBundleColumns returns columns across selected tables in a stable, business-relevant order.
func sortedBundleColumns(selected []tableBundle, columnsByTable map[string][]metadata.Column) []bundleColumn {
	var out []bundleColumn
	for _, bundle := range selected {
		key := tableKey(bundle.table.SchemaName, bundle.table.TableName)
		for _, col := range columnsByTable[key] {
			out = append(out, bundleColumn{bundle: bundle, col: col})
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		pi, pj := columnPriority(out[i].col), columnPriority(out[j].col)
		if pi != pj {
			return pi < pj
		}
		li := tableLabel(out[i].bundle.table) + "." + out[i].col.ColumnName
		lj := tableLabel(out[j].bundle.table) + "." + out[j].col.ColumnName
		return li < lj
	})
	return out
}

// ErrTableScopeInvalid indicates that a manually provided table scope is invalid.
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

// TableRouter selects relevant tables and builds a synthetic semantic model.
// When both an Embedder and EmbeddingReader are configured (and embeddings
// have been precomputed for the datasource), scoring blends keyword overlap
// with cosine similarity between the question and each table embedding.
type TableRouter struct {
	reader          MetadataReader
	embedder        Embedder
	embeddingReader EmbeddingReader
	embeddingWeight float64
	limits          RoutingLimits
}

// NewTableRouter creates a metadata-backed table router with no embeddings.
// Equivalent to NewTableRouterWithEmbeddings(reader, nil, nil, 0).
func NewTableRouter(reader MetadataReader) *TableRouter {
	return &TableRouter{reader: reader}
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
	}
}

// SetRoutingLimits overrides auto-model caps (zero fields use defaults).
func (r *TableRouter) SetRoutingLimits(limits RoutingLimits) {
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
) (*semantic.SemanticModel, *TableRoutingResult, error) {
	if !includeBaseTables && !includeViews {
		return nil, nil, ErrTypeScopeEmpty
	}

	tables, err := r.reader.ListTables(ctx, datasourceID, "")
	if err != nil {
		return nil, nil, fmt.Errorf("list tables: %w", err)
	}
	columns, err := r.reader.ListColumns(ctx, datasourceID, "", "")
	if err != nil {
		return nil, nil, fmt.Errorf("list columns: %w", err)
	}
	relations, err := r.reader.ListRelations(ctx, datasourceID)
	if err != nil {
		return nil, nil, fmt.Errorf("list relations: %w", err)
	}

	if err := validateManualScopeAgainstTypeScope(indexTables(tables), tableScope, includeBaseTables, includeViews); err != nil {
		return nil, &TableRoutingResult{Manual: true, NeedsClarification: true}, err
	}

	tables = filterTablesByTypeScope(tables, includeBaseTables, includeViews)
	columns = filterColumnsForTables(columns, tables)
	if len(tables) == 0 {
		return nil, &TableRoutingResult{
			NeedsClarification: true,
			Confidence:         0,
		}, nil
	}

	columnsByTable := groupColumnsByTable(columns)

	// Hybrid boost from precomputed embeddings, when configured. Skipped
	// silently on any error so a transient embedding-API failure or missing
	// vectors falls back cleanly to keyword scoring.
	embedSignals := r.embeddingSignals(ctx, datasourceID, question)

	var schemaPartitions []string
	if len(nonEmptyScope(tableScope)) == 0 {
		tables, schemaPartitions = filterTablesBySchemaCluster(tables, columnsByTable, relations, question, embedSignals.tableBoost)
	}

	selected, result, err := r.selectTables(tables, columnsByTable, question, tableScope, embedSignals.tableBoost)
	if err != nil {
		return nil, result, err
	}
	if len(schemaPartitions) > 0 {
		result.ensureDebug().SchemaPartitions = schemaPartitions
	}
	if result.RankingMethod == "" {
		switch {
		case result.Manual:
			result.RankingMethod = "manual"
		case len(embedSignals.tableBoost) > 0:
			result.RankingMethod = "hybrid"
		default:
			result.RankingMethod = "keyword"
		}
	}
	if result.NeedsClarification {
		return nil, result, nil
	}

	tblIdx := indexTables(tables)
	if len(selected) > 0 && !result.Manual && len(nonEmptyScope(tableScope)) == 0 {
		selected = appendEntityResolverTables(selected, columnsByTable, relations, tblIdx, tokenSet(question), maxExpandedAutoTables, nameResolverMaxHops)
		selected = appendQuestionEntityTables(selected, tables, relations, tblIdx, tokenSet(question), maxExpandedAutoTables, nameResolverMaxHops)
		beforeBridge := bundleKeySet(selected)
		selected = expandSelectedWithJoinBridges(selected, relations, tblIdx, maxExpandedAutoTables)
		result.ensureDebug().BridgeTables = addedBundleLabels(beforeBridge, selected)
	}

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
	columnsForModel := rankColumnsForSemanticModel(connected, columnsByTable, relations, question, embedSignals.columnScores, limits.MaxColumnsPerTable)
	model := buildSemanticModel(datasourceID, connected, columnsForModel, relations, limits)
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

func (r *TableRouter) selectTables(
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
func (r *TableRouter) embeddingSignals(ctx context.Context, datasourceID, question string) embeddingSignals {
	if r.embedder == nil || r.embeddingReader == nil || r.embeddingWeight <= 0 {
		return embeddingSignals{}
	}
	storedTables, tableErr := r.embeddingReader.ListTableEmbeddings(ctx, datasourceID)
	storedColumns, columnErr := r.embeddingReader.ListColumnEmbeddings(ctx, datasourceID)
	if (tableErr != nil || len(storedTables) == 0) && (columnErr != nil || len(storedColumns) == 0) {
		return embeddingSignals{}
	}
	qVecs, err := r.embedder.Embed(ctx, []string{question})
	if err != nil || len(qVecs) == 0 || len(qVecs[0]) == 0 {
		return embeddingSignals{}
	}
	q := qVecs[0]
	model := r.embedder.Model()
	signals := embeddingSignals{}
	if tableErr == nil && len(storedTables) > 0 {
		signals.tableBoost = make(map[string]float64, len(storedTables))
		for _, te := range storedTables {
			if te.Model != "" && te.Model != model {
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
			if ce.Model != "" && ce.Model != model {
				continue
			}
			sim := CosineSimilarity(q, ce.Embedding)
			signals.columnScores[columnKey(ce.SchemaName, ce.TableName, ce.ColumnName)] = sim
		}
	}
	return signals
}

func selectManualTables(
	tables []metadata.Table,
	tableScope []string,
) ([]tableBundle, *TableRoutingResult, error) {
	tableIndex := indexTables(tables)
	var selected []tableBundle
	seen := make(map[string]struct{})

	for _, ref := range nonEmptyScope(tableScope) {
		table, err := resolveTableRef(tableIndex, ref)
		if err != nil {
			result := &TableRoutingResult{Manual: true, NeedsClarification: true}
			return nil, result, fmt.Errorf("%w: %v", ErrTableScopeInvalid, err)
		}
		key := tableKey(table.SchemaName, table.TableName)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		selected = append(selected, tableBundle{
			table:        table,
			score:        1,
			keywordScore: 1,
		})
	}

	result := &TableRoutingResult{
		SelectedTables: bundleLabels(selected),
		Confidence:     1,
		Manual:         true,
	}
	return selected, result, nil
}

func selectAutomaticTables(
	tables []metadata.Table,
	columnsByTable map[string][]metadata.Column,
	question string,
	embedBoost map[string]float64,
) ([]tableBundle, *TableRoutingResult, error) {
	tokens := tokenSet(question)
	bundles := make([]tableBundle, 0, len(tables))
	for _, table := range tables {
		key := tableKey(table.SchemaName, table.TableName)
		keywordScore := scoreTable(table, columnsByTable[key], tokens)
		embeddingScore := embedBoost[key]
		score := keywordScore + embeddingScore
		bundles = append(bundles, tableBundle{
			table:          table,
			score:          score,
			keywordScore:   keywordScore,
			embeddingScore: embeddingScore,
		})
	}

	sort.SliceStable(bundles, func(i, j int) bool {
		if bundles[i].score == bundles[j].score {
			return tableLabel(bundles[i].table) < tableLabel(bundles[j].table)
		}
		return bundles[i].score > bundles[j].score
	})

	candidates := topCandidates(bundles, 5)
	confidence := routeConfidence(bundles)
	if len(bundles) == 0 || bundles[0].score == 0 || confidence < minRouteConfidence {
		return nil, &TableRoutingResult{
			Candidates:         candidates,
			Confidence:         confidence,
			NeedsClarification: true,
		}, nil
	}

	selected := []tableBundle{bundles[0]}
	for _, bundle := range bundles[1:] {
		if len(selected) >= maxAutoSelectedTables || bundle.score == 0 {
			break
		}
		if bundle.score >= bundles[0].score*activeRoutingWeights().SelectionRelativeThreshold {
			selected = append(selected, bundle)
		}
	}
	selected = appendCategoryTableIfMissing(selected, bundles, tokens, maxAutoSelectedTables)
	selected = appendProductTableIfMissing(selected, bundles, tokens, maxAutoSelectedTables)

	result := &TableRoutingResult{
		SelectedTables: bundleLabels(selected),
		Candidates:     candidates,
		Confidence:     confidence,
	}
	return selected, result, nil
}

// appendCategoryTableIfMissing ensures category breakdown questions pull in a *category* table
// even when it ranked just below the score threshold (needed for revenue-by-category queries).
func appendCategoryTableIfMissing(
	selected []tableBundle,
	bundles []tableBundle,
	tokens map[string]bool,
	maxN int,
) []tableBundle {
	if !isCategoryOrProductQuestion(tokens) {
		return selected
	}
	lex := activeRoutingLexicon()
	for _, b := range selected {
		if tableNameMatchesSubstrings(b.table.TableName, lex.CategoryTableSubstrings) {
			return selected
		}
	}
	pick := func() (tableBundle, bool) {
		for _, b := range bundles {
			if b.score == 0 {
				continue
			}
			if !tableNameMatchesSubstrings(b.table.TableName, lex.CategoryTableSubstrings) {
				continue
			}
			k := tableKey(b.table.SchemaName, b.table.TableName)
			if bundleSliceContains(selected, k) {
				continue
			}
			return b, true
		}
		return tableBundle{}, false
	}
	if len(selected) < maxN {
		if b, ok := pick(); ok {
			selected = append(selected, b)
		}
		return selected
	}
	// Full slate but no category table: swap out the lowest-priority tail pick.
	if len(selected) < 2 {
		return selected
	}
	if b, ok := pick(); ok {
		selected[len(selected)-1] = b
	}
	return selected
}

// appendQuestionEntityTables pulls in unselected tables whose name (after token
// expansion) matches a question token, following the shortest FK path from the
// already-selected set. This covers "show total sales by customer" — the user
// names an entity but does not say "customer name", so the readable-labels
// resolver does not fire, yet the customers table still needs to be in the
// context for a "by customer" group-by to be sensible.
//
// Conservative: only adds a table when there is an FK path to one of the
// already-selected tables within maxHops, so unrelated entity collisions
// ("show sales" → don't pull in employees) are filtered out.
func appendQuestionEntityTables(
	selected []tableBundle,
	tables []metadata.Table,
	relations []metadata.Relation,
	idx tableIndex,
	tokens map[string]bool,
	maxN, maxHops int,
) []tableBundle {
	if len(tokens) == 0 || len(selected) >= maxN {
		return selected
	}
	selectedKeys := make(map[string]struct{}, len(selected))
	for _, b := range selected {
		selectedKeys[tableKey(b.table.SchemaName, b.table.TableName)] = struct{}{}
	}

	adj := relationAdjacency(relations)
	from := make(map[string]struct{}, len(selectedKeys))
	for k := range selectedKeys {
		from[k] = struct{}{}
	}

	for _, t := range tables {
		key := tableKey(t.SchemaName, t.TableName)
		if _, ok := selectedKeys[key]; ok {
			continue
		}
		nameTokens := tokenSet(t.TableName)
		matched := false
		for tok := range nameTokens {
			// Ignore generic tokens that match too eagerly across schemas
			// (e.g. table called "data" or column-name leftover).
			if len(tok) < 3 {
				continue
			}
			if tokens[tok] {
				matched = true
				break
			}
		}
		if !matched {
			continue
		}
		path := shortestPathFromSet(adj, from, key)
		if path == nil || len(path) > maxHops+1 {
			continue
		}
		for i := 1; i < len(path); i++ {
			if len(selected) >= maxN {
				return selected
			}
			pkey := path[i]
			if _, ok := selectedKeys[pkey]; ok {
				continue
			}
			pt, ok := idx.byFullName[pkey]
			if !ok {
				continue
			}
			w := activeRoutingWeights()
			score := w.EntityPathBridgeScore
			if pkey == key {
				score = w.EntityPathTargetScore
			}
			selected = append(selected, tableBundle{table: pt, score: score})
			selectedKeys[pkey] = struct{}{}
			from[pkey] = struct{}{}
		}
	}
	return selected
}

// appendEntityResolverTables ensures questions like "<entity> name" can be answered
// by pulling in the entity table itself plus any downstream display-name table
// reached through FK chains, even when those tables didn't score in the initial
// top picks (or are views without FK metadata to begin with).
func appendEntityResolverTables(
	selected []tableBundle,
	columnsByTable map[string][]metadata.Column,
	relations []metadata.Relation,
	idx tableIndex,
	tokens map[string]bool,
	maxN, maxHops int,
) []tableBundle {
	if !wantsReadableLabelsQuestion(tokens) {
		return selected
	}

	entityTokens := make(map[string]bool)
	for tok := range tokens {
		if isNameLikeToken(tok) {
			continue
		}
		entityTokens[tok] = true
	}
	if len(entityTokens) == 0 {
		return selected
	}

	selectedKeys := make(map[string]struct{})
	for _, b := range selected {
		selectedKeys[tableKey(b.table.SchemaName, b.table.TableName)] = struct{}{}
	}

	adj := relationAdjacency(relations)

	addPathTo := func(targetKey string) {
		from := make(map[string]struct{}, len(selectedKeys))
		for k := range selectedKeys {
			from[k] = struct{}{}
		}
		path := shortestPathFromSet(adj, from, targetKey)
		if path == nil {
			return
		}
		for i := 1; i < len(path); i++ {
			if len(selected) >= maxN {
				return
			}
			pkey := path[i]
			if _, ok := selectedKeys[pkey]; ok {
				continue
			}
			t, ok := idx.byFullName[pkey]
			if !ok {
				continue
			}
			w := activeRoutingWeights()
			score := w.ResolverPathBridgeScore
			if pkey == targetKey {
				score = w.ResolverPathTargetScore
			}
			selected = append(selected, tableBundle{table: t, score: score})
			selectedKeys[pkey] = struct{}{}
		}
	}

	type cand struct {
		key  string
		hops int
	}

	// BFS once to enumerate candidate tables matching entity tokens within maxHops.
	visited := make(map[string]int, len(selectedKeys))
	queue := make([]string, 0, len(selectedKeys))
	for k := range selectedKeys {
		visited[k] = 0
		queue = append(queue, k)
	}
	var entityCands []cand
	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		d := visited[cur]
		if d >= maxHops {
			continue
		}
		for _, nb := range adj[cur] {
			if _, seen := visited[nb]; seen {
				continue
			}
			visited[nb] = d + 1
			queue = append(queue, nb)
			if _, ok := selectedKeys[nb]; ok {
				continue
			}
			t, ok := idx.byFullName[nb]
			if !ok {
				continue
			}
			tname := strings.ToLower(t.TableName)
			if entityTokens[tname] || entityTokens[singularize(tname)] {
				entityCands = append(entityCands, cand{nb, d + 1})
			}
		}
	}
	sort.SliceStable(entityCands, func(i, j int) bool { return entityCands[i].hops < entityCands[j].hops })

	// Pass 1: include entity tables themselves (and bridges on the path).
	for _, c := range entityCands {
		if len(selected) >= maxN {
			break
		}
		addPathTo(c.key)
	}

	// Pass 2: for each entity-matching table in the connected set (whether it
	// was already there from initial scoring or just added in pass 1) that has
	// no display-name column of its own, walk one or two more FK hops to find a
	// display-bearing partner (e.g. customer → person.firstname/lastname).
	entityKeys := make([]string, 0, len(entityCands)+len(selected))
	for _, b := range selected {
		bk := tableKey(b.table.SchemaName, b.table.TableName)
		tname := strings.ToLower(b.table.TableName)
		if entityTokens[tname] || entityTokens[singularize(tname)] {
			entityKeys = append(entityKeys, bk)
		}
	}
	for _, c := range entityCands {
		entityKeys = append(entityKeys, c.key)
	}
	visitedEntity := make(map[string]bool, len(entityKeys))
	for _, ek := range entityKeys {
		if len(selected) >= maxN {
			break
		}
		if visitedEntity[ek] {
			continue
		}
		visitedEntity[ek] = true
		if _, ok := selectedKeys[ek]; !ok {
			continue
		}
		c := cand{key: ek}
		if hasDisplayNameInColumns(columnsByTable[c.key]) {
			continue
		}
		eVisited := map[string]int{c.key: 0}
		eQueue := []string{c.key}
		bestKey := ""
		bestHops := -1
		for len(eQueue) > 0 {
			cur := eQueue[0]
			eQueue = eQueue[1:]
			d := eVisited[cur]
			if d >= maxHops {
				continue
			}
			for _, nb := range adj[cur] {
				if _, seen := eVisited[nb]; seen {
					continue
				}
				eVisited[nb] = d + 1
				eQueue = append(eQueue, nb)
				if !hasDisplayNameInColumns(columnsByTable[nb]) {
					continue
				}
				if bestHops < 0 || d+1 < bestHops {
					bestKey = nb
					bestHops = d + 1
				}
			}
		}
		if bestKey != "" {
			addPathTo(bestKey)
		}
	}

	return selected
}

func hasDisplayNameInColumns(cols []metadata.Column) bool {
	for _, c := range cols {
		if isDisplayNameColumn(c.ColumnName) {
			return true
		}
	}
	return false
}

func isNameLikeToken(tok string) bool {
	lex := activeRoutingLexicon()
	for _, t := range lex.NameLikeTokens {
		if tok == t {
			return true
		}
	}
	return false
}

func bundleSliceContains(bundles []tableBundle, key string) bool {
	for _, b := range bundles {
		if tableKey(b.table.SchemaName, b.table.TableName) == key {
			return true
		}
	}
	return false
}

func wantsReadableLabelsQuestion(tokens map[string]bool) bool {
	return activeRoutingLexicon().HasAnyToken(tokens, activeRoutingLexicon().ReadableLabelTokens)
}

func scoreTable(table metadata.Table, columns []metadata.Column, tokens map[string]bool) float64 {
	w := activeRoutingWeights()
	lex := activeRoutingLexicon()
	score := weightedTokenScore(tokens, table.SchemaName+" "+table.TableName, w.TableName)
	if table.Description != nil {
		score += weightedTokenScore(tokens, *table.Description, w.TableDescription)
	}
	score = w.ApplyTableBoosts(table.TableName, tokens, score, lex)
	for _, col := range columns {
		score += weightedTokenScore(tokens, col.ColumnName, w.ColumnName)
		score += weightedTokenScore(tokens, col.DataType, w.ColumnDataType)
		if col.Description != nil {
			score += weightedTokenScore(tokens, *col.Description, w.ColumnDescription)
		}
		if isRevenueLikeQuestion(tokens) && isRevenueLikeColumn(col) {
			score += w.RevenueColumnBoost
		}
	}
	if wantsReadableLabelsQuestion(tokens) {
		for _, col := range columns {
			if isDisplayNameColumn(col.ColumnName) {
				score += w.ReadableLabelColumnBoost
				break
			}
		}
	}
	return score
}

func isCategoryOrProductQuestion(tokens map[string]bool) bool {
	return activeRoutingLexicon().HasAnyToken(tokens, activeRoutingLexicon().CategoryProductTokens)
}

func isQuantityOrCountIntent(tokens map[string]bool) bool {
	return activeRoutingLexicon().HasAnyToken(tokens, activeRoutingLexicon().QuantityTokens)
}

// appendProductTableIfMissing pulls in production.product (or subcategory) when the
// question is product-focused but the top-N scored tables missed the catalog table
// (common when line items score highest).
func appendProductTableIfMissing(
	selected []tableBundle,
	bundles []tableBundle,
	tokens map[string]bool,
	maxN int,
) []tableBundle {
	if !isCategoryOrProductQuestion(tokens) {
		return selected
	}
	lex := activeRoutingLexicon()
	for _, b := range selected {
		if tableNameMatchesSubstrings(b.table.TableName, lex.ProductCatalogSubstrings) {
			return selected
		}
	}
	pick := func() (tableBundle, bool) {
		for _, b := range bundles {
			if b.score == 0 {
				continue
			}
			if !tableNameMatchesSubstrings(b.table.TableName, lex.ProductCatalogSubstrings) {
				continue
			}
			k := tableKey(b.table.SchemaName, b.table.TableName)
			if bundleSliceContains(selected, k) {
				continue
			}
			return b, true
		}
		return tableBundle{}, false
	}
	if len(selected) < maxN {
		if b, ok := pick(); ok {
			selected = append(selected, b)
		}
		return selected
	}
	if len(selected) < 2 {
		return selected
	}
	if b, ok := pick(); ok {
		selected[len(selected)-1] = b
	}
	return selected
}

func buildSemanticModel(
	datasourceID string,
	selected []tableBundle,
	columnsByTable map[string][]metadata.Column,
	relations []metadata.Relation,
	limits RoutingLimits,
) *semantic.SemanticModel {
	limits = limits.withDefaults()
	base := selected[0].table
	label := "Auto-detected tables"
	description := "Generated from datasource metadata for an AI question."
	model := &semantic.SemanticModel{
		ID:           autoModelPrefix + strings.Join(bundleLabels(selected), ","),
		DatasourceID: datasourceID,
		Name:         autoModelPrefix + strings.Join(bundleLabels(selected), ","),
		Label:        &label,
		Description:  &description,
		BaseSchema:   base.SchemaName,
		BaseTable:    base.TableName,
		IsActive:     true,
	}

	model.Dimensions = buildDimensions(selected, columnsByTable, limits)
	model.Metrics = buildMetrics(selected, columnsByTable, limits)
	model.Joins = buildJoins(selected, relations)
	return model
}

func isMandatorySemanticColumn(col metadata.Column, relationCols map[string]bool) bool {
	if col.IsPrimaryKey || col.IsForeignKey || isDateOrTimeType(col.DataType) || isDisplayNameColumn(col.ColumnName) {
		return true
	}
	if relationCols != nil && relationCols[col.ColumnName] {
		return true
	}
	return false
}

func relationColumnsForSelectedTables(relations []metadata.Relation, selectedKeys map[string]bool) map[string]map[string]bool {
	out := make(map[string]map[string]bool)
	add := func(tableKey, columnName string) {
		if out[tableKey] == nil {
			out[tableKey] = make(map[string]bool)
		}
		out[tableKey][columnName] = true
	}
	for _, rel := range relations {
		fromKey := tableKey(rel.FromSchema, rel.FromTable)
		toKey := tableKey(rel.ToSchema, rel.ToTable)
		if !selectedKeys[fromKey] || !selectedKeys[toKey] {
			continue
		}
		add(fromKey, rel.FromColumn)
		add(toKey, rel.ToColumn)
	}
	return out
}

func buildDimensions(selected []tableBundle, columnsByTable map[string][]metadata.Column, limits RoutingLimits) []semantic.Dimension {
	limits = limits.withDefaults()
	maxDims := limits.MaxDimensions
	maxDateGrains := limits.MaxDateGrainExtras
	nameCounts := columnNameCounts(selected, columnsByTable)
	pairs := sortedBundleColumns(selected, columnsByTable)
	if len(pairs) > maxDims {
		pairs = pairs[:maxDims]
	}
	dimensions := make([]semantic.Dimension, 0, len(pairs))
	dateGrainAdded := 0
	for _, p := range pairs {
		if len(dimensions) >= maxDims {
			break
		}
		name := p.col.ColumnName
		if nameCounts[p.col.ColumnName] > 1 {
			name = p.bundle.table.TableName + "_" + p.col.ColumnName
		}
		colRef := p.bundle.table.TableName + "." + p.col.ColumnName
		syn := displayNameSynonyms(p.bundle.table.TableName, p.col.ColumnName)
		if sx := softDeleteColumnSynonyms(p.col.ColumnName, p.col.DataType); len(sx) > 0 {
			syn = append(syn, sx...)
		}
		dimensions = append(dimensions, semantic.Dimension{
			Name:        name,
			ColumnRef:   colRef,
			Type:        dimensionType(p.col.DataType),
			Description: p.col.Description,
			Synonyms:    syn,
			IsActive:    true,
		})
		if !isDateOrTimeType(p.col.DataType) || dateGrainAdded >= maxDateGrains {
			continue
		}
		hasTime := hasTimeComponent(p.col.DataType)
		grains := []struct {
			part, suffix string
			requiresTime bool
			syns         []string
		}{
			{"year", "_year", false, []string{"year", "years", "yearly", "annual", "yıl", "yil", "yıllık", "yillik", "per year", "by year"}},
			{"quarter", "_quarter", false, []string{"quarter", "quarters", "qtr", "çeyrek", "ceyrek", "çeyreklik", "ceyreklik"}},
			{"month", "_month", false, []string{"month", "months", "monthly", "ay", "aylık", "aylik", "per month", "by month"}},
			{"day", "_day", false, []string{"day", "days", "daily", "gün", "gun", "günlük", "gunluk", "per day", "by day", "günü", "gunu"}},
			{"hour", "_hour", true, []string{"hour", "hours", "hourly", "saat", "saatlik", "saatte", "saatli", "per hour", "by hour"}},
		}
		for _, g := range grains {
			if g.requiresTime && !hasTime {
				continue
			}
			if len(dimensions) >= maxDims || dateGrainAdded >= maxDateGrains {
				break
			}
			dimensions = append(dimensions, semantic.Dimension{
				Name:        name + g.suffix,
				ColumnRef:   colRef,
				Type:        string(semantic.DimensionTypeDate),
				TimeGrain:   g.part,
				Synonyms:    g.syns,
				Description: p.col.Description,
				IsActive:    true,
			})
			dateGrainAdded++
		}
	}
	return dimensions
}

// softDeleteColumnSynonyms adds NL phrases so questions like "silinen tweet"
// map to deletion-indicator dimensions (deleted_at, is_deleted, delete_flag, …).
func softDeleteColumnSynonyms(columnName, dataType string) []string {
	n := strings.ToLower(strings.TrimSpace(columnName))
	t := strings.ToLower(strings.TrimSpace(dataType))
	isTimeish := strings.Contains(t, "timestamp") || strings.Contains(t, "timestamptz") ||
		t == "date"
	isBool := strings.Contains(t, "bool")
	isNum := isNumericType(t)

	tsDeleted := n == "deleted_at" || strings.HasSuffix(n, "_deleted_at") ||
		n == "removed_at" || strings.HasSuffix(n, "_removed_at")
	tsArchived := n == "archived_at" || strings.HasSuffix(n, "_archived_at")

	switch {
	case isTimeish && tsDeleted:
		return []string{
			"deleted", "removed", "trashed", "erased", "soft delete", "soft-delete",
			"silinen", "silinmiş", "silindi", "silinmis", "kaldırılan", "kaldirilan",
		}
	case isTimeish && tsArchived:
		return []string{
			"archived", "arşiv", "arsiv", "arşivlenmiş", "arsivlenmis",
			"deleted", "silinen", "kaldırılan", "kaldirilan",
		}
	case isBool && (n == "is_deleted" || n == "is_removed" || n == "is_archived" || n == "deleted"):
		return []string{
			"deleted", "removed", "archived", "silinen", "silinmiş", "silinmis", "silindi", "kaldırılan", "kaldirilan",
		}
	case isNum && (n == "delete_flag" || n == "deleted_flag" || n == "is_delete"):
		return []string{
			"deleted", "delete flag", "silinen", "silme bayrağı", "silme bayragi",
		}
	default:
		return nil
	}
}

// displayNameSynonyms tags human-readable label columns (name, title, label, ...)
// with the parent table's name and its known translations, so a question that
// refers to the entity generically ("customer" / "müşteri") routes to the
// display column instead of the primary key.
func displayNameSynonyms(tableName, columnName string) []string {
	if !isDisplayNameColumn(columnName) {
		return nil
	}
	base := singularize(strings.ToLower(tableName))
	seen := map[string]bool{}
	add := func(s string) []string {
		s = strings.TrimSpace(strings.ToLower(s))
		if s == "" || seen[s] {
			return nil
		}
		seen[s] = true
		return []string{s}
	}
	var out []string
	out = append(out, add(tableName)...)
	out = append(out, add(base)...)
	for _, syn := range activeRoutingLexicon().ExpandTokenSynonyms(base) {
		out = append(out, add(syn)...)
	}
	return out
}

func isDisplayNameColumn(name string) bool {
	n := strings.ToLower(name)
	switch n {
	case "title", "label", "username", "email":
		return true
	}
	// Matches name, firstname, lastname, middlename, surname, full_name,
	// display_name, store_name, *Name, etc.
	return strings.HasSuffix(n, "name")
}

func singularize(name string) string {
	switch {
	case strings.HasSuffix(name, "ies") && len(name) > 3:
		return strings.TrimSuffix(name, "ies") + "y"
	case strings.HasSuffix(name, "ses") && len(name) > 3:
		return strings.TrimSuffix(name, "es")
	case strings.HasSuffix(name, "s") && len(name) > 3:
		return strings.TrimSuffix(name, "s")
	default:
		return name
	}
}

func buildMetrics(selected []tableBundle, columnsByTable map[string][]metadata.Column, limits RoutingLimits) []semantic.Metric {
	limits = limits.withDefaults()
	maxMetrics := limits.MaxMetrics
	lex := activeRoutingLexicon()
	metrics := []semantic.Metric{{
		Name:        "row_count",
		Expression:  "*",
		Aggregation: string(semantic.AggCount),
		Synonyms:    append([]string(nil), lex.RowCountSynonyms...),
		IsActive:    true,
	}}

	nameCounts := columnNameCounts(selected, columnsByTable)
	pairs := sortedBundleColumns(selected, columnsByTable)

	appendMetric := func(m semantic.Metric) {
		if len(metrics) >= maxMetrics {
			return
		}
		metrics = append(metrics, m)
	}

	for _, p := range pairs {
		if len(metrics) >= maxMetrics {
			break
		}
		col := p.col
		name := col.ColumnName
		if nameCounts[col.ColumnName] > 1 {
			name = p.bundle.table.TableName + "_" + col.ColumnName
		}
		expression := p.bundle.table.TableName + "." + col.ColumnName
		switch {
		case isNumericType(col.DataType):
			appendMetric(metric("sum_"+name, expression, semantic.AggSum, col.Description, nil))
			if !limits.SlimNumericMetrics {
				appendMetric(metric("avg_"+name, expression, semantic.AggAvg, col.Description, nil))
				appendMetric(metric("min_"+name, expression, semantic.AggMin, col.Description, lex.MetricSynonymList("min_numeric")))
			}
			appendMetric(metric("max_"+name, expression, semantic.AggMax, col.Description, lex.MetricSynonymList("max_numeric")))
		case isDateOrTimeType(col.DataType):
			appendMetric(metric("min_"+name, expression, semantic.AggMin, col.Description, lex.MetricSynonymList("min_date")))
			appendMetric(metric("max_"+name, expression, semantic.AggMax, col.Description, lex.MetricSynonymList("max_date")))
		}
	}
	return metrics
}

func metric(name string, expression string, aggregation semantic.AggregationType, description *string, synonyms []string) semantic.Metric {
	return semantic.Metric{
		Name:        name,
		Expression:  expression,
		Aggregation: string(aggregation),
		Description: description,
		Synonyms:    synonyms,
		IsActive:    true,
	}
}

func buildJoins(selected []tableBundle, relations []metadata.Relation) []semantic.Join {
	if len(selected) < 2 {
		return nil
	}
	allKeys := make(map[string]bool, len(selected))
	for _, bundle := range selected {
		allKeys[tableKey(bundle.table.SchemaName, bundle.table.TableName)] = true
	}

	seen := make(map[string]bool)
	var joins []semantic.Join
	for _, rel := range relations {
		fromKey := tableKey(rel.FromSchema, rel.FromTable)
		toKey := tableKey(rel.ToSchema, rel.ToTable)
		if !allKeys[fromKey] || !allKeys[toKey] {
			continue
		}
		dedupe := rel.ConstraintName
		if dedupe == "" {
			dedupe = fromKey + "|" + toKey + "|" + rel.FromColumn + "|" + rel.ToColumn
		}
		if seen[dedupe] {
			continue
		}
		seen[dedupe] = true
		jname := rel.ConstraintName
		if jname == "" {
			jname = fmt.Sprintf("%s_%s_%s_to_%s", rel.FromTable, rel.FromColumn, rel.ToTable, rel.ToColumn)
		}
		joins = append(joins, semantic.Join{
			Name:         jname,
			FromSchema:   rel.FromSchema,
			FromTable:    rel.FromTable,
			FromColumn:   rel.FromColumn,
			ToSchema:     rel.ToSchema,
			ToTable:      rel.ToTable,
			ToColumn:     rel.ToColumn,
			JoinType:     semantic.DefaultJoinType,
			Relationship: rel.RelationshipType,
			IsActive:     true,
		})
	}
	return joins
}

// connectSelectedTables keeps every scored table that connects to the base table
// through a path of metadata relations (not only direct base↔table edges).
func connectSelectedTables(selected []tableBundle, relations []metadata.Relation) ([]tableBundle, []string) {
	if len(selected) < 2 {
		return selected, nil
	}

	connected := []tableBundle{selected[0]}
	remaining := append([]tableBundle(nil), selected[1:]...)
	var joinPaths []string

	for len(remaining) > 0 {
		added := false
		var still []tableBundle
		for _, cand := range remaining {
			attached := false
			for _, exist := range connected {
				if rel, ok := directRelation(exist.table, cand.table, relations); ok {
					connected = append(connected, cand)
					joinPaths = append(joinPaths, relationPath(rel))
					attached = true
					added = true
					break
				}
			}
			if !attached {
				still = append(still, cand)
			}
		}
		remaining = still
		if !added {
			break
		}
	}
	return connected, joinPaths
}

// relationAdjacency builds an undirected graph of tables linked by metadata FKs.
func relationAdjacency(relations []metadata.Relation) map[string][]string {
	adj := make(map[string][]string)
	link := func(a, b string) {
		adj[a] = append(adj[a], b)
		adj[b] = append(adj[b], a)
	}
	for _, rel := range relations {
		a := tableKey(rel.FromSchema, rel.FromTable)
		b := tableKey(rel.ToSchema, rel.ToTable)
		link(a, b)
	}
	return adj
}

// shortestPathFromSet returns a path from any node in `from` to `to`, or nil if unreachable.
func shortestPathFromSet(adj map[string][]string, from map[string]struct{}, to string) []string {
	if _, ok := from[to]; ok {
		return []string{to}
	}
	queue := make([]string, 0, len(from))
	parent := make(map[string]string)
	seen := make(map[string]struct{})
	for k := range from {
		queue = append(queue, k)
		seen[k] = struct{}{}
		parent[k] = ""
	}
	for qi := 0; qi < len(queue); qi++ {
		cur := queue[qi]
		for _, nb := range adj[cur] {
			if _, ok := seen[nb]; ok {
				continue
			}
			seen[nb] = struct{}{}
			parent[nb] = cur
			if nb == to {
				var path []string
				for x := to; x != ""; x = parent[x] {
					path = append(path, x)
				}
				for i, j := 0, len(path)-1; i < j; i, j = i+1, j-1 {
					path[i], path[j] = path[j], path[i]
				}
				return path
			}
			queue = append(queue, nb)
		}
	}
	return nil
}

// expandSelectedWithJoinBridges inserts intermediate tables on shortest FK paths so high-scoring
// picks that were disconnected (e.g. sales header + production productcategory) become one component.
func expandSelectedWithJoinBridges(
	selected []tableBundle,
	relations []metadata.Relation,
	idx tableIndex,
	maxTables int,
) []tableBundle {
	if len(selected) == 0 {
		return nil
	}
	adj := relationAdjacency(relations)
	keyOf := func(t metadata.Table) string { return tableKey(t.SchemaName, t.TableName) }

	included := make(map[string]tableBundle)
	list := make([]tableBundle, 0, maxTables)

	put := func(b tableBundle) {
		k := keyOf(b.table)
		if existing, ok := included[k]; ok {
			if b.score > existing.score {
				for i := range list {
					if keyOf(list[i].table) == k {
						list[i] = b
						break
					}
				}
				included[k] = b
			}
			return
		}
		if len(list) >= maxTables {
			return
		}
		included[k] = b
		list = append(list, b)
	}

	put(selected[0])
	for i := 1; i < len(selected); i++ {
		tb := selected[i]
		tkey := keyOf(tb.table)
		from := make(map[string]struct{}, len(list))
		for _, b := range list {
			from[keyOf(b.table)] = struct{}{}
		}
		if _, ok := from[tkey]; ok {
			put(tb)
			continue
		}
		path := shortestPathFromSet(adj, from, tkey)
		if path == nil {
			continue
		}
		for _, pkey := range path {
			if len(list) >= maxTables {
				break
			}
			tbl, ok := idx.byFullName[pkey]
			if !ok {
				continue
			}
			bundle := tableBundle{table: tbl, score: 0}
			if pkey == tkey {
				bundle = tb
			}
			put(bundle)
		}
	}
	return list
}

func directRelation(
	left metadata.Table,
	right metadata.Table,
	relations []metadata.Relation,
) (metadata.Relation, bool) {
	leftKey := tableKey(left.SchemaName, left.TableName)
	rightKey := tableKey(right.SchemaName, right.TableName)
	for _, rel := range relations {
		fromKey := tableKey(rel.FromSchema, rel.FromTable)
		toKey := tableKey(rel.ToSchema, rel.ToTable)
		if (fromKey == leftKey && toKey == rightKey) || (fromKey == rightKey && toKey == leftKey) {
			return rel, true
		}
	}
	return metadata.Relation{}, false
}

func relationPath(rel metadata.Relation) string {
	return fmt.Sprintf(
		"%s.%s.%s = %s.%s.%s",
		rel.FromSchema,
		rel.FromTable,
		rel.FromColumn,
		rel.ToSchema,
		rel.ToTable,
		rel.ToColumn,
	)
}

type tableIndex struct {
	byFullName map[string]metadata.Table
	byName     map[string][]metadata.Table
}

func indexTables(tables []metadata.Table) tableIndex {
	idx := tableIndex{
		byFullName: make(map[string]metadata.Table, len(tables)),
		byName:     make(map[string][]metadata.Table),
	}
	for _, table := range tables {
		fullName := tableKey(table.SchemaName, table.TableName)
		idx.byFullName[fullName] = table
		idx.byName[table.TableName] = append(idx.byName[table.TableName], table)
	}
	return idx
}

// validateManualScopeAgainstTypeScope checks manual table refs against the full datasource
// catalog so we return a clear error when the user picks a base table while "views only" (or vice versa),
// even if type filtering would remove all tables.
func validateManualScopeAgainstTypeScope(
	idx tableIndex,
	tableScope []string,
	includeBaseTables bool,
	includeViews bool,
) error {
	for _, ref := range nonEmptyScope(tableScope) {
		table, err := resolveTableRef(idx, ref)
		if err != nil {
			return fmt.Errorf("%w: %v", ErrTableScopeInvalid, err)
		}
		if !tableMatchesTypeScope(table, includeBaseTables, includeViews) {
			return fmt.Errorf("%w: %q is excluded by type scope (base tables vs views)", ErrTableScopeInvalid, ref)
		}
	}
	return nil
}

func resolveTableRef(idx tableIndex, ref string) (metadata.Table, error) {
	ref = strings.TrimSpace(ref)
	if table, ok := idx.byFullName[ref]; ok {
		return table, nil
	}
	if matches := idx.byName[ref]; len(matches) == 1 {
		return matches[0], nil
	}
	if matches := idx.byName[ref]; len(matches) > 1 {
		return metadata.Table{}, fmt.Errorf("table %q is ambiguous; use schema.table", ref)
	}
	return metadata.Table{}, fmt.Errorf("table %q not found", ref)
}

func groupColumnsByTable(columns []metadata.Column) map[string][]metadata.Column {
	grouped := make(map[string][]metadata.Column)
	for _, col := range columns {
		key := tableKey(col.SchemaName, col.TableName)
		grouped[key] = append(grouped[key], col)
	}
	return grouped
}

func columnNameCounts(selected []tableBundle, columnsByTable map[string][]metadata.Column) map[string]int {
	counts := make(map[string]int)
	for _, bundle := range selected {
		for _, col := range columnsByTable[tableKey(bundle.table.SchemaName, bundle.table.TableName)] {
			counts[col.ColumnName]++
		}
	}
	return counts
}

func topCandidates(bundles []tableBundle, limit int) []TableCandidate {
	if len(bundles) < limit {
		limit = len(bundles)
	}
	candidates := make([]TableCandidate, 0, limit)
	for _, bundle := range bundles[:limit] {
		candidate := TableCandidate{
			Table:          tableLabel(bundle.table),
			Score:          roundScore(bundle.score),
			TotalScore:     roundScore(bundle.score),
			KeywordScore:   roundScore(bundle.keywordScore),
			EmbeddingScore: roundScore(bundle.embeddingScore),
		}
		if bundle.table.Description != nil {
			candidate.Description = *bundle.table.Description
		}
		candidates = append(candidates, candidate)
	}
	return candidates
}

func roundScore(score float64) float64 {
	return math.Round(score*100) / 100
}

func markSelectedCandidates(candidates []TableCandidate, selectedTables []string) {
	selected := make(map[string]bool, len(selectedTables))
	for _, table := range selectedTables {
		selected[table] = true
	}
	for i := range candidates {
		if selected[candidates[i].Table] {
			candidates[i].Selected = true
			continue
		}
		candidates[i].RejectedReason = "not selected for final connected context"
	}
}

func eliminatedCandidateLabels(candidates []TableCandidate) []string {
	var out []string
	for _, candidate := range candidates {
		if !candidate.Selected {
			out = append(out, candidate.Table)
		}
	}
	return out
}

func applyModelContextToRouting(
	result *TableRoutingResult,
	model *semantic.SemanticModel,
	contextSource string,
	updatedAt *time.Time,
) {
	if result == nil || model == nil {
		return
	}
	result.ContextSource = contextSource
	result.ContextKey = model.ID
	if result.ContextKey == "" {
		result.ContextKey = model.Name
	}
	if model.Name != "" {
		result.SelectedModels = []string{model.Name}
	}
	if updatedAt != nil {
		result.ContextUpdatedAt = updatedAt
	} else if !model.UpdatedAt.IsZero() {
		t := model.UpdatedAt
		result.ContextUpdatedAt = &t
	}
	result.SelectedDimensions = dimensionNames(model.Dimensions)
	result.SelectedMetrics = metricNames(model.Metrics)
}

func dimensionNames(dimensions []semantic.Dimension) []string {
	names := make([]string, 0, len(dimensions))
	for _, dimension := range dimensions {
		names = append(names, dimension.Name)
	}
	return names
}

func metricNames(metrics []semantic.Metric) []string {
	names := make([]string, 0, len(metrics))
	for _, metric := range metrics {
		names = append(names, metric.Name)
	}
	return names
}

func bundleKeySet(bundles []tableBundle) map[string]bool {
	keys := make(map[string]bool, len(bundles))
	for _, bundle := range bundles {
		keys[tableKey(bundle.table.SchemaName, bundle.table.TableName)] = true
	}
	return keys
}

func addedBundleLabels(before map[string]bool, bundles []tableBundle) []string {
	var added []string
	for _, bundle := range bundles {
		label := tableLabel(bundle.table)
		if before[label] {
			continue
		}
		added = append(added, label)
	}
	return added
}

func routeConfidence(bundles []tableBundle) float64 {
	if len(bundles) == 0 || bundles[0].score == 0 {
		return 0
	}
	second := 0.0
	if len(bundles) > 1 {
		second = bundles[1].score
	}
	confidence := bundles[0].score / (bundles[0].score + second + 2)
	return math.Round(confidence*100) / 100
}

func bundleLabels(bundles []tableBundle) []string {
	labels := make([]string, 0, len(bundles))
	for _, bundle := range bundles {
		labels = append(labels, tableLabel(bundle.table))
	}
	return labels
}

func nonEmptyScope(scope []string) []string {
	filtered := make([]string, 0, len(scope))
	for _, ref := range scope {
		if strings.TrimSpace(ref) != "" {
			filtered = append(filtered, strings.TrimSpace(ref))
		}
	}
	return filtered
}

func tableLabel(table metadata.Table) string {
	return tableKey(table.SchemaName, table.TableName)
}

func tableKey(schemaName, tableName string) string {
	return schemaName + "." + tableName
}

func columnKey(schemaName, tableName, columnName string) string {
	return schemaName + "." + tableName + "." + columnName
}

func tableMatchesTypeScope(table metadata.Table, includeBase, includeViews bool) bool {
	typ := strings.ToUpper(strings.TrimSpace(table.TableType))
	switch typ {
	case "VIEW":
		return includeViews
	case "BASE TABLE":
		return includeBase
	default:
		// Empty or unknown: treat like a physical table (older syncs).
		return includeBase
	}
}

func filterTablesByTypeScope(tables []metadata.Table, includeBase, includeViews bool) []metadata.Table {
	out := make([]metadata.Table, 0, len(tables))
	for _, t := range tables {
		if tableMatchesTypeScope(t, includeBase, includeViews) {
			out = append(out, t)
		}
	}
	return out
}

func filterColumnsForTables(columns []metadata.Column, tables []metadata.Table) []metadata.Column {
	allowed := make(map[string]struct{}, len(tables))
	for _, t := range tables {
		allowed[tableKey(t.SchemaName, t.TableName)] = struct{}{}
	}
	out := make([]metadata.Column, 0, len(columns))
	for _, c := range columns {
		if _, ok := allowed[tableKey(c.SchemaName, c.TableName)]; ok {
			out = append(out, c)
		}
	}
	return out
}

func weightedTokenScore(questionTokens map[string]bool, text string, weight float64) float64 {
	textTokens := tokenSet(text)
	score := 0.0
	for token := range questionTokens {
		if textTokens[token] {
			score += weight
		}
	}
	return score
}

func tokenSet(text string) map[string]bool {
	normalized := normalizeText(text)
	tokens := make(map[string]bool)
	for _, token := range strings.Fields(normalized) {
		for _, expanded := range expandToken(token) {
			tokens[expanded] = true
		}
	}
	return tokens
}

func expandToken(token string) []string {
	if token == "" {
		return nil
	}
	expanded := []string{token}
	if strings.HasSuffix(token, "ies") && len(token) > 3 {
		expanded = append(expanded, strings.TrimSuffix(token, "ies")+"y")
	}
	if strings.HasSuffix(token, "s") && len(token) > 3 {
		expanded = append(expanded, strings.TrimSuffix(token, "s"))
	}
	expanded = append(expanded, activeRoutingLexicon().ExpandTokenSynonyms(token)...)
	return expanded
}

func normalizeText(text string) string {
	replacer := strings.NewReplacer(
		"İ", "i",
		"I", "i",
		"ı", "i",
		"Ş", "s",
		"ş", "s",
		"Ğ", "g",
		"ğ", "g",
		"Ü", "u",
		"ü", "u",
		"Ö", "o",
		"ö", "o",
		"Ç", "c",
		"ç", "c",
	)
	text = strings.ToLower(replacer.Replace(text))
	var sb strings.Builder
	for _, r := range text {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			sb.WriteRune(r)
			continue
		}
		sb.WriteRune(' ')
	}
	return sb.String()
}

func isRevenueLikeQuestion(tokens map[string]bool) bool {
	return activeRoutingLexicon().HasAnyToken(tokens, activeRoutingLexicon().RevenueTokens)
}

func isRevenueLikeColumn(col metadata.Column) bool {
	return activeRoutingLexicon().HasAnyToken(tokenSet(col.ColumnName), activeRoutingLexicon().RevenueColumnTokens)
}

func tableNameMatchesSubstrings(tableName string, substrings []string) bool {
	tn := strings.ToLower(tableName)
	for _, sub := range substrings {
		if strings.Contains(tn, sub) {
			return true
		}
	}
	return false
}

func dimensionType(dataType string) string {
	t := strings.ToLower(dataType)
	switch {
	case strings.Contains(t, "date"), strings.Contains(t, "time"):
		return string(semantic.DimensionTypeDate)
	case strings.Contains(t, "bool"):
		return string(semantic.DimensionTypeBoolean)
	case isNumericType(t):
		return string(semantic.DimensionTypeNumber)
	default:
		return string(semantic.DimensionTypeText)
	}
}

func isNumericType(dataType string) bool {
	t := strings.ToLower(dataType)
	for _, marker := range []string{
		"int",
		"numeric",
		"decimal",
		"double",
		"float",
		"real",
		"money",
		"number",
	} {
		if strings.Contains(t, marker) {
			return true
		}
	}
	return false
}

func isDateOrTimeType(dataType string) bool {
	t := strings.ToLower(dataType)
	return strings.Contains(t, "date") || strings.Contains(t, "time")
}

// hasTimeComponent reports whether a date/time column carries clock time, so
// hour-grain bucketing is meaningful. Pure DATE columns get only y/q/m/d
// variants; TIMESTAMP / DATETIME / TIME-typed columns also get _hour.
func hasTimeComponent(dataType string) bool {
	t := strings.ToLower(dataType)
	if strings.Contains(t, "timestamp") || strings.Contains(t, "datetime") {
		return true
	}
	// "time without time zone" / "time with time zone" / "time" — has clock,
	// but no calendar; skip hour as a calendar bucket anyway. Only true
	// "date" rejects.
	if strings.Contains(t, "date") && !strings.Contains(t, "datetime") {
		return false
	}
	return strings.Contains(t, "time")
}
