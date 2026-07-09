package metadata

import (
	"slices"
	"testing"

	"github.com/bytedance/sonic"
)

func TestDatasourceFunctionBlocklistConfigRoundTripPreservesOtherKeys(t *testing.T) {
	updated, err := WithDatasourceFunctionBlocklist(`{"timezone":"UTC","max_rows":100}`, []string{"custom_reader"})
	if err != nil {
		t.Fatalf("WithDatasourceFunctionBlocklist() error = %v", err)
	}
	got, err := ParseDatasourceFunctionBlocklist(updated)
	if err != nil {
		t.Fatalf("ParseDatasourceFunctionBlocklist() error = %v", err)
	}
	if !slices.Equal(got, []string{"custom_reader"}) {
		t.Fatalf("blocklist = %v, want [custom_reader]", got)
	}
	var values map[string]any
	if err := sonic.Unmarshal([]byte(updated), &values); err != nil {
		t.Fatalf("unmarshal updated config: %v", err)
	}
	if values["timezone"] != "UTC" || values["max_rows"] != float64(100) {
		t.Fatalf("unrelated config values = %v, want preserved", values)
	}
}

func TestParseDatasourceFunctionBlocklistRejectsInvalidJSON(t *testing.T) {
	if _, err := ParseDatasourceFunctionBlocklist(`{not-json}`); err == nil {
		t.Fatal("ParseDatasourceFunctionBlocklist() error = nil, want invalid JSON error")
	}
}

func TestWithDatasourceFunctionBlocklistHandlesNullConfig(t *testing.T) {
	updated, err := WithDatasourceFunctionBlocklist(`null`, []string{"custom_reader"})
	if err != nil {
		t.Fatalf("WithDatasourceFunctionBlocklist() error = %v", err)
	}
	got, err := ParseDatasourceFunctionBlocklist(updated)
	if err != nil {
		t.Fatalf("ParseDatasourceFunctionBlocklist() error = %v", err)
	}
	if !slices.Equal(got, []string{"custom_reader"}) {
		t.Fatalf("blocklist = %v, want [custom_reader]", got)
	}
}

func TestDatasourceConfigHasFunctionBlocklist(t *testing.T) {
	got, err := DatasourceConfigHasFunctionBlocklist(`{"function_blocklist":[]}`)
	if err != nil {
		t.Fatalf("DatasourceConfigHasFunctionBlocklist() error = %v", err)
	}
	if !got {
		t.Fatal("DatasourceConfigHasFunctionBlocklist() = false, want true")
	}
}
