package query

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/biqly/biqly/internal/dialect"
	"github.com/biqly/biqly/internal/security"
	"github.com/biqly/biqly/internal/semantic"
)

// normalizeSQL removes whitespace differences and SQL comments for comparison.
func normalizeSQL(sql string) string {
	// Strip SQL comments (-- to end of line)
	lines := strings.Split(sql, "\n")
	var cleaned []string
	for _, line := range lines {
		if idx := strings.Index(line, "--"); idx >= 0 {
			line = line[:idx]
		}
		line = strings.TrimSpace(line)
		if line != "" {
			cleaned = append(cleaned, line)
		}
	}
	sql = strings.Join(cleaned, " ")
	fields := strings.Fields(sql)
	return strings.Join(fields, " ")
}

// semanticContextFixture provides a rich semantic model for golden tests
// with multiple relationship types, display dimensions, and fanout scenarios.
type semanticContextFixture struct {
	Model      *semantic.SemanticModel
	LogicalQuery LogicalQuery
}

// fixtureManyToMany returns a semantic model with many-to-many relationships
// (students <-> courses via enrollments).
func fixtureManyToMany() semanticContextFixture {
	return semanticContextFixture{
		Model: &semantic.SemanticModel{
			Name:       "student_courses",
			BaseSchema: "public",
			BaseTable:  "students",
			Dimensions: []semantic.Dimension{
				{Name: "student_name", ColumnRef: "students.name", Type: "text", Label: ptrStr("Student Name")},
				{Name: "student_id", ColumnRef: "students.id", Type: "number"},
				{Name: "course_title", ColumnRef: "courses.title", Type: "text", Label: ptrStr("Course Title")},
				{Name: "course_id", ColumnRef: "courses.id", Type: "number"},
				{Name: "course_credits", ColumnRef: "courses.credits", Type: "number"},
			},
			Metrics: []semantic.Metric{
				{Name: "enrollment_count", Expression: "enrollments.id", Aggregation: "count"},
			},
			Joins: []semantic.Join{
				{
					Name:         "students_enrollments",
					FromTable:    "students",
					FromColumn:   "id",
					ToTable:      "enrollments",
					ToColumn:     "student_id",
					JoinType:     "LEFT",
					Relationship: "many_to_many",
				},
				{
					Name:         "enrollments_courses",
					FromTable:    "enrollments",
					FromColumn:   "course_id",
					ToTable:      "courses",
					ToColumn:     "id",
					JoinType:     "LEFT",
					Relationship: "many_to_many",
				},
			},
		},
		LogicalQuery: LogicalQuery{
			ModelID: "student_courses",
			Select: []SelectItem{
				{Type: "dimension", Name: "student_name"},
				{Type: "dimension", Name: "course_title"},
			},
			Filters: []Filter{{Field: "course_credits", Operator: OpGte, Value: 3}},
			OrderBy: []OrderBy{{Field: "student_name", Direction: "asc"}},
			Limit:   100,
		},
	}
}

// fixtureManyToOne returns a semantic model with many-to-one relationship
// (orders -> customers).
func fixtureManyToOne() semanticContextFixture {
	return semanticContextFixture{
		Model: &semantic.SemanticModel{
			Name:       "orders",
			BaseSchema: "public",
			BaseTable:  "orders",
			Dimensions: []semantic.Dimension{
				{Name: "customer_name", ColumnRef: "customers.name", Type: "text", Label: ptrStr("Customer Name")},
				{Name: "customer_id", ColumnRef: "customers.id", Type: "number"},
				{Name: "order_date", ColumnRef: "orders.created_at", Type: "date"},
			},
			Metrics: []semantic.Metric{
				{Name: "order_count", Expression: "orders.id", Aggregation: "count"},
				{Name: "total_revenue", Expression: "orders.total_amount", Aggregation: "sum"},
			},
			Joins: []semantic.Join{
				{
					Name:         "orders_customers",
					FromTable:    "orders",
					FromColumn:   "customer_id",
					ToTable:      "customers",
					ToColumn:     "id",
					JoinType:     "LEFT",
					Relationship: "many_to_one",
				},
			},
		},
		LogicalQuery: LogicalQuery{
			ModelID: "orders",
			Select: []SelectItem{
				{Type: "dimension", Name: "customer_name"},
				{Type: "metric", Name: "order_count"},
			},
			Filters: []Filter{{Field: "order_date", Operator: OpGte, Value: "2024-01-01"}},
			GroupBy: []GroupBy{{Field: "customer_name"}},
			OrderBy: []OrderBy{{Field: "order_count", Direction: "desc"}},
			Limit:   50,
		},
	}
}

