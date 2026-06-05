package ai

import (
	"context"
	"fmt"
	"strings"

	"github.com/biqly/biqly/internal/ai/lingua"
	"github.com/biqly/biqly/internal/i18n"
	"github.com/biqly/biqly/internal/metadata"
)

// MetadataWriter is the subset of metadata.Repository the EmbedMetadataService
// relies on for storing computed embeddings.
type MetadataWriter interface {
	ListTables(ctx context.Context, datasourceID, schemaName string) ([]metadata.Table, error)
	ListColumns(ctx context.Context, datasourceID, schemaName, tableName string) ([]metadata.Column, error)
	UpsertTableEmbedding(ctx context.Context, tableID, modelName string, embedding []float32) error
	UpsertColumnEmbedding(ctx context.Context, columnID, modelName string, embedding []float32) error
	ApplyTableTranslations(ctx context.Context, tables []metadata.Table, loc i18n.Locale) error
	ApplyColumnTranslations(ctx context.Context, cols []metadata.Column, loc i18n.Locale) error
}

// EmbedMetadataService computes table and column embeddings from datasource
// metadata, then persists them for use by the vector-aware table router.
type EmbedMetadataService struct {
	embedder    Embedder
	writer      MetadataWriter
	denySchemas map[string]struct{}
	denyTables  map[string]struct{} // key: "schema.table"
}

// NewEmbedMetadataService wires the embedder and metadata repository.
func NewEmbedMetadataService(embedder Embedder, writer MetadataWriter) *EmbedMetadataService {
	return &EmbedMetadataService{embedder: embedder, writer: writer}
}

// WithDeniedSchemas excludes every table in the listed schemas from being
// sent to the external embedding API. Use for schemas that contain regulated
// data (PII, financial records) that must not leave the cluster even as
// column/table identifier strings.
func (s *EmbedMetadataService) WithDeniedSchemas(schemas []string) *EmbedMetadataService {
	if len(schemas) == 0 {
		return s
	}
	if s.denySchemas == nil {
		s.denySchemas = make(map[string]struct{}, len(schemas))
	}
	for _, name := range schemas {
		name = strings.ToLower(strings.TrimSpace(name))
		if name != "" {
			s.denySchemas[name] = struct{}{}
		}
	}
	return s
}

// WithDeniedTables excludes specific "schema.table" pairs from embedding.
// Schemas use WithDeniedSchemas; this is the finer-grained per-table opt-out.
func (s *EmbedMetadataService) WithDeniedTables(tables []string) *EmbedMetadataService {
	if len(tables) == 0 {
		return s
	}
	if s.denyTables == nil {
		s.denyTables = make(map[string]struct{}, len(tables))
	}
	for _, name := range tables {
		name = strings.ToLower(strings.TrimSpace(name))
		if name != "" {
			s.denyTables[name] = struct{}{}
		}
	}
	return s
}

// isDeniedTable reports whether the given (schema, table) pair has been
// opted out of embedding by config (denied schema or denied table).
func (s *EmbedMetadataService) isDeniedTable(schema, table string) bool {
	sLower := strings.ToLower(strings.TrimSpace(schema))
	if _, ok := s.denySchemas[sLower]; ok {
		return true
	}
	if _, ok := s.denyTables[sLower+"."+strings.ToLower(strings.TrimSpace(table))]; ok {
		return true
	}
	return false
}

const metadataEmbeddingBatchSize = 96

// EmbedTableResult is one entry in the response — schema/table/column
// identifier plus the outcome (embedded or skipped+reason). Kind is "table" or
// "column"; Column is empty for table rows.
type EmbedTableResult struct {
	Schema  string `json:"schema"`
	Table   string `json:"table"`
	Column  string `json:"column,omitempty"`
	Kind    string `json:"kind"`
	Skipped bool   `json:"skipped,omitempty"`
	Reason  string `json:"reason,omitempty"`
}

// EmbedAllForDatasource computes embeddings for every table and column in the
// datasource and writes them to the metadata DB.
func (s *EmbedMetadataService) EmbedAllForDatasource(ctx context.Context, datasourceID string) ([]EmbedTableResult, error) {
	return s.embedForFilter(ctx, datasourceID, nil)
}

// EmbedForTables embeds only the tables/columns whose schema.table key is in
// the provided allowlist. Used to scope embedding work to one semantic model.
func (s *EmbedMetadataService) EmbedForTables(ctx context.Context, datasourceID string, allowed map[string]bool) ([]EmbedTableResult, error) {
	if len(allowed) == 0 {
		return nil, nil
	}
	return s.embedForFilter(ctx, datasourceID, allowed)
}

