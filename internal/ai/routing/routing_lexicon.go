package routing

import (
	"fmt"
	"github.com/bytedance/sonic"
	"log/slog"
	"os"
	"sync"
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

var (
	routingLexicon     *Lexicon
	routingLexiconOnce sync.Once
	errRoutingLexicon  error
)

// ActiveRoutingLexicon returns the active routing lexicon (embedded default or file override).
func ActiveRoutingLexicon() (*Lexicon, error) {
	routingLexiconOnce.Do(func() {
		routingLexicon, errRoutingLexicon = loadRoutingLexicon("")
	})
	if errRoutingLexicon != nil {
		return nil, errRoutingLexicon
	}
	return routingLexicon, nil
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
	routingLexicon = lex
	errRoutingLexicon = nil
	routingLexiconOnce = sync.Once{}
	routingLexiconOnce.Do(func() {})
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