// fixtureOneToMany returns a semantic model with one-to-many relationship
// (departments -> employees).
func fixtureOneToMany() semanticContextFixture {
	return semanticContextFixture{
		Model: &semantic.SemanticModel{
			Name:       "departments",
			BaseSchema: "public",
			BaseTable:  "departments",
			Dimensions: []semantic.Dimension{
				{Name: "department_name", ColumnRef: "departments.name", Type: "text", Label: ptrStr("Department Name")},
				{Name: "employee_name", ColumnRef: "employees.name", Type: "text", Label: ptrStr("Employee Name")},
			},
			Metrics: []semantic.Metric{
				{Name: "employee_count", Expression: "employees.id", Aggregation: "count"},
			},
			Joins: []semantic.Join{
				{
					Name:         "departments_employees",
					FromTable:    "departments",
					FromColumn:   "id",
					ToTable:      "employees",
					ToColumn:     "department_id",
					JoinType:     "LEFT",
					Relationship: "one_to_many",
				},
			},
		},
		LogicalQuery: LogicalQuery{
			ModelID: "departments",
			Select: []SelectItem{
				{Type: "dimension", Name: "department_name"},
				{Type: "metric", Name: "employee_count"},
			},
			GroupBy: []GroupBy{{Field: "department_name"}},
			OrderBy: []OrderBy{{Field: "employee_count", Direction: "desc"}},
			Limit:   20,
		},
	}
}

// fixtureOneToOne returns a semantic model with one-to-one relationship
// (users <-> user_profiles).
func fixtureOneToOne() semanticContextFixture {
	return semanticContextFixture{
		Model: &semantic.SemanticModel{
			Name:       "users",
			BaseSchema: "public",
			BaseTable:  "users",
			Dimensions: []semantic.Dimension{
				{Name: "email", ColumnRef: "users.email", Type: "text"},
				{Name: "bio", ColumnRef: "user_profiles.bio", Type: "text"},
				{Name: "is_active", ColumnRef: "users.is_active", Type: "boolean"},
			},
			Joins: []semantic.Join{
				{
					Name:         "users_profiles",
					FromTable:    "users",
					FromColumn:   "id",
					ToTable:      "user_profiles",
					ToColumn:     "user_id",
					JoinType:     "LEFT",
					Relationship: "one_to_one",
				},
			},
		},
		LogicalQuery: LogicalQuery{
			ModelID: "users",
			Select: []SelectItem{
				{Type: "dimension", Name: "email"},
				{Type: "dimension", Name: "bio"},
			},
			Filters: []Filter{{Field: "is_active", Operator: OpEq, Value: true}},
			Limit:   50,
		},
	}
}

// fixtureMultiHop returns a semantic model with multi-hop joins
// (orders -> customers, orders -> order_items -> products).
func fixtureMultiHop() semanticContextFixture {
	return semanticContextFixture{
		Model: &semantic.SemanticModel{
			Name:       "order_details",
			BaseSchema: "public",
			BaseTable:  "orders",
			Dimensions: []semantic.Dimension{
				{Name: "customer_name", ColumnRef: "customers.name", Type: "text", Label: ptrStr("Customer Name")},
				{Name: "product_name", ColumnRef: "products.name", Type: "text", Label: ptrStr("Product Name")},
				{Name: "order_status", ColumnRef: "orders.status", Type: "text"},
			},
			Metrics: []semantic.Metric{
				{Name: "item_count", Expression: "order_items.id", Aggregation: "count"},
			},
			Joins: []semantic.Join{
				{
					Name:         "orders_customers",
					FromTable:    "orders",
					FromColumn:   "customer_id",
					ToTable:      "customers",
					ToColumn:     "id",
					JoinType:     "LEFT",
					Relationship: "many_to_one",
				},
				{
					Name:         "orders_order_items",
					FromTable:    "orders",
					FromColumn:   "id",
					ToTable:      "order_items",
					ToColumn:     "order_id",
					JoinType:     "LEFT",
					Relationship: "one_to_many",
				},
				{
					Name:         "order_items_products",
					FromTable:    "order_items",
					FromColumn:   "product_id",
					ToTable:      "products",
					ToColumn:     "id",
					JoinType:     "LEFT",
					Relationship: "many_to_one",
				},
			},
		},
		LogicalQuery: LogicalQuery{
			ModelID: "order_details",
			Select: []SelectItem{
				{Type: "dimension", Name: "customer_name"},
				{Type: "dimension", Name: "product_name"},
				{Type: "metric", Name: "item_count"},
			},
			Filters: []Filter{{Field: "order_status", Operator: OpEq, Value: "completed"}},
			GroupBy: []GroupBy{{Field: "customer_name"}, {Field: "product_name"}},
			OrderBy: []OrderBy{{Field: "item_count", Direction: "desc"}},
			Limit:   100,
		},
	}
}

