package ai

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"sync"
)

// TableBoostRule adds score when all listed intents match and the table name contains a substring.
type TableBoostRule struct {
	Intents          []string  `json:"intents"`
	TableSubstrings  []string  `json:"table_substrings"`
	Boost            float64   `json:"boost"`
}

// RoutingWeights configures keyword-scoring magnitudes for table routing.
// Defaults are embedded; override with BI_AI_ROUTING_WEIGHTS_PATH (JSON).
type RoutingWeights struct {
	TableName                 float64          `json:"table_name"`
	TableDescription          float64          `json:"table_description"`
	ColumnName                float64          `json:"column_name"`
	ColumnDataType            float64          `json:"column_data_type"`
	ColumnDescription         float64          `json:"column_description"`
	RevenueColumnBoost        float64          `json:"revenue_column_boost"`
	ReadableLabelColumnBoost  float64          `json:"readable_label_column_boost"`
	ColumnKeywordMatch        float64          `json:"column_keyword_match"`
	ColumnKeywordName         float64          `json:"column_keyword_name"`
	ColumnKeywordDescription  float64          `json:"column_keyword_description"`
	ColumnRevenueBoost        float64          `json:"column_revenue_boost"`
	ColumnDisplayNameBoost    float64          `json:"column_display_name_boost"`
	SelectionRelativeThreshold float64         `json:"selection_relative_threshold"`
	EntityPathBridgeScore     float64          `json:"entity_path_bridge_score"`
	EntityPathTargetScore     float64          `json:"entity_path_target_score"`
	ResolverPathBridgeScore   float64          `json:"resolver_path_bridge_score"`
	ResolverPathTargetScore   float64          `json:"resolver_path_target_score"`
	TableBoostRules           []TableBoostRule `json:"table_boost_rules"`
}

var (
	routingWeights     *RoutingWeights
	routingWeightsOnce sync.Once
	routingWeightsErr  error
)

// ActiveRoutingWeights returns the active routing weights (embedded default or file override).
func ActiveRoutingWeights() (*RoutingWeights, error) {
	routingWeightsOnce.Do(func() {
		routingWeights, routingWeightsErr = loadRoutingWeights("")
	})
	if routingWeightsErr != nil {
		return nil, routingWeightsErr
	}
	return routingWeights, nil
}

func activeRoutingWeights() *RoutingWeights {
	w, err := ActiveRoutingWeights()
	if err != nil {
		slog.Error("routing weights unavailable, using embedded defaults", "error", err)
		fallback, parseErr := parseRoutingWeightsJSON(embeddedRoutingWeightsJSON)
		if parseErr != nil {
			panic(parseErr)
		}
		return fallback
	}
	return w
}

// InitRoutingWeights loads routing weights from an optional JSON file path.
func InitRoutingWeights(path string) error {
	w, err := loadRoutingWeights(path)
	if err != nil {
		return err
	}
	routingWeights = w
	routingWeightsErr = nil
	routingWeightsOnce = sync.Once{}
	routingWeightsOnce.Do(func() {})
	return nil
}

func loadRoutingWeights(path string) (*RoutingWeights, error) {
	w, err := parseRoutingWeightsJSON(embeddedRoutingWeightsJSON)
	if err != nil {
		return nil, fmt.Errorf("embedded routing weights: %w", err)
	}
	if path == "" {
		return w, nil
	}
	raw, err := os.ReadFile(path)
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

func parseRoutingWeightsJSON(raw []byte) (*RoutingWeights, error) {
	var w RoutingWeights
	if err := json.Unmarshal(raw, &w); err != nil {
		return nil, err
	}
	return &w, nil
}

func mergeRoutingWeights(base, override *RoutingWeights) {
	if override.TableName > 0 {
		base.TableName = override.TableName
	}
	if override.TableDescription > 0 {
		base.TableDescription = override.TableDescription
	}
	if override.ColumnName > 0 {
		base.ColumnName = override.ColumnName
	}
	if override.ColumnDataType > 0 {
		base.ColumnDataType = override.ColumnDataType
	}
	if override.ColumnDescription > 0 {
		base.ColumnDescription = override.ColumnDescription
	}
	if override.RevenueColumnBoost > 0 {
		base.RevenueColumnBoost = override.RevenueColumnBoost
	}
	if override.ReadableLabelColumnBoost > 0 {
		base.ReadableLabelColumnBoost = override.ReadableLabelColumnBoost
	}
	if override.ColumnKeywordMatch > 0 {
		base.ColumnKeywordMatch = override.ColumnKeywordMatch
	}
	if override.ColumnKeywordName > 0 {
		base.ColumnKeywordName = override.ColumnKeywordName
	}
	if override.ColumnKeywordDescription > 0 {
		base.ColumnKeywordDescription = override.ColumnKeywordDescription
	}
	if override.ColumnRevenueBoost > 0 {
		base.ColumnRevenueBoost = override.ColumnRevenueBoost
	}
	if override.ColumnDisplayNameBoost > 0 {
		base.ColumnDisplayNameBoost = override.ColumnDisplayNameBoost
	}
	if override.SelectionRelativeThreshold > 0 {
		base.SelectionRelativeThreshold = override.SelectionRelativeThreshold
	}
	if override.EntityPathBridgeScore > 0 {
		base.EntityPathBridgeScore = override.EntityPathBridgeScore
	}
	if override.EntityPathTargetScore > 0 {
		base.EntityPathTargetScore = override.EntityPathTargetScore
	}
	if override.ResolverPathBridgeScore > 0 {
		base.ResolverPathBridgeScore = override.ResolverPathBridgeScore
	}
	if override.ResolverPathTargetScore > 0 {
		base.ResolverPathTargetScore = override.ResolverPathTargetScore
	}
	if len(override.TableBoostRules) > 0 {
		base.TableBoostRules = override.TableBoostRules
	}
}

func (w *RoutingWeights) ApplyTableBoosts(tableName string, tokens map[string]bool, score float64, lex *RoutingLexicon) float64 {
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
