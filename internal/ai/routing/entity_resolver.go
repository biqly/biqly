package routing

import (
	"slices"
	"sort"
	"strings"

	"github.com/biqly/biqly/internal/metadata"
)

func appendQuestionEntityTables(
	selected []tableBundle,
	tables []metadata.Table,
	relations []metadata.Relation,
	idx tableIndex,
	tokens map[string]struct{},
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
		if _, ok := selectedKeys[key]; ok || !tableMatchesQuestionTokens(t, tokens) {
			continue
		}
		path := shortestPathFromSet(adj, from, key)
		if path == nil || len(path) > maxHops+1 {
			continue
		}
		selected, selectedKeys, from = appendEntityPathTables(selected, selectedKeys, from, idx, path, key, maxN)
		if len(selected) >= maxN {
			return selected
		}
	}
	return selected
}

func tableMatchesQuestionTokens(t metadata.Table, tokens map[string]struct{}) bool {
	for tok := range tokenSet(t.TableName) {
		if len(tok) < 3 {
			continue
		}
		if _, ok := tokens[tok]; ok {
			return true
		}
	}
	return false
}

func appendEntityPathTables(
	selected []tableBundle,
	selectedKeys, from map[string]struct{},
	idx tableIndex,
	path []string,
	targetKey string,
	maxN int,
) ([]tableBundle, map[string]struct{}, map[string]struct{}) {
	w := activeRoutingWeights()
	for i := 1; i < len(path); i++ {
		if len(selected) >= maxN {
			return selected, selectedKeys, from
		}
		pkey := path[i]
		if _, ok := selectedKeys[pkey]; ok {
			continue
		}
		pt, ok := idx.byFullName[pkey]
		if !ok {
			continue
		}
		score := w.EntityPathBridgeScore
		if pkey == targetKey {
			score = w.EntityPathTargetScore
		}
		selected = append(selected, tableBundle{table: pt, score: score})
		selectedKeys[pkey] = struct{}{}
		from[pkey] = struct{}{}
	}
	return selected, selectedKeys, from
}

// appendEntityResolverTables ensures questions like "<entity> name" can be
// answered by pulling in the entity table itself plus any downstream
// display-name table reached through FK chains, even when those tables didn't
// score in the initial top picks (or are views without FK metadata to begin
// with). Workflow:
//
//  1. Filter the question tokens to entity-like tokens (drop "name", "id"…).
//  2. BFS from the current selection to find entity-token-matching tables
//     within maxHops (entityCandidate.hops).
//  3. Pass 1 — add each entity table itself plus any FK bridges on the path.
//  4. Pass 2 — for each entity in the set that lacks a display-name column,
//     walk further hops to find a display-name-bearing partner table.
//
// The three phases live in their own functions below so each stage can be
// reasoned about in isolation.
func appendEntityResolverTables(
	selected []tableBundle,
	columnsByTable map[string][]metadata.Column,
	relations []metadata.Relation,
	idx tableIndex,
	tokens map[string]struct{},
	maxN, maxHops int,
) []tableBundle {
	if !wantsReadableLabelsQuestion(tokens) {
		return selected
	}
	entityTokens := entityTokensFromQuestion(tokens)
	if len(entityTokens) == 0 {
		return selected
	}

	selectedKeys := make(map[string]struct{})
	for _, b := range selected {
		selectedKeys[tableKey(b.table.SchemaName, b.table.TableName)] = struct{}{}
	}
	adj := relationAdjacency(relations)

	entityCands := findEntityCandidates(idx, adj, selectedKeys, entityTokens, maxHops)

	// Pass 1: include each entity table and the FK bridges on its path.
	for _, c := range entityCands {
		if len(selected) >= maxN {
			break
		}
		selected, selectedKeys = addPathToTarget(selected, selectedKeys, idx, adj, c.key, maxN)
	}

	// Pass 2: for each entity in the set lacking a display-name column, walk
	// further to find a display-name-bearing partner.
	selected, _ = addDisplayPartners(selected, selectedKeys, columnsByTable, idx, adj, entityTokens, entityCands, maxN, maxHops)
	return selected
}

// entityTokensFromQuestion drops "name"-like tokens ("name", "names",
// "fullname", "label") so what remains identifies the entity ("customer",
// "product") rather than the field being requested.
func entityTokensFromQuestion(tokens map[string]struct{}) map[string]struct{} {
	out := make(map[string]struct{}, len(tokens))
	for tok := range tokens {
		if isNameLikeToken(tok) {
			continue
		}
		out[tok] = struct{}{}
	}
	return out
}

// entityCandidate names a table the BFS reached that matches an entity token,
// along with how many FK hops away it was — closer wins ties downstream.
type entityCandidate struct {
	key  string
	hops int
}

