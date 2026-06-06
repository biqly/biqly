package routing

import (
	"sort"
	"strings"

	"github.com/biqly/biqly/internal/metadata"
)

const (
	schemaPartitionScoreRatio = 0.30
	maxActiveSchemaPartitions = 2
	minSchemasToPartition     = 2
)

// filterTablesBySchemaCluster keeps tables in the schema(s) most relevant to the
// question when the datasource spans multiple schemas. Returns the filtered
// tables and the active schema names (for routing debug).
func filterTablesBySchemaCluster(
	tables []metadata.Table,
	columnsByTable map[string][]metadata.Column,
	relations []metadata.Relation,
	question string,
	embedBoost map[string]float64,
) ([]metadata.Table, []string) {
	bySchema := groupTablesBySchema(tables)
	if len(bySchema) < minSchemasToPartition {
		return tables, nil
	}

	tokens := tokenSet(question)
	type scored struct {
		name  string
		score float64
	}
	ranked := make([]scored, 0, len(bySchema))
	for schema, schemaTables := range bySchema {
		var best float64
		for _, t := range schemaTables {
			key := tableKey(t.SchemaName, t.TableName)
			s := scoreTable(t, columnsByTable[key], tokens) + embedBoost[key]
			if s > best {
				best = s
			}
		}
		ranked = append(ranked, scored{name: schema, score: best})
	}
	sort.Slice(ranked, func(i, j int) bool {
		if ranked[i].score != ranked[j].score {
			return ranked[i].score > ranked[j].score
		}
		return ranked[i].name < ranked[j].name
	})
	if len(ranked) == 0 || ranked[0].score <= 0 {
		return tables, nil
	}

	top := ranked[0].score
	active := map[string]struct{}{ranked[0].name: {}}
	for i := 1; i < len(ranked) && len(active) < maxActiveSchemaPartitions; i++ {
		if ranked[i].score >= top*schemaPartitionScoreRatio {
			active[ranked[i].name] = struct{}{}
		}
	}
	expandSchemaPartitionWithFK(active, relations)

	var out []metadata.Table
	names := make([]string, 0, len(active))
	for schema, schemaTables := range bySchema {
		if _, ok := active[schema]; !ok {
			continue
		}
		names = append(names, schema)
		out = append(out, schemaTables...)
	}
	sort.Strings(names)
	sort.SliceStable(out, func(i, j int) bool {
		return tableLabel(out[i]) < tableLabel(out[j])
	})
	return out, names
}

// expandSchemaPartitionWithFK keeps tables in schemas linked by FK to an active
// schema so cross-schema join paths (e.g. sales.customer → person.person) stay
// reachable for entity resolution.
func expandSchemaPartitionWithFK(active map[string]struct{}, relations []metadata.Relation) {
	for {
		changed := false
		for _, rel := range relations {
			from := normalizeSchemaName(rel.FromSchema)
			to := normalizeSchemaName(rel.ToSchema)
			_, hasFrom := active[from]
			_, hasTo := active[to]
			if hasFrom && !hasTo {
				active[to] = struct{}{}
				changed = true
			}
			if hasTo && !hasFrom {
				active[from] = struct{}{}
				changed = true
			}
		}
		if !changed {
			return
		}
	}
}

func normalizeSchemaName(schema string) string {
	schema = strings.TrimSpace(schema)
	if schema == "" {
		return "public"
	}
	return schema
}

func groupTablesBySchema(tables []metadata.Table) map[string][]metadata.Table {
	out := make(map[string][]metadata.Table)
	for _, t := range tables {
		out[normalizeSchemaName(t.SchemaName)] = append(out[normalizeSchemaName(t.SchemaName)], t)
	}
	return out
}
