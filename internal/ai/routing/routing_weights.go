package routing

import (
	"fmt"
	"github.com/bytedance/sonic"
	"log/slog"
	"os"
	"strings"
	"sync"
)

// TableBoostRule adds score when all listed intents match and the table name contains a substring.
type TableBoostRule struct {
	Intents         []string `json:"intents"`
	TableSubstrings []string `json:"table_substrings"`
	Boost           float64  `json:"boost"`
}

// Weights RoutingWeights configures keyword-scoring magnitudes for table routing.
// Defaults are embedded; override with BI_AI_ROUTING_WEIGHTS_PATH (JSON).
type Weights struct {
	TableName                  float64          `json:"table_name"`
	TableDescription           float64          `json:"table_description"`
	ColumnName                 float64          `json:"column_name"`
	ColumnDataType             float64          `json:"column_data_type"`
	ColumnDescription          float64          `json:"column_description"`
	RevenueColumnBoost         float64          `json:"revenue_column_boost"`
	ReadableLabelColumnBoost   float64          `json:"readable_label_column_boost"`
	ColumnKeywordMatch         float64          `json:"column_keyword_match"`
	ColumnKeywordName          float64          `json:"column_keyword_name"`
	ColumnKeywordDescription   float64          `json:"column_keyword_description"`
	ColumnRevenueBoost         float64          `json:"column_revenue_boost"`
	ColumnDisplayNameBoost     float64          `json:"column_display_name_boost"`
	SelectionRelativeThreshold float64          `json:"selection_relative_threshold"`
	EntityPathBridgeScore      float64          `json:"entity_path_bridge_score"`
	EntityPathTargetScore      float64          `json:"entity_path_target_score"`
	ResolverPathBridgeScore    float64          `json:"resolver_path_bridge_score"`
	ResolverPathTargetScore    float64          `json:"resolver_path_target_score"`
	TableBoostRules            []TableBoostRule `json:"table_boost_rules"`
}

var (
	routingWeightsMu     sync.Mutex
	routingWeights       *Weights
	routingWeightsLoaded bool
	errRoutingWeights    error
)

// ActiveRoutingWeights returns the active routing weights (embedded default or file override).
func ActiveRoutingWeights() (*Weights, error) {
	routingWeightsMu.Lock()
	if !routingWeightsLoaded {
		routingWeights, errRoutingWeights = loadRoutingWeights("")
		routingWeightsLoaded = true
	}
	routingWeightsMu.Unlock()
	if errRoutingWeights != nil {
		return nil, errRoutingWeights
	}
	return routingWeights, nil
}

func activeRoutingWeights() *Weights {
	w, err := ActiveRoutingWeights()
	if err != nil {
		slog.Error("routing weights unavailable, using embedded defaults", "error", err)
		fallback, parseErr := parseRoutingWeightsJSON(embeddedRoutingWeightsJSON)
		if parseErr != nil {
			slog.Error("embedded routing weights invalid", "error", parseErr)
			return &Weights{}
		}
		return fallback
	}
	return w
}

// InitRoutingWeights loads routing weights from an optional JSON file path.
// Empty path keeps embedded defaults. Call once at process startup.
func InitRoutingWeights(path string) error {
	w, err := loadRoutingWeights(path)
	if err != nil {
		return err
	}
	routingWeightsMu.Lock()
	routingWeights = w
	errRoutingWeights = nil
	routingWeightsLoaded = true
	routingWeightsMu.Unlock()
	return nil
}

func loadRoutingWeights(path string) (*Weights, error) {
	w, err := parseRoutingWeightsJSON(embeddedRoutingWeightsJSON)
	if err != nil {
		return nil, fmt.Errorf("embedded routing weights: %w", err)
	}
	if path == "" {
		return w, nil
	}
	raw, err := os.ReadFile(path) //nolint:gosec // operator-provided routing weights path
	if err != nil {
		return nil, fmt.Errorf("read routing weights %q: %w", path, err)
	}
	override, err := parseRoutingWeightsJSON(raw)
	if err != nil {
		return nil, fmt.Errorf("parse routing weights %q: %w", path, err)
	}
	mergeRoutingWeights(w, override)
	return w, nil
}

func parseRoutingWeightsJSON(raw []byte) (*Weights, error) {
	var w Weights
	if err := sonic.ConfigStd.Unmarshal(raw, &w); err != nil {
		return nil, err
	}
	return &w, nil
}

func mergeRoutingWeights(base, override *Weights) {
	mergePositiveWeight(&base.TableName, override.TableName)
	mergePositiveWeight(&base.TableDescription, override.TableDescription)
	mergePositiveWeight(&base.ColumnName, override.ColumnName)
	mergePositiveWeight(&base.ColumnDataType, override.ColumnDataType)
	mergePositiveWeight(&base.ColumnDescription, override.ColumnDescription)
	mergePositiveWeight(&base.RevenueColumnBoost, override.RevenueColumnBoost)
	mergePositiveWeight(&base.ReadableLabelColumnBoost, override.ReadableLabelColumnBoost)
	mergePositiveWeight(&base.ColumnKeywordMatch, override.ColumnKeywordMatch)
	mergePositiveWeight(&base.ColumnKeywordName, override.ColumnKeywordName)
	mergePositiveWeight(&base.ColumnKeywordDescription, override.ColumnKeywordDescription)
	mergePositiveWeight(&base.ColumnRevenueBoost, override.ColumnRevenueBoost)
	mergePositiveWeight(&base.ColumnDisplayNameBoost, override.ColumnDisplayNameBoost)
	mergePositiveWeight(&base.SelectionRelativeThreshold, override.SelectionRelativeThreshold)
	mergePositiveWeight(&base.EntityPathBridgeScore, override.EntityPathBridgeScore)
	mergePositiveWeight(&base.EntityPathTargetScore, override.EntityPathTargetScore)
	mergePositiveWeight(&base.ResolverPathBridgeScore, override.ResolverPathBridgeScore)
	mergePositiveWeight(&base.ResolverPathTargetScore, override.ResolverPathTargetScore)
	if len(override.TableBoostRules) > 0 {
		base.TableBoostRules = override.TableBoostRules
	}
}

func mergePositiveWeight(base *float64, override float64) {
	if override > 0 {
		*base = override
	}
}

func (w *Weights) ApplyTableBoosts(tableName string, tokens map[string]struct{}, score float64, lex *Lexicon) float64 {
	if w == nil || lex == nil {
		return score
	}
	tn := strings.ToLower(tableName)
	for _, rule := range w.TableBoostRules {
		if !lex.MatchIntents(tokens, rule.Intents) {
			continue
		}
		for _, sub := range rule.TableSubstrings {
			if strings.Contains(tn, sub) {
				score += rule.Boost
				break
			}
		}
	}
	return score
}

// InitRouting loads lexicon and weights from optional config paths.
func InitRouting(lexiconPath, weightsPath string) error {
	if err := InitRoutingLexicon(lexiconPath); err != nil {
		return err
	}
	return InitRoutingWeights(weightsPath)
}
