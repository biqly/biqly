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

func logicalQueryJSONFields(t *testing.T) []string {
	t.Helper()
	typ := reflect.TypeOf(LogicalQuery{})
	fields := make([]string, 0, typ.NumField())
	for i := 0; i < typ.NumField(); i++ {
		tag := typ.Field(i).Tag.Get("json")
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
