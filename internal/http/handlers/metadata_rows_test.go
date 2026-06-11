package handlers

import (
	"strings"
	"testing"

	"github.com/biqly/biqly/internal/dialect"
)

func TestBuildTableRowsWhereRejectsUnknownColumn(t *testing.T) {
	cols := map[string]bool{"name": true}
	_, _, err := buildTableRowsWhere(dialect.Postgres, []tableRowsFilter{
		{Column: "name; DROP TABLE x", Operator: "eq", Value: "a"},
	}, cols)
	if err == nil {
		t.Fatal("expected error for unknown column")
	}
}

func TestBuildTableRowsWhereRejectsUnknownOperator(t *testing.T) {
	cols := map[string]bool{"name": true}
	_, _, err := buildTableRowsWhere(dialect.Postgres, []tableRowsFilter{
		{Column: "name", Operator: "regex", Value: "a"},
	}, cols)
	if err == nil {
		t.Fatal("expected error for unsupported operator")
	}
}

func TestBuildTableRowsWherePredicatesAndArgs(t *testing.T) {
	cols := map[string]bool{"name": true, "likes": true}
	where, args, err := buildTableRowsWhere(dialect.Postgres, []tableRowsFilter{
		{Column: "name", Operator: "contains", Value: "ali"},
		{Column: "likes", Operator: "gte", Value: "10"},
	}, cols)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(where, `"name"`) || !strings.Contains(where, `"likes" >= $2`) {
		t.Errorf("unexpected where clause: %q", where)
	}
	if len(args) != 2 || args[0] != "%ali%" || args[1] != "10" {
		t.Errorf("unexpected args: %#v", args)
	}
}

func TestBuildTableRowsWhereMultiChipBecomesOrGroup(t *testing.T) {
	cols := map[string]bool{"name": true}
	where, args, err := buildTableRowsWhere(dialect.Postgres, []tableRowsFilter{
		{Column: "name", Operator: "eq", Value: `["a","b"]`},
	}, cols)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(where, "OR") || !strings.HasPrefix(strings.TrimSpace(where), "WHERE (") {
		t.Errorf("expected OR group, got %q", where)
	}
	if len(args) != 2 {
		t.Errorf("expected 2 args, got %#v", args)
	}
}
