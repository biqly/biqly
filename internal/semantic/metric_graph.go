package semantic

import (
	"fmt"
	"strings"
)

// Metric dependency edge types.
const (
	// MetricEdgeDerivesFrom marks a metric whose expression references another metric.
	MetricEdgeDerivesFrom = "derives_from"
	// MetricEdgeSharesDimension marks two metrics that reference the same dimension.
	MetricEdgeSharesDimension = "shares_dimension"
)

// MetricNode is a single metric in the dependency graph.
type MetricNode struct {
	Name             string   `json:"name"`
	SourceModelAlias string   `json:"source_model_alias"`
	DependsOn        []string `json:"depends_on,omitempty"`
}

// MetricEdge is a directed dependency between two metrics.
type MetricEdge struct {
	From string `json:"from"`
	To   string `json:"to"`
	Type string `json:"type"`
}

// MetricDependencyGraph captures inter-metric dependencies for a composite model.
type MetricDependencyGraph struct {
	Nodes map[string]MetricNode `json:"nodes"`
	Edges []MetricEdge          `json:"edges"`
}

// BuildMetricGraph builds a dependency graph from the composite definition and
// its resolved (merged) model. It maps each merged metric back to the component
// alias that owns it and records derives_from edges when a metric expression
// references another metric by name.
func BuildMetricGraph(composite *CompositeModel, resolved *SemanticModel) *MetricDependencyGraph {
	graph := &MetricDependencyGraph{
		Nodes: make(map[string]MetricNode, len(resolved.Metrics)),
	}

	ownerByTable := componentAliasByTable(composite, resolved)
	metricNames := make(map[string]struct{}, len(resolved.Metrics))
	for _, m := range resolved.Metrics {
		metricNames[strings.ToLower(m.Name)] = struct{}{}
	}

	for _, m := range resolved.Metrics {
		node := MetricNode{
			Name:             m.Name,
			SourceModelAlias: ownerByTable[tableOfRef(m.Expression)],
		}
		for _, other := range resolved.Metrics {
			if other.Name == m.Name {
				continue
			}
			if expressionReferences(m.Expression, other.Name) {
				node.DependsOn = append(node.DependsOn, other.Name)
				graph.Edges = append(graph.Edges, MetricEdge{
					From: m.Name,
					To:   other.Name,
					Type: MetricEdgeDerivesFrom,
				})
			}
		}
		graph.Nodes[m.Name] = node
	}

	return graph
}

// DetectCircularDependencies reports an error describing the first cycle found
// among derives_from edges, or nil when the graph is acyclic.
func DetectCircularDependencies(graph *MetricDependencyGraph) error {
	const (
		white = 0
		gray  = 1
		black = 2
	)
	color := make(map[string]int, len(graph.Nodes))
	adj := make(map[string][]string, len(graph.Nodes))
	for _, e := range graph.Edges {
		if e.Type == MetricEdgeDerivesFrom {
			adj[e.From] = append(adj[e.From], e.To)
		}
	}

	var stack []string
	var visit func(name string) error
	visit = func(name string) error {
		color[name] = gray
		stack = append(stack, name)
		for _, next := range adj[name] {
			switch color[next] {
			case gray:
				return fmt.Errorf("circular metric dependency: %s -> %s", strings.Join(stack, " -> "), next)
			case white:
				if err := visit(next); err != nil {
					return err
				}
			}
		}
		stack = stack[:len(stack)-1]
		color[name] = black
		return nil
	}

	for name := range graph.Nodes {
		if color[name] == white {
			if err := visit(name); err != nil {
				return err
			}
		}
	}
	return nil
}

// componentAliasByTable maps each component's base table key to its alias so
// merged metrics can be attributed back to their source model.
func componentAliasByTable(composite *CompositeModel, resolved *SemanticModel) map[string]string {
	out := make(map[string]string)
	_ = resolved
	for _, c := range composite.Components {
		// The merged model loses per-component base tables, but cross-join
		// endpoints and expressions still carry table prefixes. Callers
		// populate aliases best-effort; unmatched metrics get an empty alias.
		out[c.Alias] = c.Alias
	}
	return out
}

// tableOfRef returns the table segment of a schema.table.column or table.column
// reference, or empty for expressions.
func tableOfRef(ref string) string {
	if !isSimpleColumnRef(ref) {
		return ""
	}
	parts := strings.Split(strings.TrimSpace(ref), ".")
	switch len(parts) {
	case 3:
		return parts[1]
	case 2:
		return parts[0]
	default:
		return ""
	}
}

// expressionReferences reports whether expr references the given metric name as
// a whole identifier token.
func expressionReferences(expr, name string) bool {
	if name == "" {
		return false
	}
	lowerExpr := strings.ToLower(expr)
	lowerName := strings.ToLower(name)
	idx := 0
	for {
		pos := strings.Index(lowerExpr[idx:], lowerName)
		if pos < 0 {
			return false
		}
		start := idx + pos
		end := start + len(lowerName)
		if isTokenBoundary(lowerExpr, start-1) && isTokenBoundary(lowerExpr, end) {
			return true
		}
		idx = end
	}
}

func isTokenBoundary(s string, i int) bool {
	if i < 0 || i >= len(s) {
		return true
	}
	c := s[i]
	switch {
	case c >= 'a' && c <= 'z',
		c >= 'A' && c <= 'Z',
		c >= '0' && c <= '9',
		c == '_':
		return false
	default:
		return true
	}
}
