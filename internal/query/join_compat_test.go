package query

import (
	"testing"

	"github.com/biqly/biqly/internal/semantic"
)

func TestJoinDataTypesCompatible(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name  string
		left  string
		right string
		want  bool
	}{
		{"same integer types", "integer", "bigint", true},
		{"serial to int", "bigserial", "int8", true},
		{"integer to numeric cross-group", "integer", "numeric(12,2)", true},
		{"double to bigint cross-group", "double precision", "bigint", true},
		{"text to citext", "text", "citext", true},
		{"varchar with length to text", "character varying(255)", "text", true},
		{"uuid to uuid", "uuid", "uuid", true},
		{"uuid to text rejected", "uuid", "text", false},
		{"uuid to date rejected", "uuid", "date", false},
		{"date to timestamptz cross-group", "date", "timestamp with time zone", true},
		{"timestamp to date cross-group", "timestamp", "date", true},
		{"date to integer rejected", "date", "integer", false},
		{"boolean to boolean", "bool", "boolean", true},
		{"boolean to integer rejected", "boolean", "integer", false},
		{"json only joins json", "jsonb", "json", true},
		{"json to text rejected", "jsonb", "text", false},
		{"unknown type fails open", "hstore", "uuid", true},
		{"both unknown fails open", "hstore", "ltree", true},
		{"empty type fails open", "", "uuid", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := JoinDataTypesCompatible(tc.left, tc.right); got != tc.want {
				t.Fatalf("JoinDataTypesCompatible(%q, %q) = %v, want %v", tc.left, tc.right, got, tc.want)
			}
		})
	}
}

func TestValidateJoinColumnTypes(t *testing.T) {
	t.Parallel()

	types := map[string]string{
		"public.orders.customer_id": "uuid",
		"public.orders.created_at":  "timestamp with time zone",
		"public.customers.id":       "uuid",
		"public.customers.signup":   "date",
	}
	lookup := func(schema, table, column string) (string, bool) {
		v, ok := types[schema+"."+table+"."+column]
		return v, ok
	}
	model := &semantic.SemanticModel{
		BaseSchema: "public",
		BaseTable:  "orders",
		Joins: []semantic.Join{
			// compatible: uuid = uuid (schemas resolved from base schema)
			{Name: "ok", FromTable: "orders", FromColumn: "customer_id", ToTable: "customers", ToColumn: "id"},
			// incompatible: timestamptz = uuid
			{Name: "bad", FromSchema: "public", FromTable: "orders", FromColumn: "created_at", ToSchema: "public", ToTable: "customers", ToColumn: "id"},
			// unknown column: fail open
			{Name: "unknown", FromTable: "orders", FromColumn: "missing_col", ToTable: "customers", ToColumn: "id"},
			// incomplete join: skipped
			{Name: "incomplete", FromTable: "orders", FromColumn: "", ToTable: "customers", ToColumn: "id"},
		},
	}

	errs := ValidateJoinColumnTypes(model, lookup)
	if len(errs) != 1 {
		t.Fatalf("expected exactly 1 validation error, got %d: %v", len(errs), errs)
	}
	if errs[0].Code != "INCOMPATIBLE_JOIN_COLUMN_TYPES" {
		t.Fatalf("unexpected code: %s", errs[0].Code)
	}
	if errs[0].Value != "bad" {
		t.Fatalf("expected offending join name 'bad', got %q", errs[0].Value)
	}

	if errs := ValidateJoinColumnTypes(nil, lookup); errs != nil {
		t.Fatalf("nil model should validate clean, got %v", errs)
	}
	if errs := ValidateJoinColumnTypes(model, nil); errs != nil {
		t.Fatalf("nil lookup should validate clean, got %v", errs)
	}
}