// fixtureDisplayPriority returns a semantic model that tests display dimension
// priority over identifier columns.
func fixtureDisplayPriority() semanticContextFixture {
	return semanticContextFixture{
		Model: &semantic.SemanticModel{
			Name:       "customers",
			BaseSchema: "public",
			BaseTable:  "customers",
			Dimensions: []semantic.Dimension{
				{Name: "name", ColumnRef: "customers.name", Type: "text", Label: ptrStr("Customer Name"), Synonyms: []string{"customer", "müşteri"}},
				{Name: "email", ColumnRef: "customers.email", Type: "text"},
				{Name: "customer_id", ColumnRef: "customers.id", Type: "number"},
				{Name: "external_key", ColumnRef: "customers.external_key", Type: "text"},
			},
			Metrics: []semantic.Metric{
				{Name: "order_count", Expression: "orders.id", Aggregation: "count"},
			},
			Joins: []semantic.Join{
				{
					Name:         "customers_orders",
					FromTable:    "customers",
					FromColumn:   "id",
					ToTable:      "orders",
					ToColumn:     "customer_id",
					JoinType:     "LEFT",
					Relationship: "one_to_many",
				},
			},
		},
		LogicalQuery: LogicalQuery{
			ModelID: "customers",
			Select: []SelectItem{
				{Type: "dimension", Name: "name"},
				{Type: "dimension", Name: "email"},
				{Type: "metric", Name: "order_count"},
			},
			GroupBy: []GroupBy{{Field: "name"}, {Field: "email"}},
			OrderBy: []OrderBy{{Field: "order_count", Direction: "desc"}},
			Limit:   50,
		},
	}
}

func ptrStr(s string) *string { return &s }

// fixtureCalculated returns a semantic model with calculated dimensions.
func fixtureCalculated() semanticContextFixture {
	return semanticContextFixture{
		Model: &semantic.SemanticModel{
			Name:       "orders",
			BaseSchema: "public",
			BaseTable:  "orders",
			Dimensions: []semantic.Dimension{
				{Name: "full_name", CalculatedExpression: `COALESCE(orders.first_name, '') || ' ' || COALESCE(orders.last_name, '')`, Type: "text"},
				{Name: "first_name", ColumnRef: "orders.first_name", Type: "text"},
				{Name: "last_name", ColumnRef: "orders.last_name", Type: "text"},
			},
			Metrics: []semantic.Metric{
				{Name: "order_count", Expression: "orders.id", Aggregation: "count"},
			},
		},
		LogicalQuery: LogicalQuery{
			ModelID: "orders",
			Select: []SelectItem{
				{Type: "dimension", Name: "full_name"},
				{Type: "metric", Name: "order_count"},
			},
			GroupBy: []GroupBy{{Field: "full_name"}},
			OrderBy: []OrderBy{{Field: "order_count", Direction: "desc"}},
			Limit:   50,
		},
	}
}