//nolint:gocognit
func (s *EmbedMetadataService) embedForFilter(ctx context.Context, datasourceID string, allowed map[string]bool) ([]EmbedTableResult, error) {
	tables, err := s.writer.ListTables(ctx, datasourceID, "")
	if err != nil {
		return nil, fmt.Errorf("list tables: %w", err)
	}
	cols, err := s.writer.ListColumns(ctx, datasourceID, "", "")
	if err != nil {
		return nil, fmt.Errorf("list columns: %w", err)
	}
	if allowed != nil {
		filteredTables := make([]metadata.Table, 0, len(tables))
		for i := range tables {
			t := tables[i]
			if allowed[t.SchemaName+"."+t.TableName] {
				filteredTables = append(filteredTables, t)
			}
		}
		tables = filteredTables
		filteredCols := make([]metadata.Column, 0, len(cols))
		for _, c := range cols {
			if allowed[c.SchemaName+"."+c.TableName] {
				filteredCols = append(filteredCols, c)
			}
		}
		cols = filteredCols
	}
	if len(s.denySchemas) > 0 || len(s.denyTables) > 0 {
		filteredTables := make([]metadata.Table, 0, len(tables))
		for i := range tables {
			t := tables[i]
			if !s.isDeniedTable(t.SchemaName, t.TableName) {
				filteredTables = append(filteredTables, t)
			}
		}
		tables = filteredTables
		filteredCols := make([]metadata.Column, 0, len(cols))
		for _, c := range cols {
			if !s.isDeniedTable(c.SchemaName, c.TableName) {
				filteredCols = append(filteredCols, c)
			}
		}
		cols = filteredCols
	}
	if len(tables) == 0 {
		return nil, nil
	}

	locales := embeddingLocalesWritten()
	results := make([]EmbedTableResult, 0, (len(tables)+len(cols))*len(locales))
	for _, loc := range locales {
		locTables := tables
		locCols := cols
		profile, _ := i18n.LocaleProfileFor(loc)
		if profile.UsesMetadataTranslations {
			locTables = append([]metadata.Table(nil), tables...)
			locCols = append([]metadata.Column(nil), cols...)
			if err := s.writer.ApplyTableTranslations(ctx, locTables, loc); err != nil {
				return nil, fmt.Errorf("apply table translations (%s): %w", loc, err)
			}
			if err := s.writer.ApplyColumnTranslations(ctx, locCols, loc); err != nil {
				return nil, fmt.Errorf("apply column translations (%s): %w", loc, err)
			}
		}
		locColsByTable := groupColumnsByTableKey(locCols)
		if tableResults, err := s.embedTables(ctx, locTables, locColsByTable, loc); err != nil {
			return nil, err
		} else {
			results = append(results, tableResults...)
		}
		if columnResults, err := s.embedColumns(ctx, locCols, loc); err != nil {
			return nil, err
		} else {
			results = append(results, columnResults...)
		}
	}
	return results, nil
}

func (s *EmbedMetadataService) embedTables(ctx context.Context, tables []metadata.Table, colsByTable map[string][]metadata.Column, loc i18n.Locale) ([]EmbedTableResult, error) {
	texts := make([]string, 0, len(tables))
	refs := make([]metadata.Table, 0, len(tables))
	for i := range tables {
		key := tables[i].SchemaName + "." + tables[i].TableName
		txt := buildTableEmbeddingText(&tables[i], colsByTable[key])
		if strings.TrimSpace(txt) == "" {
			continue
		}
		texts = append(texts, txt)
		refs = append(refs, tables[i])
	}

	vectors, err := embedInBatches(ctx, s.embedder, texts, metadataEmbeddingBatchSize)
	if err != nil {
		return nil, fmt.Errorf("embed tables: %w", err)
	}

	model := lingua.EmbeddingModelForLocale(s.embedder.Model(), loc)
	results := make([]EmbedTableResult, 0, len(refs))
	for i, t := range refs {
		results = append(results, EmbedTableResult{Schema: t.SchemaName, Table: t.TableName, Kind: "table"})
		if i >= len(vectors) || len(vectors[i]) == 0 {
			results[len(results)-1].Skipped = true
			results[len(results)-1].Reason = "no embedding produced"
			continue
		}
		if err := s.writer.UpsertTableEmbedding(ctx, t.ID, model, vectors[i]); err != nil {
			results[len(results)-1].Skipped = true
			results[len(results)-1].Reason = "store failed: " + err.Error()
			continue
		}
	}
	return results, nil
}

