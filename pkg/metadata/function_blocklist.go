package metadata

import (
	"encoding/json"
	"fmt"

	"github.com/bytedance/sonic"
)

const FunctionBlocklistConfigKey = "function_blocklist"

// DatasourceFunctionBlocklistConfig is the datasource.config JSON subset that
// controls additional denied SQL functions.
type DatasourceFunctionBlocklistConfig struct {
	FunctionBlocklist []string `json:"function_blocklist,omitempty"`
}

// ParseDatasourceFunctionBlocklist returns the custom entries stored in the
// datasource config. Empty config is equivalent to an empty object.
func ParseDatasourceFunctionBlocklist(config string) ([]string, error) {
	if config == "" {
		return nil, nil
	}
	var value DatasourceFunctionBlocklistConfig
	if err := sonic.Unmarshal([]byte(config), &value); err != nil {
		return nil, fmt.Errorf("parse datasource config: %w", err)
	}
	return value.FunctionBlocklist, nil
}

// DatasourceConfigHasFunctionBlocklist reports whether config explicitly
// contains the reserved function_blocklist key.
func DatasourceConfigHasFunctionBlocklist(config string) (bool, error) {
	values, err := decodeDatasourceConfig(config)
	if err != nil {
		return false, err
	}
	_, ok := values[FunctionBlocklistConfigKey]
	return ok, nil
}

// WithDatasourceFunctionBlocklist replaces only the function_blocklist key,
// preserving every unrelated datasource configuration key.
func WithDatasourceFunctionBlocklist(config string, functions []string) (string, error) {
	values, err := decodeDatasourceConfig(config)
	if err != nil {
		return "", err
	}
	encoded, err := sonic.Marshal(functions)
	if err != nil {
		return "", fmt.Errorf("encode function blocklist: %w", err)
	}
	values[FunctionBlocklistConfigKey] = encoded
	updated, err := sonic.Marshal(values)
	if err != nil {
		return "", fmt.Errorf("encode datasource config: %w", err)
	}
	return string(updated), nil
}

func decodeDatasourceConfig(config string) (map[string]json.RawMessage, error) {
	values := make(map[string]json.RawMessage)
	if config == "" {
		return values, nil
	}
	if err := sonic.Unmarshal([]byte(config), &values); err != nil {
		return nil, fmt.Errorf("parse datasource config: %w", err)
	}
	if values == nil {
		values = make(map[string]json.RawMessage)
	}
	return values, nil
}
