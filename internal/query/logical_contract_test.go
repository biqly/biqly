package query

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestLogicalQueryFrontendTypeMatchesGoJSONContract(t *testing.T) {
	frontend, err := os.ReadFile(filepath.Join("..", "..", "frontend", "src", "types", "ai.ts"))
	if err != nil {
		t.Fatalf("ReadFile(frontend/src/types/ai.ts) error = %v, want nil", err)
	}
	source := string(frontend)

	for _, name := range logicalQueryJSONFields(t) {
		if !strings.Contains(source, name+"?:") && !strings.Contains(source, name+":") {
			t.Errorf("frontend LogicalQuery type missing JSON field %q", name)
		}
	}
	for _, stale := range []string{"data_source:", "window_functions?"} {
		if strings.Contains(source, stale) {
			t.Errorf("frontend LogicalQuery type contains stale field %q", stale)
		}
	}
}

func TestEnsureVersionDefaultsToCurrent(t *testing.T) {
	lq := LogicalQuery{}
	lq.EnsureVersion()
	if lq.Version != CurrentLogicalQueryVersion {
		t.Errorf("EnsureVersion default = %q, want %q", lq.Version, CurrentLogicalQueryVersion)
	}
}

func TestEnsureVersionPreservesExistingValue(t *testing.T) {
	lq := LogicalQuery{Version: "v2-experiment"}
	lq.EnsureVersion()
	if lq.Version != "v2-experiment" {
		t.Errorf("EnsureVersion overwrote caller version: got %q", lq.Version)
	}
}

func TestEnsureVersionNilReceiverIsSafe(t *testing.T) {
	t.Helper()
	var lq *LogicalQuery
	lq.EnsureVersion()
}

func TestEnsureGroupBySelectedAddsMissingDimensionsBeforeMetrics(t *testing.T) {
	lq := LogicalQuery{
		Select: []SelectItem{
			{Type: SelectTypeMetric, Name: "row_count"},
			{Type: SelectTypeMetric, Name: "sum_retweets"},
		},
		GroupBy: []GroupBy{{Field: "created_at_ts_day"}},
	}

	lq.EnsureGroupBySelected()

	want := []SelectItem{
		{Type: SelectTypeDimension, Name: "created_at_ts_day"},
		{Type: SelectTypeMetric, Name: "row_count"},
		{Type: SelectTypeMetric, Name: "sum_retweets"},
	}
	if !reflect.DeepEqual(lq.Select, want) {
		t.Errorf("EnsureGroupBySelected() select = %+v, want %+v", lq.Select, want)
	}
}

func TestEnsureGroupBySelectedDoesNotDuplicateExistingDimension(t *testing.T) {
	lq := LogicalQuery{
		Select: []SelectItem{
			{Type: SelectTypeDimension, Name: "created_at_ts_day"},
			{Type: SelectTypeMetric, Name: "row_count"},
		},
		GroupBy: []GroupBy{{Field: "created_at_ts_day"}},
	}

	lq.EnsureGroupBySelected()

	if len(lq.Select) != 2 {
		t.Errorf("EnsureGroupBySelected() duplicated select items: %+v", lq.Select)
	}
}

func logicalQueryJSONFields(t *testing.T) []string {
	t.Helper()
	typ := reflect.TypeFor[LogicalQuery]()
	fields := make([]string, 0, typ.NumField())
	for field := range typ.Fields() {
		tag := field.Tag.Get("json")
		if tag == "-" || tag == "" {
			continue
		}
		name := strings.Split(tag, ",")[0]
		if name == "" {
			continue
		}
		fields = append(fields, name)
	}
	return fields
}