// TestGolden_PostgresSimpleSelect verifies compiler output matches golden SQL.
func TestGolden_PostgresSimpleSelect(t *testing.T) {
	golden, err := os.ReadFile(filepath.Join("..", "..", "testdata", "sql", "postgres", "group_by_metric.sql"))
	if err != nil {
		t.Skip("golden file not found")
	}

	model := &semantic.SemanticModel{
		Name:       "sales_orders",
		BaseSchema: "sales",
		BaseTable:  "salesorderheader",
		Dimensions: []semantic.Dimension{
			{Name: "country", ColumnRef: "salesterritory.countryregioncode", Type: "text"},
			{Name: "order_date", ColumnRef: "salesorderheader.orderdate", Type: "date"},
		},
		Metrics: []semantic.Metric{
			{Name: "order_count", Expression: "salesorderheader.salesorderid", Aggregation: "count"},
		},
		Joins: []semantic.Join{
			{
				Name:         "salesorderheader_salesterritory",
				FromTable:    "salesorderheader",
				FromColumn:   "territoryid",
				ToTable:      "salesterritory",
				ToColumn:     "territoryid",
				JoinType:     "LEFT",
				Relationship: "many_to_one",
			},
		},
	}

	lq := LogicalQuery{
		ModelID: "sales_orders",
		Select: []SelectItem{
			{Type: "dimension", Name: "country"},
			{Type: "metric", Name: "order_count"},
		},
		Filters: []Filter{{Field: "order_date", Operator: OpGte, Value: "2011-01-01"}},
		GroupBy: []GroupBy{{Field: "country"}},
		OrderBy: []OrderBy{{Field: "order_count", Direction: "desc"}},
		Limit:   100,
	}

	compiler := NewCompiler(dialect.PostgresDialect{})
	cq, err := compiler.Compile(context.Background(), lq, model)
	if err != nil {
		t.Fatalf("compilation failed: %v", err)
	}

	expected := normalizeSQL(string(golden))
	actual := normalizeSQL(cq.SQL)

	if expected != actual {
		t.Errorf("SQL mismatch.\nExpected:\n%s\n\nGot:\n%s", expected, actual)
	}
}

// TestGolden_MySQLSimpleSelect verifies MySQL compiler output.
func TestGolden_MySQLSimpleSelect(t *testing.T) {
	golden, err := os.ReadFile(filepath.Join("..", "..", "testdata", "sql", "mysql", "simple_select.sql"))
	if err != nil {
		t.Skip("golden file not found")
	}

	model := &semantic.SemanticModel{
		Name:       "products",
		BaseSchema: "store",
		BaseTable:  "products",
		Dimensions: []semantic.Dimension{
			{Name: "name", ColumnRef: "products.name", Type: "text"},
		},
	}

	lq := LogicalQuery{
		ModelID: "products",
		Select:  []SelectItem{{Type: "dimension", Name: "name"}},
		Filters: []Filter{{Field: "name", Operator: OpContains, Value: "test"}},
		Limit:   50,
	}

	compiler := NewCompiler(dialect.MySQLDialect{})
	cq, err := compiler.Compile(context.Background(), lq, model)
	if err != nil {
		t.Fatalf("MySQL compilation failed: %v", err)
	}

	expected := normalizeSQL(string(golden))
	actual := normalizeSQL(cq.SQL)

	if expected != actual {
		t.Errorf("MySQL SQL mismatch.\nExpected:\n%s\n\nGot:\n%s", expected, actual)
	}
}

// TestGolden_PostgresManyToOne verifies many-to-one join with display dimension.
func TestGolden_PostgresManyToOne(t *testing.T) {
	golden, err := os.ReadFile(filepath.Join("..", "..", "testdata", "sql", "postgres", "many_to_one_join.sql"))
	if err != nil {
		t.Skip("golden file not found")
	}

	fixture := fixtureManyToOne()
	compiler := NewCompiler(dialect.PostgresDialect{})
	cq, err := compiler.Compile(context.Background(), fixture.LogicalQuery, fixture.Model)
	if err != nil {
		t.Fatalf("compilation failed: %v", err)
	}

	expected := normalizeSQL(string(golden))
	actual := normalizeSQL(cq.SQL)

	if expected != actual {
		t.Errorf("SQL mismatch.\nExpected:\n%s\n\nGot:\n%s", expected, actual)
	}
}

