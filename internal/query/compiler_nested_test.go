package query

import (
	"context"
	"testing"

	"github.com/biqly/biqly/internal/dialect"
	"github.com/biqly/biqly/internal/semantic"
)

func TestResolveFromClause_DefaultsToModel(t *testing.T) {
	c := &Compiler{dialect: dialect.Postgres}
	model := &semantic.SemanticModel{BaseSchema: "public", BaseTable: "orders"}
	lq := &LogicalQuery{
		Select: []SelectItem{{Type: SelectTypeMetric, Name: "revenue"}},
		Limit:  100,
	}
	args := &[]any{}

	fromClause, err := c.resolveFromClause(lq, model, args)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := `"public"."orders"`
	if fromClause != want {
		t.Fatalf("got %q, want %q", fromClause, want)
	}
}

func TestResolveFromClause_FromCTE(t *testing.T) {
	c := &Compiler{dialect: dialect.Postgres}
	model := &semantic.SemanticModel{
		Dimensions: []semantic.Dimension{
			{Name: "country", ColumnRef: "customers.country", Type: "text"},
		},
	}
	lq := &LogicalQuery{
		Select:  []SelectItem{{Type: SelectTypeDimension, Name: "country"}},
		FromCTE: "my_cte",
		Limit:   100,
		Version: CurrentLogicalQueryVersion,
	}
	args := &[]any{}

	fromClause, err := c.resolveFromClause(lq, model, args)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := `"my_cte"`
	if fromClause != want {
		t.Fatalf("got %q, want %q", fromClause, want)
	}
}

func TestResolveFromClause_FromSubquery(t *testing.T) {
	c := NewCompiler(dialect.PostgresDialect{}).withCompileCtx(context.Background())
	model := &semantic.SemanticModel{
		Dimensions: []semantic.Dimension{
			{Name: "country", ColumnRef: "customers.country", Type: "text"},
		},
	}
	lq := &LogicalQuery{
		Select: []SelectItem{{Type: SelectTypeDimension, Name: "country"}},
		FromSubquery: &SubqueryBody{
			Select: []SelectItem{{Type: SelectTypeDimension, Name: "country"}},
		},
		FromAlias: "sub",
		Limit:     100,
		Version:   CurrentLogicalQueryVersion,
	}
	args := &[]any{}

	fromClause, err := c.resolveFromClause(lq, model, args)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if fromClause == "" {
		t.Fatal("expected non-empty from clause")
	}
}

func TestResolveFromClause_FromSubqueryDefaultAlias(t *testing.T) {
	c := NewCompiler(dialect.PostgresDialect{}).withCompileCtx(context.Background())
	model := &semantic.SemanticModel{
		Dimensions: []semantic.Dimension{
			{Name: "country", ColumnRef: "customers.country", Type: "text"},
		},
	}
	lq := &LogicalQuery{
		Select: []SelectItem{{Type: SelectTypeDimension, Name: "country"}},
		FromSubquery: &SubqueryBody{
			Select: []SelectItem{{Type: SelectTypeDimension, Name: "country"}},
		},
		Limit:   100,
		Version: CurrentLogicalQueryVersion,
	}
	args := &[]any{}

	fromClause, err := c.resolveFromClause(lq, model, args)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if fromClause == "" {
		t.Fatal("expected non-empty from clause")
	}
}
