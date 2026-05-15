package query

import (
	"strings"

	"github.com/biqly/biqly/internal/dialect"
	"github.com/biqly/biqly/internal/semantic"
)

// ParsedColumnRef is a decomposed physical column reference.
type ParsedColumnRef struct {
	Schema string
	Table  string
	Column string
}

// TableKey returns a stable schema.table identifier for join graphs.
func TableKey(schema, table string) string {
	schema = strings.TrimSpace(schema)
	table = strings.TrimSpace(table)
	if schema == "" {
		return table
	}
	return schema + "." + table
}

// ParseColumnRef splits table.column or schema.table.column using defaultSchema for two-part refs.
func ParseColumnRef(ref, defaultSchema string) (ParsedColumnRef, bool) {
	parts := splitDot(ref)
	switch len(parts) {
	case 3:
		return ParsedColumnRef{parts[0], parts[1], parts[2]}, true
	case 2:
		return ParsedColumnRef{defaultSchema, parts[0], parts[1]}, true
	default:
		return ParsedColumnRef{}, false
	}
}

// SchemaResolver resolves schemas for tables and column references during compilation.
type SchemaResolver struct {
	modelBaseSchema string
	defaultSchema   string
	tableSchemas    map[string]string
}

// NewSchemaResolver builds a resolver from the semantic model and optional logical query overrides.
func NewSchemaResolver(model *semantic.SemanticModel, lq *LogicalQuery) *SchemaResolver {
	r := &SchemaResolver{
		modelBaseSchema: model.BaseSchema,
		tableSchemas:    make(map[string]string),
	}
	if lq != nil {
		r.defaultSchema = strings.TrimSpace(lq.DefaultSchema)
		for table, schema := range lq.TableSchemas {
			if schema = strings.TrimSpace(schema); schema != "" {
				r.tableSchemas[table] = schema
			}
		}
	}
	return r
}

// EffectiveDefaultSchema returns the schema used for two-part column references.
func (r *SchemaResolver) EffectiveDefaultSchema() string {
	if r.defaultSchema != "" {
		return r.defaultSchema
	}
	return r.modelBaseSchema
}

// SchemaForTable returns the schema for a physical table name.
func (r *SchemaResolver) SchemaForTable(table string) string {
	if s, ok := r.tableSchemas[table]; ok && s != "" {
		return s
	}
	return r.EffectiveDefaultSchema()
}

// ParseColumnRef parses a column reference using the resolver default schema.
func (r *SchemaResolver) ParseColumnRef(ref string) (ParsedColumnRef, bool) {
	return ParseColumnRef(ref, r.EffectiveDefaultSchema())
}

// PhysicalColumnRef returns a dialect-quotable column path. Uses schema.table.column
// when the table schema differs from the model base (cross-schema); otherwise table.column.
func (r *SchemaResolver) PhysicalColumnRef(ref string) string {
	p, ok := r.ParseColumnRef(ref)
	if !ok {
		return ref
	}
	if override, ok := r.tableSchemas[p.Table]; ok && override != "" {
		p.Schema = override
	}
	if p.Schema != r.EffectiveDefaultSchema() {
		return TableKey(p.Schema, p.Table) + "." + p.Column
	}
	return p.Table + "." + p.Column
}

// QualifyColumn returns dialect-quoted schema.table.column SQL.
func (r *SchemaResolver) QualifyColumn(d dialect.Dialect, ref string) string {
	return d.QuoteIdent(r.PhysicalColumnRef(ref))
}

// QualifyTable returns dialect-quoted schema.table SQL.
func (r *SchemaResolver) QualifyTable(d dialect.Dialect, schema, table string) string {
	if schema == "" {
		if s, t := splitQualifiedTableName(table); s != "" {
			return d.QuoteIdent(TableKey(s, t))
		}
		schema = r.SchemaForTable(table)
		table = strings.TrimSpace(table)
	}
	return d.QuoteIdent(TableKey(schema, table))
}

// JoinSideKey returns the physical table key for a join endpoint.
func (r *SchemaResolver) JoinSideKey(schema, table string) string {
	if schema == "" {
		if s, t := splitQualifiedTableName(table); s != "" {
			return TableKey(s, t)
		}
		schema = r.SchemaForTable(table)
	} else if s, t := splitQualifiedTableName(table); s != "" {
		return TableKey(s, t)
	}
	return TableKey(schema, table)
}

func splitQualifiedTableName(table string) (schema, name string) {
	parts := strings.Split(strings.TrimSpace(table), ".")
	if len(parts) == 2 {
		return parts[0], parts[1]
	}
	return "", strings.TrimSpace(table)
}

func splitDot(s string) []string {
	var result []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '.' {
			result = append(result, s[start:i])
			start = i + 1
		}
	}
	result = append(result, s[start:])
	return result
}
