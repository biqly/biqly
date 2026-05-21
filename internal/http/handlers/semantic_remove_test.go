package handlers

import (
	"testing"

	"github.com/biqly/biqly/internal/semantic"
)

func TestJoinMatchesSchema(t *testing.T) {
	base := "public"
	j := semanticJoin("j1", "analytics", "orders", "public", "users")
	if !joinMatchesSchema(j, "analytics", base) {
		t.Fatal("expected join to match analytics schema")
	}
	if !joinMatchesSchema(j, "public", base) {
		t.Fatal("expected join to match public schema on to side")
	}
	j2 := semanticJoin("j2", "", "orders", "", "lines")
	if !joinMatchesSchema(j2, "public", base) {
		t.Fatal("empty schema should default to base")
	}
	if joinMatchesSchema(j2, "analytics", base) {
		t.Fatal("join with only base tables should not match analytics")
	}
}

func TestColumnRefMatchesSchema(t *testing.T) {
	base := "public"
	if !columnRefMatchesSchema("analytics.events.id", "analytics", base) {
		t.Fatal("qualified analytics ref")
	}
	if columnRefMatchesSchema("public.users.id", "analytics", base) {
		t.Fatal("public ref should not match analytics")
	}
	if !columnRefMatchesSchema("users.email", "public", base) {
		t.Fatal("unqualified base-schema ref")
	}
	if columnRefMatchesSchema("analytics.users.email", "public", base) {
		t.Fatal("other schema prefix should not match public exclusion")
	}
}

func TestExpressionReferencesSchema(t *testing.T) {
	if !expressionReferencesSchema("sum(analytics.orders.amount)", "analytics") {
		t.Fatal("expression with analytics prefix")
	}
	if expressionReferencesSchema("sum(public.orders.amount)", "analytics") {
		t.Fatal("other schema in expression")
	}
	if !expressionReferencesSchema(`sum("analytics"."orders"."amount")`, "analytics") {
		t.Fatal("quoted schema prefix")
	}
}

func semanticJoin(id, fromSchema, fromTable, toSchema, toTable string) semantic.Join {
	return semantic.Join{
		ID:         id,
		FromSchema: fromSchema,
		FromTable:  fromTable,
		ToSchema:   toSchema,
		ToTable:    toTable,
	}
}
