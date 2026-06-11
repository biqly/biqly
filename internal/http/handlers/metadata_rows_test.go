package handlers

import (
	"strings"
	"testing"

	"github.com/biqly/biqly/internal/dialect"
	"github.com/biqly/biqly/internal/metadata"
	"github.com/biqly/biqly/internal/query"
	"github.com/biqly/biqly/internal/security/pii"
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

func TestBuildTableRowsProjectionAppliesPIIMasking(t *testing.T) {
	emailType := pii.TypeEmail
	tcknType := pii.TypeTCKimlikNo
	columns := []metadata.Column{
		{SchemaName: "public", TableName: "customers", ColumnName: "id"},
		{SchemaName: "public", TableName: "customers", ColumnName: "email", PIIType: &emailType},
		{SchemaName: "public", TableName: "customers", ColumnName: "tckn", PIIType: &tcknType},
	}
	access, types := pii.BuildColumnAccessMaps(pii.RoleViewer, columns, nil)
	cfg := &query.PIIMaskingConfig{ColumnAccess: access, ColumnTypes: types}

	projection := buildTableRowsProjection(dialect.Postgres, columns, cfg)

	if strings.Contains(strings.Join(projection, ", "), `"tckn"`) {
		t.Fatalf("hidden column must not be projected: %#v", projection)
	}
	if !containsProjection(projection, `"id"`) {
		t.Fatalf("raw column missing from projection: %#v", projection)
	}
	if !containsProjection(projection, `AS "email"`) || !containsProjection(projection, "'***'") {
		t.Fatalf("masked column must be projected as masked expression with stable alias: %#v", projection)
	}
}

func TestBuildTableRowsWhereRejectsProtectedPIIColumns(t *testing.T) {
	cols := map[string]bool{"email": true, "tckn": true}
	protected := map[string]bool{"email": true, "tckn": true}

	_, _, err := buildTableRowsWhere(dialect.Postgres, []tableRowsFilter{
		{Column: "email", Operator: "eq", Value: "alice@example.com"},
	}, cols, protected)
	if err == nil {
		t.Fatal("expected masked column filter to be rejected")
	}

	_, _, err = buildTableRowsWhere(dialect.Postgres, []tableRowsFilter{
		{Column: "tckn", Operator: "eq", Value: "10000000146"},
	}, cols, protected)
	if err == nil {
		t.Fatal("expected hidden column filter to be rejected")
	}
}

func TestBuildTableRowsOrderRejectsProtectedPIIColumns(t *testing.T) {
	err := validateTableRowsOrderBy("email", map[string]bool{"email": true}, map[string]bool{"email": true})
	if err == nil {
		t.Fatal("expected masked order_by column to be rejected")
	}
}

func containsProjection(projection []string, want string) bool {
	for _, expr := range projection {
		if strings.Contains(expr, want) {
			return true
		}
	}
	return false
}
