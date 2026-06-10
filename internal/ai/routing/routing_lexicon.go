package routing

import (
	"fmt"
	"log/slog"
	"maps"
	"os"
	"sync"
	"time"

	"github.com/biqly/biqly/internal/ai/lexicon"
	"github.com/bytedance/sonic"
)

// Lexicon RoutingLexicon holds token expansion and intent vocabulary for table routing.
// Defaults are embedded; override with BI_AI_ROUTING_LEXICON_PATH (JSON).
type Lexicon struct {
	TokenSynonyms            map[string][]string `json:"token_synonyms"`
	CategoryProductTokens    []string            `json:"category_product_tokens"`
	QuantityTokens           []string            `json:"quantity_tokens"`
	RevenueTokens            []string            `json:"revenue_tokens"`
	RevenueColumnTokens      []string            `json:"revenue_column_tokens"`
	NameLikeTokens           []string            `json:"name_like_tokens"`
	ReadableLabelTokens      []string            `json:"readable_label_tokens"`
	RowCountSynonyms         []string            `json:"row_count_synonyms"`
	MetricSynonyms           map[string][]string `json:"metric_synonyms"`
	CategoryTableSubstrings  []string            `json:"category_table_substrings"`
	ProductCatalogSubstrings []string            `json:"product_catalog_substrings"`
}

// routingLexiconOverlayTTL bounds how long a cached merge of the base lexicon
// with the DB-backed NL lexicon overlay is served before re-merging. Stacked
// with the lexicon store's own TTL, cross-replica convergence after an admin
// write is at most the sum of the two windows (ADR-0001 K6).
const routingLexiconOverlayTTL = 30 * time.Second

var (
	routingLexiconMu     sync.Mutex
	baseRoutingLexicon   *Lexicon
	baseRoutingLoaded    bool
	errRoutingLexicon    error
	mergedRoutingLexicon *Lexicon
	mergedRoutingExpires time.Time
)

// ActiveRoutingLexicon returns the active routing lexicon: embedded defaults,
// optional file override (BI_AI_ROUTING_LEXICON_PATH), and the DB-backed NL
// lexicon overlay (token_synonym / metric_synonym domains) merged on top.
func ActiveRoutingLexicon() (*Lexicon, error) {
	routingLexiconMu.Lock()
	defer routingLexiconMu.Unlock()
	if !baseRoutingLoaded {
		baseRoutingLexicon, errRoutingLexicon = loadRoutingLexicon("")
		baseRoutingLoaded = true
	}
	if errRoutingLexicon != nil {
		return nil, errRoutingLexicon
	}
	if mergedRoutingLexicon != nil && time.Now().Before(mergedRoutingExpires) {
		return mergedRoutingLexicon, nil
	}
	mergedRoutingLexicon = overlayRoutingLexicon(baseRoutingLexicon)
	mergedRoutingExpires = time.Now().Add(routingLexiconOverlayTTL)
	return mergedRoutingLexicon, nil
}

// overlayRoutingLexicon merges DB-managed token/metric synonyms onto a copy of
// the base lexicon. Per-key entries replace the base entry (mergeRoutingLexicon
// semantics); with no DB rows the base is returned as-is.
func overlayRoutingLexicon(base *Lexicon) *Lexicon {
	tokenOverlay := lexicon.Active().DomainTerms(lexicon.DomainTokenSynonym)
	metricOverlay := lexicon.Active().DomainTerms(lexicon.DomainMetricSynonym)
	if len(tokenOverlay) == 0 && len(metricOverlay) == 0 {
		return base
	}
	merged := *base
	merged.TokenSynonyms = overlaySynonymMap(base.TokenSynonyms, tokenOverlay)
	merged.MetricSynonyms = overlaySynonymMap(base.MetricSynonyms, metricOverlay)
	return &merged
}

func overlaySynonymMap(base, overlay map[string][]string) map[string][]string {
	if len(overlay) == 0 {
		return base
	}
	out := make(map[string][]string, len(base)+len(overlay))
	maps.Copy(out, base)
	maps.Copy(out, overlay)
	return out
}

// InvalidateRoutingLexicon drops the cached overlay merge so the next lookup
// re-reads the NL lexicon. Admin lexicon writes call this on the writing
// replica; others converge within the TTL windows.
func InvalidateRoutingLexicon() {
	routingLexiconMu.Lock()
	mergedRoutingLexicon = nil
	mergedRoutingExpires = time.Time{}
	routingLexiconMu.Unlock()
}