// TestGolden_PostgresOneToMany verifies one-to-many join with fanout.
func TestGolden_PostgresOneToMany(t *testing.T) {
	golden, err := os.ReadFile(filepath.Join("..", "..", "testdata", "sql", "postgres", "one_to_many_join.sql"))
	if err != nil {
		t.Skip("golden file not found")
	}

	fixture := fixtureOneToMany()
	compiler := NewCompiler(dialect.PostgresDialect{})
	cq, err := compiler.Compile(context.Background(), fixture.LogicalQuery, fixture.Model)
	if err != nil {
		t.Fatalf("compilation failed: %v", err)
	}

	expected := normalizeSQL(string(golden))
	actual := normalizeSQL(cq.SQL)

	if expected != actual {
		t.Errorf("SQL mismatch.\nExpected:\n%s\n\nGot:\n%s", expected, actual)
	}
}

// TestGolden_PostgresOneToOne verifies one-to-one join without fanout.
func TestGolden_PostgresOneToOne(t *testing.T) {
	golden, err := os.ReadFile(filepath.Join("..", "..", "testdata", "sql", "postgres", "one_to_one_join.sql"))
	if err != nil {
		t.Skip("golden file not found")
	}

	fixture := fixtureOneToOne()
	compiler := NewCompiler(dialect.PostgresDialect{})
	cq, err := compiler.Compile(context.Background(), fixture.LogicalQuery, fixture.Model)
	if err != nil {
		t.Fatalf("compilation failed: %v", err)
	}

	expected := normalizeSQL(string(golden))
	actual := normalizeSQL(cq.SQL)

	if expected != actual {
		t.Errorf("SQL mismatch.\nExpected:\n%s\n\nGot:\n%s", expected, actual)
	}
}

// TestGolden_PostgresManyToMany verifies many-to-many join with fanout warning.
func TestGolden_PostgresManyToMany(t *testing.T) {
	golden, err := os.ReadFile(filepath.Join("..", "..", "testdata", "sql", "postgres", "many_to_many_join.sql"))
	if err != nil {
		t.Skip("golden file not found")
	}

	fixture := fixtureManyToMany()
	compiler := NewCompiler(dialect.PostgresDialect{})
	cq, err := compiler.Compile(context.Background(), fixture.LogicalQuery, fixture.Model)
	if err != nil {
		t.Fatalf("compilation failed: %v", err)
	}

	expected := normalizeSQL(string(golden))
	actual := normalizeSQL(cq.SQL)

	if expected != actual {
		t.Errorf("SQL mismatch.\nExpected:\n%s\n\nGot:\n%s", expected, actual)
	}
}

// TestGolden_PostgresMultiHop verifies multi-hop join with mixed relationship types.
func TestGolden_PostgresMultiHop(t *testing.T) {
	golden, err := os.ReadFile(filepath.Join("..", "..", "testdata", "sql", "postgres", "multi_hop_join.sql"))
	if err != nil {
		t.Skip("golden file not found")
	}

	fixture := fixtureMultiHop()
	compiler := NewCompiler(dialect.PostgresDialect{})
	cq, err := compiler.Compile(context.Background(), fixture.LogicalQuery, fixture.Model)
	if err != nil {
		t.Fatalf("compilation failed: %v", err)
	}

	expected := normalizeSQL(string(golden))
	actual := normalizeSQL(cq.SQL)

	if expected != actual {
		t.Errorf("SQL mismatch.\nExpected:\n%s\n\nGot:\n%s", expected, actual)
	}
}

// TestGolden_PostgresDisplayPriority verifies display columns preferred over IDs.
func TestGolden_PostgresDisplayPriority(t *testing.T) {
	golden, err := os.ReadFile(filepath.Join("..", "..", "testdata", "sql", "postgres", "display_dimension_priority.sql"))
	if err != nil {
		t.Skip("golden file not found")
	}

	fixture := fixtureDisplayPriority()
	compiler := NewCompiler(dialect.PostgresDialect{})
	cq, err := compiler.Compile(context.Background(), fixture.LogicalQuery, fixture.Model)
	if err != nil {
		t.Fatalf("compilation failed: %v", err)
	}

	expected := normalizeSQL(string(golden))
	actual := normalizeSQL(cq.SQL)

	if expected != actual {
		t.Errorf("SQL mismatch.\nExpected:\n%s\n\nGot:\n%s", expected, actual)
	}
}

