package ai

import (
	"context"
	"errors"
	"fmt"
	"math"
	"sort"
	"strings"
	"unicode"

	"github.com/biqly/biqly/internal/metadata"
	"github.com/biqly/biqly/internal/semantic"
)

const (
	autoModelPrefix           = "auto:"
	maxAutoSelectedTables     = 6
	maxExpandedAutoTables     = 12 // after scoring: add FK bridge tables so picks form one component
	nameResolverMaxHops       = 3  // max FK hops to follow when resolving "<entity> name" questions
	minRouteConfidence        = 0.35
	// Limits for auto-generated semantic models (avoids multi-hundred-column explosion in LLM prompts).
	maxAutoModelDimensions = 150
	maxAutoModelMetrics    = 120
	maxDateGrainExtras     = 36 // year/quarter/month variants per date columns (cap total)
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

// EmbeddingReader returns previously-computed table embeddings for a
// datasource. Tables without an embedding (never embedded, or embedded with
// a different model) are simply absent from the result. Implementations may
// also be nil — the router falls back to keyword-only scoring in that case.
type EmbeddingReader interface {
	ListTableEmbeddings(ctx context.Context, datasourceID string) ([]metadata.TableEmbedding, error)
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
	}
}

// TableCandidate is a scored table candidate returned by automatic routing.
type TableCandidate struct {
	Table       string  `json:"table"`
	Score       float64 `json:"score"`
	Description string  `json:"description,omitempty"`
}

// TableRoutingResult describes the table-routing decision for an AI query.
type TableRoutingResult struct {
	SelectedTables     []string         `json:"selected_tables,omitempty"`
	JoinPaths          []string         `json:"join_paths,omitempty"`
	Candidates         []TableCandidate `json:"candidates,omitempty"`
	Confidence         float64          `json:"confidence"`
	NeedsClarification bool             `json:"needs_clarification"`
	Manual             bool             `json:"manual"`
	// RankingMethod tells the frontend which signal drove this decision:
	// "manual" when the user supplied a scope, "hybrid" when both keyword
	// and embedding similarity contributed, otherwise "keyword".
	RankingMethod string `json:"ranking_method,omitempty"`
}