func activeRoutingLexicon() *Lexicon {
	lex, err := ActiveRoutingLexicon()
	if err != nil {
		slog.Error("routing lexicon unavailable, using embedded defaults", "error", err)
		fallback, parseErr := parseRoutingLexiconJSON(embeddedRoutingLexiconJSON)
		if parseErr != nil {
			slog.Error("embedded routing lexicon invalid", "error", parseErr)
			return &Lexicon{}
		}
		return fallback
	}
	return lex
}

// InitRoutingLexicon loads routing vocabulary from an optional JSON file path.
// Empty path keeps embedded defaults. Call once at process startup.
func InitRoutingLexicon(path string) error {
	lex, err := loadRoutingLexicon(path)
	if err != nil {
		return err
	}
	routingLexiconMu.Lock()
	baseRoutingLexicon = lex
	baseRoutingLoaded = true
	errRoutingLexicon = nil
	mergedRoutingLexicon = nil
	mergedRoutingExpires = time.Time{}
	routingLexiconMu.Unlock()
	return nil
}

func loadRoutingLexicon(path string) (*Lexicon, error) {
	lex, err := parseRoutingLexiconJSON(embeddedRoutingLexiconJSON)
	if err != nil {
		return nil, fmt.Errorf("embedded routing lexicon: %w", err)
	}
	if path == "" {
		return lex, nil
	}
	raw, err := os.ReadFile(path) //nolint:gosec // operator-provided routing lexicon path
	if err != nil {
		return nil, fmt.Errorf("read routing lexicon %q: %w", path, err)
	}
	override, err := parseRoutingLexiconJSON(raw)
	if err != nil {
		return nil, fmt.Errorf("parse routing lexicon %q: %w", path, err)
	}
	mergeRoutingLexicon(lex, override)
	return lex, nil
}

func parseRoutingLexiconJSON(raw []byte) (*Lexicon, error) {
	var lex Lexicon
	if err := sonic.ConfigStd.Unmarshal(raw, &lex); err != nil {
		return nil, err
	}
	if lex.TokenSynonyms == nil {
		lex.TokenSynonyms = map[string][]string{}
	}
	if lex.MetricSynonyms == nil {
		lex.MetricSynonyms = map[string][]string{}
	}
	return &lex, nil
}

func mergeRoutingLexicon(base, override *Lexicon) {
	if len(override.TokenSynonyms) > 0 {
		for k, v := range override.TokenSynonyms {
			base.TokenSynonyms[k] = v
		}
	}
	mergeStringSliceField(&base.CategoryProductTokens, override.CategoryProductTokens)
	mergeStringSliceField(&base.QuantityTokens, override.QuantityTokens)
	mergeStringSliceField(&base.RevenueTokens, override.RevenueTokens)
	mergeStringSliceField(&base.RevenueColumnTokens, override.RevenueColumnTokens)
	mergeStringSliceField(&base.NameLikeTokens, override.NameLikeTokens)
	mergeStringSliceField(&base.ReadableLabelTokens, override.ReadableLabelTokens)
	mergeStringSliceField(&base.RowCountSynonyms, override.RowCountSynonyms)
	mergeStringSliceField(&base.CategoryTableSubstrings, override.CategoryTableSubstrings)
	mergeStringSliceField(&base.ProductCatalogSubstrings, override.ProductCatalogSubstrings)
	if len(override.MetricSynonyms) > 0 {
		for k, v := range override.MetricSynonyms {
			base.MetricSynonyms[k] = v
		}
	}
}

func mergeStringSliceField(dst *[]string, src []string) {
	if len(src) > 0 {
		*dst = src
	}
}

func (*Lexicon) HasAnyToken(tokens map[string]struct{}, vocabulary []string) bool {
	for _, t := range vocabulary {
		if _, ok := tokens[t]; ok {
			return true
		}
	}
	return false
}

func (lex *Lexicon) ExpandTokenSynonyms(token string) []string {
	if lex == nil || lex.TokenSynonyms == nil {
		return nil
	}
	return lex.TokenSynonyms[token]
}

func (lex *Lexicon) MatchIntents(tokens map[string]struct{}, intents []string) bool {
	for _, intent := range intents {
		switch intent {
		case "catalog":
			if !lex.HasAnyToken(tokens, lex.CategoryProductTokens) {
				return false
			}
		case "quantity":
			if !lex.HasAnyToken(tokens, lex.QuantityTokens) {
				return false
			}
		case "revenue":
			if !lex.HasAnyToken(tokens, lex.RevenueTokens) {
				return false
			}
		default:
			return false
		}
	}
	return len(intents) > 0
}

func (lex *Lexicon) MetricSynonymList(key string) []string {
	if lex == nil || lex.MetricSynonyms == nil {
		return nil
	}
	return lex.MetricSynonyms[key]
}