// TestGolden_MySQLManyToOne verifies MySQL many-to-one join.
func TestGolden_MySQLManyToOne(t *testing.T) {
	golden, err := os.ReadFile(filepath.Join("..", "..", "testdata", "sql", "mysql", "many_to_one_join.sql"))
	if err != nil {
		t.Skip("golden file not found")
	}

	fixture := fixtureManyToOne()
	compiler := NewCompiler(dialect.MySQLDialect{})
	cq, err := compiler.Compile(context.Background(), fixture.LogicalQuery, fixture.Model)
	if err != nil {
		t.Fatalf("compilation failed: %v", err)
	}

	expected := normalizeSQL(string(golden))
	actual := normalizeSQL(cq.SQL)

	if expected != actual {
		t.Errorf("MySQL SQL mismatch.\nExpected:\n%s\n\nGot:\n%s", expected, actual)
	}
}

// TestGolden_SQLServerManyToOne verifies SQL Server many-to-one join.
func TestGolden_SQLServerManyToOne(t *testing.T) {
	golden, err := os.ReadFile(filepath.Join("..", "..", "testdata", "sql", "sqlserver", "many_to_one_join.sql"))
	if err != nil {
		t.Skip("golden file not found")
	}

	fixture := fixtureManyToOne()
	compiler := NewCompiler(dialect.SQLServerDialect{})
	cq, err := compiler.Compile(context.Background(), fixture.LogicalQuery, fixture.Model)
	if err != nil {
		t.Fatalf("compilation failed: %v", err)
	}

	expected := normalizeSQL(string(golden))
	actual := normalizeSQL(cq.SQL)

	if expected != actual {
		t.Errorf("SQL Server SQL mismatch.\nExpected:\n%s\n\nGot:\n%s", expected, actual)
	}
}

// TestPlanner_FanoutDetection verifies the planner detects fanout risks.
func TestPlanner_FanoutDetection(t *testing.T) {
	tests := []struct {
		name            string
		fixture         func() semanticContextFixture
		expectWarnings  bool
	}{
		{
			name:           "many_to_one_no_fanout",
			fixture:        fixtureManyToOne,
			expectWarnings: false,
		},
		{
			name:           "one_to_one_no_fanout",
			fixture:        fixtureOneToOne,
			expectWarnings: false,
		},
		{
			name:           "many_to_many_fanout",
			fixture:        fixtureManyToMany,
			expectWarnings: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fixture := tt.fixture()
			planner := NewPlanner()
			result, err := planner.Plan(fixture.LogicalQuery, fixture.Model)
			if err != nil {
				t.Fatalf("planning failed: %v", err)
			}

			hasWarnings := len(result.Warnings) > 0
			if hasWarnings != tt.expectWarnings {
				t.Errorf("expected warnings=%v, got warnings=%v (%v)", tt.expectWarnings, hasWarnings, result.Warnings)
			}
		})
	}
}

// TestValidator_FanoutRelationship verifies validator catches fanout risks.
func TestValidator_FanoutRelationship(t *testing.T) {
	// This tests the semantic publish validator, not the query validator.
	// The publish validator already warns on one_to_many and many_to_many.
	// Here we verify the query planner propagates those warnings correctly.

	fixture := fixtureManyToMany()
	planner := NewPlanner()
	result, err := planner.Plan(fixture.LogicalQuery, fixture.Model)
	if err != nil {
		t.Fatalf("planning failed: %v", err)
	}

	foundFanout := false
	for _, w := range result.Warnings {
		if strings.Contains(strings.ToLower(w), "many-to-many") || strings.Contains(strings.ToLower(w), "fanout") {
			foundFanout = true
			break
		}
	}
	if !foundFanout {
		t.Errorf("expected fanout warning for many-to-many join, got: %v", result.Warnings)
	}
}

// TestGolden_PostgresCalculatedDimension verifies calculated dimensions.
func TestGolden_PostgresCalculatedDimension(t *testing.T) {
	golden, err := os.ReadFile(filepath.Join("..", "..", "testdata", "sql", "postgres", "calculated_dimension.sql"))
	if err != nil {
		t.Skip("golden file not found")
	}

	fixture := fixtureCalculated()
	compiler := NewCompiler(dialect.PostgresDialect{})
	cq, err := compiler.Compile(context.Background(), fixture.LogicalQuery, fixture.Model)
	if err != nil {
		t.Fatalf("compilation failed: %v", err)
	}

	expected := normalizeSQL(string(golden))
	actual := normalizeSQL(cq.SQL)

	if expected != actual {
		t.Errorf("SQL mismatch.\nExpected:\n%s\n\nGot:\n%s", expected, actual)
	}
}

