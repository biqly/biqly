package query

import (
	"testing"

	"github.com/biqly/biqly/internal/dialect"
)

func TestCaseSensitiveComparison_MySQL(t *testing.T) {
	result := caseSensitiveComparison("mysql", `"col"`, "=", "$1")
	want := `"col" = BINARY $1`
	if result != want {
		t.Fatalf("got %q, want %q", result, want)
	}
}

func TestCaseSensitiveComparison_SQLServer(t *testing.T) {
	result := caseSensitiveComparison("sqlserver", "[col]", "=", "@p1")
	want := "[col] = @p1 COLLATE Latin1_General_CS_AS"
	if result != want {
		t.Fatalf("got %q, want %q", result, want)
	}
}

func TestCaseSensitiveComparison_Default(t *testing.T) {
	result := caseSensitiveComparison("postgres", `"col"`, "=", "$1")
	want := `"col" = $1`
	if result != want {
		t.Fatalf("got %q, want %q", result, want)
	}
}

func TestBuildStartsWithFilter_SingleString(t *testing.T) {
	c := &Compiler{dialect: dialect.Postgres}
	args := &[]any{}
	lhsSQL := `"orders"."status"`

	sql, newArgs, err := c.buildStartsWithFilter(Filter{Value: "acti"}, lhsSQL, nil, args)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	wantSQL := `"orders"."status" ILIKE $1`
	if sql != wantSQL {
		t.Fatalf("got %q, want %q", sql, wantSQL)
	}
	if len(newArgs) != 0 {
		t.Fatalf("newArgs should be nil, got %v", newArgs)
	}
	if len(*args) != 1 || (*args)[0] != "acti%" {
		t.Fatalf("args = %v, want [acti%%]", *args)
	}
}

func TestBuildStartsWithFilter_Slice(t *testing.T) {
	c := &Compiler{dialect: dialect.Postgres}
	args := &[]any{}
	lhsSQL := `"col"`

	sql, _, err := c.buildStartsWithFilter(Filter{Value: []string{"foo", "bar"}}, lhsSQL, nil, args)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sql == "" {
		t.Fatal("expected non-empty SQL")
	}
	if len(*args) != 2 {
		t.Fatalf("expected 2 args, got %d", len(*args))
	}
}

func TestBuildStartsWithFilter_EmptySlice(t *testing.T) {
	c := &Compiler{dialect: dialect.Postgres}
	args := &[]any{}
	lhsSQL := `"col"`

	sql, _, err := c.buildStartsWithFilter(Filter{Value: []string{}}, lhsSQL, nil, args)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sql != "1=1" {
		t.Fatalf("got %q, want 1=1", sql)
	}
}

func TestBuildStartsWithFilter_NonStringValue(t *testing.T) {
	c := &Compiler{dialect: dialect.Postgres}
	args := &[]any{}
	lhsSQL := `"col"`

	sql, _, err := c.buildStartsWithFilter(Filter{Value: int64(42)}, lhsSQL, nil, args)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sql == "" {
		t.Fatal("expected non-empty SQL")
	}
}

func TestBuildStartsWithFilter_CaseSensitiveMySQL(t *testing.T) {
	c := &Compiler{dialect: dialect.MySQL}
	args := &[]any{}
	lhsSQL := "`col`"

	sql, _, err := c.buildStartsWithFilter(Filter{Value: "Foo", CaseSensitive: true}, lhsSQL, nil, args)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	wantSQL := "`col` LIKE BINARY ?"
	if sql != wantSQL {
		t.Fatalf("got %q, want %q", sql, wantSQL)
	}
}

func TestBuildEndsWithFilter_SingleString(t *testing.T) {
	c := &Compiler{dialect: dialect.Postgres}
	args := &[]any{}
	lhsSQL := `"orders"."status"`

	sql, _, err := c.buildEndsWithFilter(Filter{Value: "ive"}, lhsSQL, nil, args)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	wantSQL := `"orders"."status" ILIKE $1`
	if sql != wantSQL {
		t.Fatalf("got %q, want %q", sql, wantSQL)
	}
	if len(*args) != 1 || (*args)[0] != "%ive" {
		t.Fatalf("args = %v, want [%%ive]", *args)
	}
}

func TestBuildEndsWithFilter_Slice(t *testing.T) {
	c := &Compiler{dialect: dialect.Postgres}
	args := &[]any{}
	lhsSQL := `"col"`

	sql, _, err := c.buildEndsWithFilter(Filter{Value: []string{"ing", "ed"}}, lhsSQL, nil, args)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sql == "" {
		t.Fatal("expected non-empty SQL")
	}
	if len(*args) != 2 {
		t.Fatalf("expected 2 args, got %d", len(*args))
	}
}

func TestBuildEndsWithFilter_EmptySlice(t *testing.T) {
	c := &Compiler{dialect: dialect.Postgres}
	args := &[]any{}
	lhsSQL := `"col"`

	sql, _, err := c.buildEndsWithFilter(Filter{Value: []string{}}, lhsSQL, nil, args)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sql != "1=1" {
		t.Fatalf("got %q, want 1=1", sql)
	}
}

func TestBuildEndsWithFilter_NonStringValue(t *testing.T) {
	c := &Compiler{dialect: dialect.Postgres}
	args := &[]any{}
	lhsSQL := `"col"`

	sql, _, err := c.buildEndsWithFilter(Filter{Value: float64(3.14)}, lhsSQL, nil, args)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sql == "" {
		t.Fatal("expected non-empty SQL")
	}
}

func TestBuildStartsWithFilter_CaseSensitive(t *testing.T) {
	c := &Compiler{dialect: dialect.Postgres}
	args := &[]any{}
	lhsSQL := `"col"`

	sql, _, err := c.buildStartsWithFilter(Filter{Value: "test", CaseSensitive: true}, lhsSQL, nil, args)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	wantSQL := `"col" LIKE $1`
	if sql != wantSQL {
		t.Fatalf("got %q, want %q", sql, wantSQL)
	}
}
