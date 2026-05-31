package routing

import (
	"fmt"

	"github.com/biqly/biqly/internal/metadata"
	"github.com/biqly/biqly/internal/semantic"
)

func buildJoins(selected []tableBundle, relations []metadata.Relation) []semantic.Join {
	if len(selected) < 2 {
		return nil
	}
	allKeys := make(map[string]bool, len(selected))
	for _, bundle := range selected {
		allKeys[tableKey(bundle.table.SchemaName, bundle.table.TableName)] = true
	}

	seen := make(map[string]bool)
	joins := make([]semantic.Join, 0, len(relations))
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

	connected := make([]tableBundle, 1, len(selected))
	connected[0] = selected[0]
	remaining := make([]tableBundle, 0, len(selected)-1)
	remaining = append(remaining, selected[1:]...)
	joinPaths := make([]string, 0, len(selected)-1)

	for len(remaining) > 0 {
		added := false
		still := make([]tableBundle, 0, len(remaining))
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
	adj := make(map[string][]string, len(relations)*2)
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