// TestGolden_RowFilterInjection verifies row-level security filters are
// correctly injected into compiled SQL (golden test for permission injection).
func TestGolden_RowFilterInjection_Eq(t *testing.T) {
	golden, err := os.ReadFile(filepath.Join("..", "..", "testdata", "sql", "postgres", "row_filter_eq.sql"))
	if err != nil {
		t.Skip("golden file not found")
	}

	fixture := fixtureManyToOne()
	compiler := NewCompiler(dialect.PostgresDialect{})
	// Use a field that exists in the fixture: order_date -> orders.created_at
	rowFilters := []security.RowFilter{
		{Field: "order_date", Operator: "eq", Value: "2024-01-01"},
	}

	cq, err := compiler.CompileWithPermissions(context.Background(), fixture.LogicalQuery, fixture.Model, rowFilters)
	if err != nil {
		t.Fatalf("compilation with permissions failed: %v", err)
	}

	expected := normalizeSQL(string(golden))
	actual := normalizeSQL(cq.SQL)

	if expected != actual {
		t.Errorf("Row filter injection SQL mismatch.\nExpected:\n%s\n\nGot:\n%s", expected, actual)
	}
}

func TestGolden_RowFilterInjection_In(t *testing.T) {
	golden, err := os.ReadFile(filepath.Join("..", "..", "testdata", "sql", "postgres", "row_filter_in.sql"))
	if err != nil {
		t.Skip("golden file not found")
	}

	fixture := fixtureManyToOne()
	compiler := NewCompiler(dialect.PostgresDialect{})
	rowFilters := []security.RowFilter{
		{Field: "order_date", Operator: "in", Value: []any{"2024-01-01", "2024-02-01", "2024-03-01"}},
	}

	cq, err := compiler.CompileWithPermissions(context.Background(), fixture.LogicalQuery, fixture.Model, rowFilters)
	if err != nil {
		t.Fatalf("compilation with permissions failed: %v", err)
	}

	expected := normalizeSQL(string(golden))
	actual := normalizeSQL(cq.SQL)

	if expected != actual {
		t.Errorf("Row filter IN injection SQL mismatch.\nExpected:\n%s\n\nGot:\n%s", expected, actual)
	}
}

func TestGolden_RowFilterInjection_WithExistingWhere(t *testing.T) {
	fixture := fixtureManyToOne()
	compiler := NewCompiler(dialect.PostgresDialect{})
	rowFilters := []security.RowFilter{
		{Field: "customer_name", Operator: "eq", Value: "acme"},
	}

	lq := fixture.LogicalQuery
	lq.Filters = append(lq.Filters, Filter{Field: "order_date", Operator: OpGte, Value: "2024-01-01"})

	cq, err := compiler.CompileWithPermissions(context.Background(), lq, fixture.Model, rowFilters)
	if err != nil {
		t.Fatalf("compilation with permissions failed: %v", err)
	}

	// Should contain both the original WHERE and the row filter
	sql := strings.ToUpper(cq.SQL)
	if !strings.Contains(sql, "WHERE") {
		t.Fatalf("expected WHERE clause, got: %s", cq.SQL)
	}
	// Both filters should be ANDed
	if !strings.Contains(sql, "AND") {
		t.Errorf("expected AND combining filters, got: %s", cq.SQL)
	}
}

func TestGolden_RowFilterInjection_NoFilters(t *testing.T) {
	fixture := fixtureManyToOne()
	compiler := NewCompiler(dialect.PostgresDialect{})

	cq, err := compiler.CompileWithPermissions(context.Background(), fixture.LogicalQuery, fixture.Model, nil)
	if err != nil {
		t.Fatalf("compilation with permissions failed: %v", err)
	}

	// Without row filters, should produce same SQL as normal compile
	cqNormal, err := compiler.Compile(context.Background(), fixture.LogicalQuery, fixture.Model)
	if err != nil {
		t.Fatalf("normal compilation failed: %v", err)
	}

	if cq.SQL != cqNormal.SQL {
		t.Errorf("expected same SQL without row filters.\nNormal:\n%s\n\nWithPermissions:\n%s", cqNormal.SQL, cq.SQL)
	}
}