// findEntityCandidates BFS-walks the FK graph from every currently selected
// table up to maxHops, recording tables whose names (or singular forms) match
// an entity token. The result is sorted by hops so the closest hits are
// considered first.
func findEntityCandidates(
	idx tableIndex,
	adj map[string][]string,
	selectedKeys map[string]struct{},
	entityTokens map[string]struct{},
	maxHops int,
) []entityCandidate {
	visited := make(map[string]int, len(selectedKeys))
	queue := make([]string, 0, len(selectedKeys))
	for k := range selectedKeys {
		visited[k] = 0
		queue = append(queue, k)
	}
	var cands []entityCandidate
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
			_, hasToken := entityTokens[tname]
			_, hasSingular := entityTokens[singularize(tname)]
			if hasToken || hasSingular {
				cands = append(cands, entityCandidate{key: nb, hops: d + 1})
			}
		}
	}
	sort.SliceStable(cands, func(i, j int) bool { return cands[i].hops < cands[j].hops })
	return cands
}

// addPathToTarget extends selected with every table on the shortest FK path
// from the existing selection set to targetKey, including the target itself.
// Already-selected tables are skipped. selectedKeys is updated in place so
// repeated calls share work.
func addPathToTarget(
	selected []tableBundle,
	selectedKeys map[string]struct{},
	idx tableIndex,
	adj map[string][]string,
	targetKey string,
	maxN int,
) ([]tableBundle, map[string]struct{}) {
	from := make(map[string]struct{}, len(selectedKeys))
	for k := range selectedKeys {
		from[k] = struct{}{}
	}
	path := shortestPathFromSet(adj, from, targetKey)
	if path == nil {
		return selected, selectedKeys
	}
	w := activeRoutingWeights()
	for i := 1; i < len(path); i++ {
		if len(selected) >= maxN {
			return selected, selectedKeys
		}
		pkey := path[i]
		if _, ok := selectedKeys[pkey]; ok {
			continue
		}
		t, ok := idx.byFullName[pkey]
		if !ok {
			continue
		}
		score := w.ResolverPathBridgeScore
		if pkey == targetKey {
			score = w.ResolverPathTargetScore
		}
		selected = append(selected, tableBundle{table: t, score: score})
		selectedKeys[pkey] = struct{}{}
	}
	return selected, selectedKeys
}

// addDisplayPartners finds, for each entity table in the current set lacking a
// display-name column, the closest FK-reachable table that DOES have one, and
// adds the path to selected. Avoids running display lookup twice per entity.
func addDisplayPartners(
	selected []tableBundle,
	selectedKeys map[string]struct{},
	columnsByTable map[string][]metadata.Column,
	idx tableIndex,
	adj map[string][]string,
	entityTokens map[string]struct{},
	entityCands []entityCandidate,
	maxN, maxHops int,
) ([]tableBundle, map[string]struct{}) {
	entityKeys := make([]string, 0, len(entityCands)+len(selected))
	for _, b := range selected {
		bk := tableKey(b.table.SchemaName, b.table.TableName)
		tname := strings.ToLower(b.table.TableName)
		_, hasToken := entityTokens[tname]
		_, hasSingular := entityTokens[singularize(tname)]
		if hasToken || hasSingular {
			entityKeys = append(entityKeys, bk)
		}
	}
	for _, c := range entityCands {
		entityKeys = append(entityKeys, c.key)
	}

	visitedEntity := make(map[string]struct{}, len(entityKeys))
	for _, ek := range entityKeys {
		if len(selected) >= maxN {
			break
		}
		if _, ok := visitedEntity[ek]; ok {
			continue
		}
		visitedEntity[ek] = struct{}{}
		if _, ok := selectedKeys[ek]; !ok {
			continue
		}
		if hasDisplayNameInColumns(columnsByTable[ek]) {
			continue
		}
		if bestKey := nearestDisplayPartner(ek, adj, columnsByTable, maxHops); bestKey != "" {
			selected, selectedKeys = addPathToTarget(selected, selectedKeys, idx, adj, bestKey, maxN)
		}
	}
	return selected, selectedKeys
}

// nearestDisplayPartner walks the FK graph outward from startKey and returns
// the first table whose columns include a display-name column. Empty string
// when no such partner exists within maxHops.
func nearestDisplayPartner(
	startKey string,
	adj map[string][]string,
	columnsByTable map[string][]metadata.Column,
	maxHops int,
) string {
	eVisited := map[string]int{startKey: 0}
	eQueue := []string{startKey}
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
	return bestKey
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
	return slices.Contains(lex.NameLikeTokens, tok)
}

func bundleSliceContains(bundles []tableBundle, key string) bool {
	for _, b := range bundles {
		if tableKey(b.table.SchemaName, b.table.TableName) == key {
			return true
		}
	}
	return false
}

func wantsReadableLabelsQuestion(tokens map[string]struct{}) bool {
	return activeRoutingLexicon().HasAnyToken(tokens, activeRoutingLexicon().ReadableLabelTokens)
}
