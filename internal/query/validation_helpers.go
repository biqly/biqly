package query

import (
	"slices"
	"strings"

	"github.com/biqly/biqly/internal/semantic"
)

const maxFieldSuggestions = 3

func validationErrWithCode(field, message, code, value string, allowed []string) error {
	return ValidationErrors{&ValidationError{
		Field:               field,
		Message:             message,
		Code:                code,
		Value:               value,
		AllowedAlternatives: allowed,
	}}
}

func getDimensionNames(model *semantic.SemanticModel) []string {
	if model == nil {
		return nil
	}
	names := make([]string, 0, len(model.Dimensions))
	for _, d := range model.Dimensions {
		names = append(names, d.Name)
	}
	return names
}

func getMetricNames(model *semantic.SemanticModel) []string {
	if model == nil {
		return nil
	}
	names := make([]string, 0, len(model.Metrics))
	for _, m := range model.Metrics {
		names = append(names, m.Name)
	}
	return names
}

func getAllFieldNames(model *semantic.SemanticModel) []string {
	if model == nil {
		return nil
	}
	names := make([]string, 0, len(model.Dimensions)+len(model.Metrics))
	for _, d := range model.Dimensions {
		names = append(names, d.Name)
	}
	for _, m := range model.Metrics {
		names = append(names, m.Name)
	}
	return names
}

// suggestAlternatives returns the top candidates closest to the unknown string.
func suggestAlternatives(unknown string, candidates []string) []string {
	if len(candidates) == 0 {
		return nil
	}
	type candidateWithDist struct {
		name string
		dist int
	}
	list := make([]candidateWithDist, 0, len(candidates))
	unknownLower := strings.ToLower(unknown)
	for _, c := range candidates {
		cLower := strings.ToLower(c)
		d := levenshteinDistance(unknownLower, cLower)
		// Give a small bonus (lower distance) if one contains the other
		if strings.Contains(cLower, unknownLower) || strings.Contains(unknownLower, cLower) {
			d -= 1
		}
		list = append(list, candidateWithDist{name: c, dist: d})
	}
	slices.SortFunc(list, func(a, b candidateWithDist) int {
		if a.dist != b.dist {
			return a.dist - b.dist
		}
		return strings.Compare(a.name, b.name)
	})

	limit := maxFieldSuggestions
	if len(list) < limit {
		limit = len(list)
	}
	res := make([]string, 0, limit)
	for i := 0; i < limit; i++ {
		// Only suggest if reasonably similar, or it's the absolute best recommendation
		if list[i].dist <= 5 || i == 0 {
			res = append(res, list[i].name)
		}
	}
	return res
}

func levenshteinDistance(s, t string) int {
	if s == t {
		return 0
	}
	if len(s) == 0 {
		return len(t)
	}
	if len(t) == 0 {
		return len(s)
	}
	v0 := make([]int, len(t)+1)
	v1 := make([]int, len(t)+1)
	for i := 0; i <= len(t); i++ {
		v0[i] = i
	}
	for i := 0; i < len(s); i++ {
		v1[0] = i + 1
		for j := 0; j < len(t); j++ {
			cost := 1
			if s[i] == t[j] {
				cost = 0
			}
			v1[j+1] = min(v1[j]+1, v0[j+1]+1)
			if v0[j]+cost < v1[j+1] {
				v1[j+1] = v0[j] + cost
			}
		}
		copy(v0, v1)
	}
	return v0[len(t)]
}