func (s *EmbedMetadataService) embedColumns(ctx context.Context, cols []metadata.Column, loc i18n.Locale) ([]EmbedTableResult, error) {
	texts := make([]string, 0, len(cols))
	refs := make([]metadata.Column, 0, len(cols))
	for i := range cols {
		txt := buildColumnEmbeddingText(&cols[i])
		if strings.TrimSpace(txt) == "" {
			continue
		}
		texts = append(texts, txt)
		refs = append(refs, cols[i])
	}

	vectors, err := embedInBatches(ctx, s.embedder, texts, metadataEmbeddingBatchSize)
	if err != nil {
		return nil, fmt.Errorf("embed columns: %w", err)
	}

	model := lingua.EmbeddingModelForLocale(s.embedder.Model(), loc)
	results := make([]EmbedTableResult, 0, len(refs))
	for i, c := range refs {
		results = append(results, EmbedTableResult{Schema: c.SchemaName, Table: c.TableName, Column: c.ColumnName, Kind: "column"})
		if i >= len(vectors) || len(vectors[i]) == 0 {
			results[len(results)-1].Skipped = true
			results[len(results)-1].Reason = "no embedding produced"
			continue
		}
		if err := s.writer.UpsertColumnEmbedding(ctx, c.ID, model, vectors[i]); err != nil {
			results[len(results)-1].Skipped = true
			results[len(results)-1].Reason = "store failed: " + err.Error()
			continue
		}
	}
	return results, nil
}

func embedInBatches(ctx context.Context, embedder Embedder, texts []string, batchSize int) ([][]float32, error) {
	if len(texts) == 0 {
		return nil, nil
	}
	if batchSize <= 0 {
		batchSize = len(texts)
	}
	out := make([][]float32, 0, len(texts))
	for start := 0; start < len(texts); start += batchSize {
		end := start + batchSize
		if end > len(texts) {
			end = len(texts)
		}
		batch, err := embedder.Embed(ctx, texts[start:end])
		if err != nil {
			return nil, err
		}
		out = append(out, batch...)
	}
	return out, nil
}

// buildTableEmbeddingText assembles the natural-language summary we feed to the
// embedder for a single table: schema-qualified name, optional description,
// and a compact column list (names + types). Kept short on purpose — table
// retrieval doesn't need full data — so embedding cost stays bounded even
// for catalogs with hundreds of columns per table.
func buildTableEmbeddingText(t *metadata.Table, cols []metadata.Column) string {
	var sb strings.Builder
	_, _ = fmt.Fprintf(&sb, "Table: %s.%s", t.SchemaName, t.TableName)
	if t.Description != nil && strings.TrimSpace(*t.Description) != "" {
		fmt.Fprintf(&sb, "\nDescription: %s", strings.TrimSpace(*t.Description))
	}
	if len(cols) > 0 {
		sb.WriteString("\nColumns: ")
		const maxCols = 60 // cap for very wide tables
		for i, c := range cols {
			if i >= maxCols {
				fmt.Fprintf(&sb, ", … (+%d more)", len(cols)-maxCols)
				break
			}
			if i > 0 {
				sb.WriteString(", ")
			}
			fmt.Fprintf(&sb, "%s (%s)", c.ColumnName, c.DataType)
		}
	}
	return sb.String()
}

// buildColumnEmbeddingText assembles a compact semantic description for one
// column. Structural flags stay out of the text except FK target because ranking
// heuristics already use PK/FK/date/type from metadata.
func buildColumnEmbeddingText(c *metadata.Column) string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "Column: %s.%s.%s", c.SchemaName, c.TableName, c.ColumnName)
	fmt.Fprintf(&sb, "\nTable: %s.%s", c.SchemaName, c.TableName)
	fmt.Fprintf(&sb, "\nData type: %s", c.DataType)
	if c.Description != nil && strings.TrimSpace(*c.Description) != "" {
		fmt.Fprintf(&sb, "\nDescription: %s", strings.TrimSpace(*c.Description))
	}
	if c.ReferencedTable != nil && c.ReferencedColumn != nil && *c.ReferencedTable != "" && *c.ReferencedColumn != "" {
		refSchema := c.SchemaName
		if c.ReferencedSchema != nil && *c.ReferencedSchema != "" {
			refSchema = *c.ReferencedSchema
		}
		fmt.Fprintf(&sb, "\nReferences: %s.%s.%s", refSchema, *c.ReferencedTable, *c.ReferencedColumn)
	}
	return sb.String()
}

func groupColumnsByTableKey(cols []metadata.Column) map[string][]metadata.Column {
	out := make(map[string][]metadata.Column)
	for _, c := range cols {
		k := c.SchemaName + "." + c.TableName
		out[k] = append(out[k], c)
	}
	return out
}
