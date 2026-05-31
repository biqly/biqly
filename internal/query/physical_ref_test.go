package query

import (
	"testing"

	"github.com/biqly/biqly/internal/dialect"
	"github.com/biqly/biqly/internal/semantic"
)

func TestParseColumnRef(t *testing.T) {
	p, ok := ParseColumnRef("person.person.firstname", "sales")
	if !ok || p.Schema != "person" || p.Table != "person" || p.Column != "firstname" {
		t.Fatalf("unexpected parse: %+v ok=%v", p, ok)
	}
	p, ok = ParseColumnRef("customer.accountnumber", "sales")
	if !ok || p.Schema != "sales" || p.Table != "customer" || p.Column != "accountnumber" {
		t.Fatalf("unexpected parse: %+v ok=%v", p, ok)
	}
}

func TestCompiler_CrossSchemaJoin(t *testing.T) {
	model := &semantic.SemanticModel{
		Name:       "sales_customer_person",
		BaseSchema: "sales",
		BaseTable:  "customer",
		Dimensions: []semantic.Dimension{
			{Name: "first_name", ColumnRef: "person.person.firstname", Type: "text"},
			{Name: "account_number", ColumnRef: "customer.accountnumber", Type: "text"},
		},
		Joins: []semantic.Join{
			{
				Name:       "customer_person",
				FromSchema: "sales",
				FromTable:  "customer",
				FromColumn: "personid",
				ToSchema:   "person",
				ToTable:    "person",
				ToColumn:   "businessentityid",
				JoinType:   "LEFT",
			},
		},
	}

	lq := LogicalQuery{
		Select: []SelectItem{
			{Type: SelectTypeDimension, Name: "first_name"},
			{Type: SelectTypeDimension, Name: "account_number"},
		},
		Limit: 10,
	}

	cq, err := NewCompiler(dialect.PostgresDialect{}).Compile(t.Context(), &lq, model)
	if err != nil {
		t.Fatalf("Compile() error = %v", err)
	}
	if !containsStr(cq.SQL, `"person"."person"`) {
		t.Fatalf("expected person schema in JOIN, got:\n%s", cq.SQL)
	}
	if !containsStr(cq.SQL, `"sales"."customer"`) {
		t.Fatalf("expected sales.customer in FROM, got:\n%s", cq.SQL)
	}
}

func TestSchemaResolver_TableSchemasOverride(t *testing.T) {
	model := &semantic.SemanticModel{BaseSchema: "sales", BaseTable: "orders"}
	lq := LogicalQuery{TableSchemas: map[string]string{"orders": "archive"}}
	r := NewSchemaResolver(model, &lq)
	if got := r.SchemaForTable("orders"); got != "archive" {
		t.Fatalf("SchemaForTable() = %q, want archive", got)
	}
	ref := r.PhysicalColumnRef("orders.total")
	if ref != "archive.orders.total" {
		t.Fatalf("PhysicalColumnRef() = %q, want archive.orders.total", ref)
	}
}

func TestPhysicalColumnRef_SameSchemaUsesTwoPart(t *testing.T) {
	model := &semantic.SemanticModel{BaseSchema: "public", BaseTable: "orders"}
	r := NewSchemaResolver(model, nil)
	if got := r.PhysicalColumnRef("customers.name"); got != "customers.name" {
		t.Fatalf("PhysicalColumnRef() = %q, want customers.name", got)
	}
}