type tableBundle struct {
	table metadata.Table
	score float64
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

	// Hybrid boost from precomputed table embeddings, when configured. Skipped
	// silently on any error so a transient embedding-API failure or missing
	// vectors falls back cleanly to keyword scoring.
	embedBoost := r.embeddingBoost(ctx, datasourceID, question)

	selected, result, err := r.selectTables(tables, columnsByTable, question, tableScope, embedBoost)
	if err != nil {
		return nil, result, err
	}
	if result.RankingMethod == "" {
		switch {
		case result.Manual:
			result.RankingMethod = "manual"
		case len(embedBoost) > 0:
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
		selected = expandSelectedWithJoinBridges(selected, relations, tblIdx, maxExpandedAutoTables)
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

	model := buildSemanticModel(datasourceID, connected, columnsByTable, relations)
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

// embeddingBoost returns a per-tableKey cosine-similarity score scaled by the
// configured weight. Returns nil (no boost) when embeddings aren't configured
// or any step fails — keyword-only scoring still works.
func (r *TableRouter) embeddingBoost(ctx context.Context, datasourceID, question string) map[string]float64 {
	if r.embedder == nil || r.embeddingReader == nil || r.embeddingWeight <= 0 {
		return nil
	}
	stored, err := r.embeddingReader.ListTableEmbeddings(ctx, datasourceID)
	if err != nil || len(stored) == 0 {
		return nil
	}
	qVecs, err := r.embedder.Embed(ctx, []string{question})
	if err != nil || len(qVecs) == 0 || len(qVecs[0]) == 0 {
		return nil
	}
	q := qVecs[0]
	out := make(map[string]float64, len(stored))
	for _, te := range stored {
		sim := CosineSimilarity(q, te.Embedding)
		if sim <= 0 {
			continue
		}
		out[tableKey(te.SchemaName, te.TableName)] = sim * r.embeddingWeight
	}
	return out
}

func selectManualTables(
	tables []metadata.Table,
	tableScope []string,
) ([]tableBundle, *TableRoutingResult, error) {
	tableIndex := indexTables(tables)
	var selected []tableBundle
	seen := make(map[string]bool)

	for _, ref := range nonEmptyScope(tableScope) {
		table, err := resolveTableRef(tableIndex, ref)
		if err != nil {
			result := &TableRoutingResult{Manual: true, NeedsClarification: true}
			return nil, result, fmt.Errorf("%w: %w", ErrTableScopeInvalid, err)
		}
		key := tableKey(table.SchemaName, table.TableName)
		if seen[key] {
			continue
		}
		seen[key] = true
		selected = append(selected, tableBundle{
			table: table,
			score: 1,
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
		score := scoreTable(table, columnsByTable[key], tokens)
		if boost, ok := embedBoost[key]; ok {
			score += boost
		}
		bundles = append(bundles, tableBundle{
			table: table,
			score: score,
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
		if bundle.score >= bundles[0].score*0.30 || bundle.score >= 5 {
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
	for _, b := range selected {
		if strings.Contains(strings.ToLower(b.table.TableName), "category") {
			return selected
		}
	}
	pick := func() (tableBundle, bool) {
		for _, b := range bundles {
			if b.score == 0 {
				continue
			}
			tn := strings.ToLower(b.table.TableName)
			if !strings.Contains(tn, "category") {
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

	selectedKeys := make(map[string]bool)
	for _, b := range selected {
		selectedKeys[tableKey(b.table.SchemaName, b.table.TableName)] = true
	}

	adj := relationAdjacency(relations)

	addPathTo := func(targetKey string) {
		from := make(map[string]bool, len(selectedKeys))
		for k := range selectedKeys {
			from[k] = true
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
			if selectedKeys[pkey] {
				continue
			}
			t, ok := idx.byFullName[pkey]
			if !ok {
				continue
			}
			score := 0.4
			if pkey == targetKey {
				score = 1.0
			}
			selected = append(selected, tableBundle{table: t, score: score})
			selectedKeys[pkey] = true
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
			if selectedKeys[nb] {
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
		if !selectedKeys[ek] {
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
	switch tok {
	case "name", "names", "named", "title", "titles", "label", "labels",
		"isim", "ismi", "adi", "ad", "unvan", "baslik", "basligi":
		return true
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
	for t := range tokens {
		switch strings.ToLower(strings.TrimSpace(t)) {
		case "name", "names", "named", "title", "titles", "label", "labels",
			"isim", "adı", "adi", "ad", "unvan", "başlık", "baslik":
			return true
		}
	}
	return false
}

func scoreTable(table metadata.Table, columns []metadata.Column, tokens map[string]bool) float64 {
	score := weightedTokenScore(tokens, table.SchemaName+" "+table.TableName, 5)
	if table.Description != nil {
		score += weightedTokenScore(tokens, *table.Description, 1.5)
	}
	tn := strings.ToLower(table.TableName)
	if isCategoryOrProductQuestion(tokens) {
		if strings.Contains(tn, "category") || strings.Contains(tn, "subcategor") {
			score += 14
		}
		if tn == "product" || strings.Contains(tn, "productcategory") || strings.Contains(tn, "productsubcategory") {
			score += 8
		}
		if tn == "salesorderdetail" || strings.Contains(tn, "salesorderdetail") {
			score += 12
		}
	}
	if isCategoryOrProductQuestion(tokens) && isQuantityOrCountIntent(tokens) {
		// Line items / order detail: quantity sold, "adet", top products by count.
		if strings.Contains(tn, "orderdetail") || strings.Contains(tn, "order_detail") ||
			strings.Contains(tn, "orderline") || strings.Contains(tn, "order_line") {
			score += 10
		}
	}
	for _, col := range columns {
		score += weightedTokenScore(tokens, col.ColumnName, 2)
		score += weightedTokenScore(tokens, col.DataType, 0.2)
		if col.Description != nil {
			score += weightedTokenScore(tokens, *col.Description, 1)
		}
		if isRevenueLikeQuestion(tokens) && isRevenueLikeColumn(col) {
			score += 2
		}
	}
	if wantsReadableLabelsQuestion(tokens) {
		for _, col := range columns {
			if isDisplayNameColumn(col.ColumnName) {
				score += 5
				break
			}
		}
	}
	return score
}

func isCategoryOrProductQuestion(tokens map[string]bool) bool {
	for _, t := range []string{
		"category", "categories", "kategori", "kategoriler",
		"product", "products", "item", "items",
		"urun", "urunu", "urunun", "urunler", "urunleri", "urunden",
	} {
		if tokens[t] {
			return true
		}
	}
	return false
}

// isQuantityOrCountIntent matches questions about counts, units, or order quantity
// (including Turkish "adet", "miktar") after token expansion.
func isQuantityOrCountIntent(tokens map[string]bool) bool {
	for _, t := range []string{
		"quantity", "qty", "count", "rows", "row", "units", "unit",
		"adet", "miktar", "miktari", "miktarda", "adette",
		"pieces", "piece",
	} {
		if tokens[t] {
			return true
		}
	}
	return false
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
	for _, b := range selected {
		tn := strings.ToLower(b.table.TableName)
		if tn == "product" || strings.Contains(tn, "productsubcategor") {
			return selected
		}
	}
	pick := func() (tableBundle, bool) {
		for _, b := range bundles {
			if b.score == 0 {
				continue
			}
			tn := strings.ToLower(b.table.TableName)
			if tn != "product" && !strings.Contains(tn, "productsubcategor") {
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
) *semantic.SemanticModel {
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

	model.Dimensions = buildDimensions(selected, columnsByTable)
	model.Metrics = buildMetrics(selected, columnsByTable)
	model.Joins = buildJoins(selected, relations)
	return model
}

func buildDimensions(selected []tableBundle, columnsByTable map[string][]metadata.Column) []semantic.Dimension {
	nameCounts := columnNameCounts(selected, columnsByTable)
	pairs := sortedBundleColumns(selected, columnsByTable)
	if len(pairs) > maxAutoModelDimensions {
		pairs = pairs[:maxAutoModelDimensions]
	}
	dimensions := make([]semantic.Dimension, 0, len(pairs))
	dateGrainAdded := 0
	for _, p := range pairs {
		if len(dimensions) >= maxAutoModelDimensions {
			break
		}
		name := p.col.ColumnName
		if nameCounts[p.col.ColumnName] > 1 {
			name = p.bundle.table.TableName + "_" + p.col.ColumnName
		}
		colRef := p.bundle.table.TableName + "." + p.col.ColumnName
		dimensions = append(dimensions, semantic.Dimension{
			Name:        name,
			ColumnRef:   colRef,
			Type:        dimensionType(p.col.DataType),
			Description: p.col.Description,
			Synonyms:    displayNameSynonyms(p.bundle.table.TableName, p.col.ColumnName),
			IsActive:    true,
		})
		if !isDateOrTimeType(p.col.DataType) || dateGrainAdded >= maxDateGrainExtras {
			continue
		}
		for _, g := range []struct {
			part, suffix string
			syns         []string
		}{
			{"year", "_year", []string{"year", "years", "yearly", "annual", "yıl", "yıllık", "per year", "by year"}},
			{"quarter", "_quarter", []string{"quarter", "quarters", "qtr", "çeyrek"}},
			{"month", "_month", []string{"month", "months", "monthly", "ay", "aylık", "per month", "by month"}},
		} {
			if len(dimensions) >= maxAutoModelDimensions || dateGrainAdded >= maxDateGrainExtras {
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
	for _, syn := range tokenSynonyms[base] {
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

func buildMetrics(selected []tableBundle, columnsByTable map[string][]metadata.Column) []semantic.Metric {
	metrics := []semantic.Metric{{
		Name:        "row_count",
		Expression:  "*",
		Aggregation: string(semantic.AggCount),
		Synonyms:    []string{"count", "rows", "total", "adet", "sayisi", "sayısı", "kaç", "kac"},
		IsActive:    true,
	}}

	nameCounts := columnNameCounts(selected, columnsByTable)
	pairs := sortedBundleColumns(selected, columnsByTable)

	appendMetric := func(m semantic.Metric) {
		if len(metrics) >= maxAutoModelMetrics {
			return
		}
		metrics = append(metrics, m)
	}

	for _, p := range pairs {
		if len(metrics) >= maxAutoModelMetrics {
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
			appendMetric(metric("avg_"+name, expression, semantic.AggAvg, col.Description, nil))
			appendMetric(metric("min_"+name, expression, semantic.AggMin, col.Description, minNumericSynonyms))
			appendMetric(metric("max_"+name, expression, semantic.AggMax, col.Description, maxNumericSynonyms))
		case isDateOrTimeType(col.DataType):
			appendMetric(metric("min_"+name, expression, semantic.AggMin, col.Description, minDateSynonyms))
			appendMetric(metric("max_"+name, expression, semantic.AggMax, col.Description, maxDateSynonyms))
		}
	}
	return metrics
}

var (
	minNumericSynonyms = []string{"min", "minimum", "lowest", "smallest", "en az", "en kucuk"}
	maxNumericSynonyms = []string{"max", "maximum", "highest", "largest", "en cok", "en buyuk"}
	minDateSynonyms    = []string{"earliest", "first", "oldest", "ilk", "en eski", "en erken"}
	maxDateSynonyms    = []string{"latest", "last", "most recent", "newest", "son", "en son", "en yeni", "son tarih"}
)

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
			FromTable:    rel.FromTable,
			FromColumn:   rel.FromColumn,
			ToTable:      rel.ToTable,
			ToColumn:     rel.ToColumn,
			JoinType:     "LEFT",
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
func shortestPathFromSet(adj map[string][]string, from map[string]bool, to string) []string {
	if from[to] {
		return []string{to}
	}
	queue := make([]string, 0, len(from))
	parent := make(map[string]string)
	seen := make(map[string]bool)
	for k := range from {
		queue = append(queue, k)
		seen[k] = true
		parent[k] = ""
	}
	for qi := 0; qi < len(queue); qi++ {
		cur := queue[qi]
		for _, nb := range adj[cur] {
			if seen[nb] {
				continue
			}
			seen[nb] = true
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
		from := make(map[string]bool, len(list))
		for _, b := range list {
			from[keyOf(b.table)] = true
		}
		if from[tkey] {
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
			return fmt.Errorf("%w: %w", ErrTableScopeInvalid, err)
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
			Table: tableLabel(bundle.table),
			Score: math.Round(bundle.score*100) / 100,
		}
		if bundle.table.Description != nil {
			candidate.Description = *bundle.table.Description
		}
		candidates = append(candidates, candidate)
	}
	return candidates
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
	expanded = append(expanded, tokenSynonyms[token]...)
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

var tokenSynonyms = map[string][]string{
	"adet":     {"count", "row", "rows", "quantity", "qty"},
	"adette":   {"count", "quantity", "adet"},
	"bazinda":  {"by", "per", "basis"},
	"bazli":    {"based", "by"},
	"category": {"categories", "kategori", "kategoriler", "class", "group"},
	"categories": {"category", "kategori"},
	"amount":   {"total", "revenue", "sales"},
	"avg":      {"average", "mean", "ortalama"},
	"average":  {"avg", "ortalama"},
	"customer": {"client", "musteri"},
	"gelir":    {"revenue", "sales", "amount", "total"},
	"kac":      {"count", "row", "rows"},
	"miktar":   {"quantity", "qty", "count", "amount"},
	"miktari":  {"quantity", "qty", "miktar"},
	"miktarda": {"quantity", "miktar"},
	"musteri":  {"customer", "client"},
	"order":    {"purchase", "sale", "siparis"},
	"ortalama": {"avg", "average"},
	"price":    {"amount", "total", "revenue", "sales"},
	"purchase": {"order", "sale"},
	"revenue":  {"amount", "total", "sales", "price"},
	"sale":     {"order", "revenue", "amount", "total"},
	"sales":    {"order", "revenue", "amount", "total"},
	"satis":    {"sales", "sale", "revenue", "amount", "total"},
	"sayisi":   {"count", "row", "rows"},
	"siparis":  {"order", "sale", "purchase"},
	"satan":    {"sale", "sales", "sold", "selling"},
	"total":    {"amount", "revenue", "sales"},
	"urun":     {"product", "item", "products"},
	"urunden":  {"urun", "product", "item"},
	"urunler":  {"urun", "product", "products", "items"},
	"urunleri": {"urun", "product", "products"},
	"urunun":   {"urun", "product"},
	"urunu":    {"urun", "product", "products", "item"},
}

func isRevenueLikeQuestion(tokens map[string]bool) bool {
	for _, token := range []string{"revenue", "sales", "sale", "amount", "total", "gelir", "satis"} {
		if tokens[token] {
			return true
		}
	}
	return false
}

func isRevenueLikeColumn(col metadata.Column) bool {
	tokens := tokenSet(col.ColumnName)
	for _, token := range []string{"amount", "total", "price", "revenue", "sales"} {
		if tokens[token] {
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
