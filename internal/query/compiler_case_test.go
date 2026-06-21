package query

import (
	"testing"

	"github.com/biqly/biqly/internal/dialect"
	"github.com/biqly/biqly/internal/semantic"
)

func TestBuildCaseThen_Dimension(t *testing.T) {
	c := &Compiler{dialect: dialect.Postgres}
	dimMap := map[string]*semantic.Dimension{
		"status": {Name: "status", ColumnRef: "orders.status", Type: "text"},
	}
	resolver := NewSchemaResolver(&semantic.SemanticModel{BaseSchema: "public"}, nil)
	args := &[]any{}

	sql, err := c.buildCaseThen(CaseThen{Type: CaseThenTypeDimension, Dimension: "status"}, dimMap, resolver, args)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sql != `"orders"."status"` {
		t.Fatalf("got %q, want \"orders\".\"status\"", sql)
	}
}

func TestBuildCaseThen_DimensionEmptyType(t *testing.T) {
	c := &Compiler{dialect: dialect.Postgres}
	dimMap := map[string]*semantic.Dimension{
		"status": {Name: "status", ColumnRef: "orders.status", Type: "text"},
	}
	resolver := NewSchemaResolver(&semantic.SemanticModel{BaseSchema: "public"}, nil)
	args := &[]any{}

	sql, err := c.buildCaseThen(CaseThen{Dimension: "status"}, dimMap, resolver, args)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sql != `"orders"."status"` {
		t.Fatalf("got %q, want \"orders\".\"status\"", sql)
	}
}

func TestBuildCaseThen_EmptyDimensionName(t *testing.T) {
	c := &Compiler{dialect: dialect.Postgres}
	dimMap := map[string]*semantic.Dimension{}
	resolver := NewSchemaResolver(&semantic.SemanticModel{}, nil)
	args := &[]any{}

	_, err := c.buildCaseThen(CaseThen{Type: CaseThenTypeDimension, Dimension: ""}, dimMap, resolver, args)
	if err == nil {
		t.Fatal("expected error for empty dimension name")
	}
}

func TestBuildCaseThen_UnknownDimension(t *testing.T) {
	c := &Compiler{dialect: dialect.Postgres}
	dimMap := map[string]*semantic.Dimension{}
	resolver := NewSchemaResolver(&semantic.SemanticModel{}, nil)
	args := &[]any{}

	_, err := c.buildCaseThen(CaseThen{Type: CaseThenTypeDimension, Dimension: "nonexistent"}, dimMap, resolver, args)
	if err == nil {
		t.Fatal("expected error for unknown dimension")
	}
}

func TestBuildCaseThen_LiteralBool(t *testing.T) {
	c := &Compiler{dialect: dialect.Postgres}
	resolver := NewSchemaResolver(&semantic.SemanticModel{}, nil)
	args := &[]any{}

	// bool true
	sql, err := c.buildCaseThen(CaseThen{Type: CaseThenTypeLiteral, Literal: true}, nil, resolver, args)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sql != "TRUE" {
		t.Fatalf("got %q, want TRUE", sql)
	}

	// bool false
	sql, err = c.buildCaseThen(CaseThen{Type: CaseThenTypeLiteral, Literal: false}, nil, resolver, args)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sql != "FALSE" {
		t.Fatalf("got %q, want FALSE", sql)
	}
}

func TestBuildCaseThen_LiteralInt(t *testing.T) {
	c := &Compiler{dialect: dialect.Postgres}
	resolver := NewSchemaResolver(&semantic.SemanticModel{}, nil)
	args := &[]any{}

	sql, err := c.buildCaseThen(CaseThen{Type: CaseThenTypeLiteral, Literal: int64(42)}, nil, resolver, args)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sql != "$1" {
		t.Fatalf("got %q, want $1", sql)
	}
	if len(*args) != 1 || (*args)[0] != int64(42) {
		t.Fatalf("args = %v, want [42]", *args)
	}
}

func TestBuildCaseThen_LiteralString(t *testing.T) {
	c := &Compiler{dialect: dialect.Postgres}
	resolver := NewSchemaResolver(&semantic.SemanticModel{}, nil)
	args := &[]any{}

	sql, err := c.buildCaseThen(CaseThen{Type: CaseThenTypeLiteral, Literal: "active"}, nil, resolver, args)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sql != "$1" {
		t.Fatalf("got %q, want $1", sql)
	}
	if len(*args) != 1 || (*args)[0] != "active" {
		t.Fatalf("args = %v, want [active]", *args)
	}
}

func TestBuildCaseThen_InvalidType(t *testing.T) {
	c := &Compiler{dialect: dialect.Postgres}
	resolver := NewSchemaResolver(&semantic.SemanticModel{}, nil)
	args := &[]any{}

	_, err := c.buildCaseThen(CaseThen{Type: "invalid_type"}, nil, resolver, args)
	if err == nil {
		t.Fatal("expected error for invalid case then type")
	}
}

func TestFormatLiteral_Nil(t *testing.T) {
	c := &Compiler{dialect: dialect.Postgres}
	args := &[]any{}

	sql, err := c.formatLiteral(nil, args)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sql != "NULL" {
		t.Fatalf("got %q, want NULL", sql)
	}
}

func TestFormatLiteral_Bool(t *testing.T) {
	c := &Compiler{dialect: dialect.Postgres}
	args := &[]any{}

	sql, err := c.formatLiteral(true, args)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sql != "TRUE" {
		t.Fatalf("got %q, want TRUE", sql)
	}

	sql, err = c.formatLiteral(false, args)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sql != "FALSE" {
		t.Fatalf("got %q, want FALSE", sql)
	}
}

func TestFormatLiteral_Int(t *testing.T) {
	c := &Compiler{dialect: dialect.Postgres}
	args := &[]any{}

	sql, err := c.formatLiteral(42, args)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sql != "$1" {
		t.Fatalf("got %q, want $1", sql)
	}
	if len(*args) != 1 || (*args)[0] != 42 {
		t.Fatalf("args = %v, want [42]", *args)
	}
}

func TestFormatLiteral_String(t *testing.T) {
	c := &Compiler{dialect: dialect.Postgres}
	args := &[]any{}

	sql, err := c.formatLiteral("hello", args)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sql != "$1" {
		t.Fatalf("got %q, want $1", sql)
	}
	if len(*args) != 1 || (*args)[0] != "hello" {
		t.Fatalf("args = %v, want [hello]", *args)
	}
}

func TestFormatLiteral_Default(t *testing.T) {
	c := &Compiler{dialect: dialect.Postgres}
	args := &[]any{}

	sql, err := c.formatLiteral(struct{ Name string }{Name: "test"}, args)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sql != "$1" {
		t.Fatalf("got %q, want $1", sql)
	}
	if len(*args) != 1 {
		t.Fatalf("len(args) = %d, want 1", len(*args))
	}
}
