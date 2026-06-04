package query

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/biqly/biqly/internal/dialect"
	"github.com/biqly/biqly/internal/security"
	"github.com/biqly/biqly/internal/security/pii"
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
	Model        *semantic.SemanticModel
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

// fixtureSimpleSelect returns a simple select query structure.
func fixtureSimpleSelect() semanticContextFixture {
	return semanticContextFixture{
		Model: &semantic.SemanticModel{
			Name:       "sales_orders_simple",
			BaseSchema: "sales",
			BaseTable:  "salesorderheader",
			Dimensions: []semantic.Dimension{
				{Name: "order_number", ColumnRef: "salesorderheader.salesordernumber", Type: "text"},
				{Name: "total_due", ColumnRef: "salesorderheader.totaldue", Type: "number"},
			},
		},
		LogicalQuery: LogicalQuery{
			ModelID: "sales_orders_simple",
			Select: []SelectItem{
				{Type: "dimension", Name: "order_number"},
				{Type: "dimension", Name: "total_due"},
			},
			Filters: []Filter{{Field: "total_due", Operator: OpGte, Value: 100.0}},
			Limit:   50,
		},
	}
}

// fixtureGroupByMetric returns a query structure with a join and group by metric.
func fixtureGroupByMetric() semanticContextFixture {
	return semanticContextFixture{
		Model: &semantic.SemanticModel{
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
		},
		LogicalQuery: LogicalQuery{
			ModelID: "sales_orders",
			Select: []SelectItem{
				{Type: "dimension", Name: "country"},
				{Type: "metric", Name: "order_count"},
			},
			Filters: []Filter{{Field: "order_date", Operator: OpGte, Value: "2011-01-01"}},
			GroupBy: []GroupBy{{Field: "country"}},
			OrderBy: []OrderBy{{Field: "order_count", Direction: "desc"}},
			Limit:   100,
		},
	}
}

func fixtureComposite(t *testing.T) semanticContextFixture {
	return semanticContextFixture{
		Model: compositeGoldenModel(t),
		LogicalQuery: LogicalQuery{
			ModelID: "c-1",
			Select: []SelectItem{
				{Type: "dimension", Name: "customer_region"},
				{Type: "metric", Name: "total_revenue"},
			},
			GroupBy: []GroupBy{{Field: "customer_region"}},
			OrderBy: []OrderBy{{Field: "total_revenue", Direction: "desc"}},
			Limit:   50,
		},
	}
}

type goldenTestCase struct {
	name       string
	fixture    func(t *testing.T) semanticContextFixture
	rowFilters []security.RowFilter
}

func getGoldenTestCases() []goldenTestCase {
	return []goldenTestCase{
		{
			name: "simple_select",
			fixture: func(t *testing.T) semanticContextFixture {
				return fixtureSimpleSelect()
			},
		},
		{
			name: "group_by_metric",
			fixture: func(t *testing.T) semanticContextFixture {
				return fixtureGroupByMetric()
			},
		},
		{
			name: "many_to_one_join",
			fixture: func(t *testing.T) semanticContextFixture {
				return fixtureManyToOne()
			},
		},
		{
			name: "one_to_many_join",
			fixture: func(t *testing.T) semanticContextFixture {
				return fixtureOneToMany()
			},
		},
		{
			name: "one_to_one_join",
			fixture: func(t *testing.T) semanticContextFixture {
				return fixtureOneToOne()
			},
		},
		{
			name: "many_to_many_join",
			fixture: func(t *testing.T) semanticContextFixture {
				return fixtureManyToMany()
			},
		},
		{
			name: "multi_hop_join",
			fixture: func(t *testing.T) semanticContextFixture {
				return fixtureMultiHop()
			},
		},
		{
			name: "display_dimension_priority",
			fixture: func(t *testing.T) semanticContextFixture {
				return fixtureDisplayPriority()
			},
		},
		{
			name: "calculated_dimension",
			fixture: func(t *testing.T) semanticContextFixture {
				return fixtureCalculated()
			},
		},
		{
			name:    "composite_cross_model",
			fixture: fixtureComposite,
		},
		{
			name: "row_filter_eq",
			fixture: func(t *testing.T) semanticContextFixture {
				return fixtureManyToOne()
			},
			rowFilters: []security.RowFilter{
				{Field: "order_date", Operator: "eq", Value: "2024-01-01"},
			},
		},
		{
			name: "row_filter_in",
			fixture: func(t *testing.T) semanticContextFixture {
				return fixtureManyToOne()
			},
			rowFilters: []security.RowFilter{
				{Field: "order_date", Operator: "in", Value: []any{"2024-01-01", "2024-02-01", "2024-03-01"}},
			},
		},
	}
}

func TestGoldenAcrossDialects(t *testing.T) {
	dialects := []dialect.Dialect{
		dialect.PostgresDialect{},
		dialect.MySQLDialect{},
		dialect.SQLServerDialect{},
	}

	updateGolden := os.Getenv("UPDATE_GOLDEN") == "true"

	for _, d := range dialects {
		t.Run(d.Name(), func(t *testing.T) {
			for _, tc := range getGoldenTestCases() {
				t.Run(tc.name, func(t *testing.T) {
					fixture := tc.fixture(t)
					compiler := NewCompiler(d)
					var cq *CompiledQuery
					var err error
					if len(tc.rowFilters) > 0 {
						cq, err = compiler.CompileWithPermissions(context.Background(), &fixture.LogicalQuery, fixture.Model, tc.rowFilters, nil)
					} else {
						cq, err = compiler.Compile(context.Background(), &fixture.LogicalQuery, fixture.Model)
					}
					if err != nil {
						t.Fatalf("compilation failed: %v", err)
					}

					goldenDir := filepath.Join("..", "..", "testdata", "sql", d.Name())
					goldenPath := filepath.Join(goldenDir, tc.name+".sql")

					if updateGolden {
						if err := os.MkdirAll(goldenDir, 0750); err != nil {
							t.Fatalf("failed to create golden directory: %v", err)
						}
						if err := os.WriteFile(goldenPath, []byte(cq.SQL), 0600); err != nil {
							t.Fatalf("failed to write golden file: %v", err)
						}
					}

					// #nosec G304
					goldenBytes, err := os.ReadFile(goldenPath)
					if err != nil {
						t.Fatalf("failed to read golden file: %v", err)
					}

					expected := normalizeSQL(string(goldenBytes))
					actual := normalizeSQL(cq.SQL)

					if expected != actual {
						t.Errorf("SQL mismatch for %s/%s.\nExpected:\n%s\n\nGot:\n%s", d.Name(), tc.name, expected, actual)
					}
				})
			}
		})
	}
}

// TestPlanner_FanoutDetection verifies the planner detects fanout risks.
func TestPlanner_FanoutDetection(t *testing.T) {
	tests := []struct {
		name           string
		fixture        func() semanticContextFixture
		expectWarnings bool
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
			result, err := planner.Plan(&fixture.LogicalQuery, fixture.Model)
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
	result, err := planner.Plan(&fixture.LogicalQuery, fixture.Model)
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

// TestGolden_RowFilterInjection_WithExistingWhere verifies RowFilter injection when WHERE already exists.
func TestGolden_RowFilterInjection_WithExistingWhere(t *testing.T) {
	fixture := fixtureManyToOne()
	compiler := NewCompiler(dialect.PostgresDialect{})
	rowFilters := []security.RowFilter{
		{Field: "customer_name", Operator: "eq", Value: "acme"},
	}

	lq := fixture.LogicalQuery
	lq.Filters = append(lq.Filters, Filter{Field: "order_date", Operator: OpGte, Value: "2024-01-01"})

	cq, err := compiler.CompileWithPermissions(context.Background(), &lq, fixture.Model, rowFilters, nil)
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

// TestGolden_RowFilterInjection_NoFilters verifies compilation when no filters are present.
func TestGolden_RowFilterInjection_NoFilters(t *testing.T) {
	fixture := fixtureManyToOne()
	compiler := NewCompiler(dialect.PostgresDialect{})

	cq, err := compiler.CompileWithPermissions(context.Background(), &fixture.LogicalQuery, fixture.Model, nil, nil)
	if err != nil {
		t.Fatalf("compilation with permissions failed: %v", err)
	}

	// Without row filters, should produce same SQL as normal compile
	cqNormal, err := compiler.Compile(context.Background(), &fixture.LogicalQuery, fixture.Model)
	if err != nil {
		t.Fatalf("normal compilation failed: %v", err)
	}

	if cq.SQL != cqNormal.SQL {
		t.Errorf("expected same SQL without row filters.\nNormal:\n%s\n\nGot:\n%s", cqNormal.SQL, cq.SQL)
	}
}

// compositeGoldenModel resolves a two-component composite (orders primary +
// customers secondary, joined on customer_id) into a merged SemanticModel.
func compositeGoldenModel(t *testing.T) *semantic.SemanticModel {
	t.Helper()
	orders := &semantic.SemanticModel{
		ID: "m-orders", Name: "orders", BaseSchema: "public", BaseTable: "orders",
		Status: "published",
		Dimensions: []semantic.Dimension{
			{Name: "customer_id", ColumnRef: "customer_id", Type: "number"},
			{Name: "order_date", ColumnRef: "created_at", Type: "date"},
		},
		Metrics: []semantic.Metric{
			{Name: "total_revenue", Expression: "total_amount", Aggregation: "sum"},
		},
	}
	customers := &semantic.SemanticModel{
		ID: "m-customers", Name: "customers", BaseSchema: "public", BaseTable: "customers",
		Status: "published",
		Dimensions: []semantic.Dimension{
			{Name: "id", ColumnRef: "id", Type: "number"},
			{Name: "customer_region", ColumnRef: "region", Type: "text"},
		},
		Metrics: []semantic.Metric{
			{Name: "customer_count", Expression: "id", Aggregation: "count_distinct"},
		},
	}
	composite := &semantic.CompositeModel{
		ID: "c-1", DatasourceID: "ds-1", Name: "orders_customers", Status: "draft",
		Components: []semantic.ComponentModelRef{
			{Alias: "ord", ModelID: "m-orders", Role: semantic.ComponentRolePrimary},
			{Alias: "cust", ModelID: "m-customers", Role: semantic.ComponentRoleSecondary},
		},
		CrossModelJoins: []semantic.CrossModelJoin{
			{
				Name: "orders_customers", FromModel: "ord", FromDimension: "customer_id",
				ToModel: "cust", ToDimension: "id", JoinType: "LEFT",
				Relationship: "many_to_one", IsActive: true,
			},
		},
	}
	resolved, err := semantic.NewCompositeResolver().Resolve(composite, map[string]*semantic.SemanticModel{
		"ord": orders, "cust": customers,
	})
	if err != nil {
		t.Fatalf("composite resolve failed: %v", err)
	}
	return resolved
}

// TestGolden_PIIMasking_AllTypesPostgres verifies the exact compiled SQL for
// every PII type with "masked" access on Postgres.
func TestGolden_PIIMasking_AllTypesPostgres(t *testing.T) {
	cases := []struct {
		piiType  string
		expected string
	}{
		{pii.TypeEmail, `SELECT (LEFT(CAST("customers"."pii_col" AS TEXT), 2) || '***' || SUBSTRING(CAST("customers"."pii_col" AS TEXT) FROM POSITION('@' IN CAST("customers"."pii_col" AS TEXT)))) AS "pii_field" FROM "public"."customers" LIMIT 10`},
		{pii.TypePhone, `SELECT (LEFT(CAST("customers"."pii_col" AS TEXT), 3) || '****' || RIGHT(CAST("customers"."pii_col" AS TEXT), 2)) AS "pii_field" FROM "public"."customers" LIMIT 10`},
		{pii.TypeIBAN, `SELECT (LEFT(CAST("customers"."pii_col" AS TEXT), 4) || '****' || RIGHT(CAST("customers"."pii_col" AS TEXT), 2)) AS "pii_field" FROM "public"."customers" LIMIT 10`},
		{pii.TypeTCKimlikNo, `SELECT (LEFT(CAST("customers"."pii_col" AS TEXT), 3) || '*****') AS "pii_field" FROM "public"."customers" LIMIT 10`},
		{pii.TypeAddress, `SELECT (LEFT(CAST("customers"."pii_col" AS TEXT), 10) || '...') AS "pii_field" FROM "public"."customers" LIMIT 10`},
		{pii.TypeIPAddress, `SELECT REGEXP_REPLACE(CAST("customers"."pii_col" AS TEXT), '[0-9a-fA-F]+', '*', 'g') AS "pii_field" FROM "public"."customers" LIMIT 10`},
		{pii.TypeCreditCardLike, `SELECT (LEFT(CAST("customers"."pii_col" AS TEXT), 4) || ' **** **** ' || RIGHT(CAST("customers"."pii_col" AS TEXT), 4)) AS "pii_field" FROM "public"."customers" LIMIT 10`},
	}

	model := &semantic.SemanticModel{
		Name:       "customers",
		BaseSchema: "public",
		BaseTable:  "customers",
		Dimensions: []semantic.Dimension{
			{Name: "pii_field", ColumnRef: "customers.pii_col", Type: "text"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.piiType, func(t *testing.T) {
			cfg := &PIIMaskingConfig{
				ColumnAccess: map[string]string{"customers.pii_col": "masked"},
				ColumnTypes:  map[string]string{"customers.pii_col": tc.piiType},
			}
			lq := LogicalQuery{
				ModelID: "customers",
				Select:  []SelectItem{{Type: "dimension", Name: "pii_field"}},
				Limit:   10,
			}
			cq, err := NewCompiler(dialect.PostgresDialect{}).CompileWithPermissions(context.Background(), &lq, model, nil, cfg)
			if err != nil {
				t.Fatalf("compile failed: %v", err)
			}
			if got := normalizeSQL(cq.SQL); got != tc.expected {
				t.Errorf("PII masked SQL mismatch.\nExpected:\n%s\n\nGot:\n%s", tc.expected, got)
			}
		})
	}
}

// TestGolden_PIIMasking_EmailPerDialect verifies the exact email masking SQL
// for every supported dialect.
func TestGolden_PIIMasking_EmailPerDialect(t *testing.T) {
	cases := []struct {
		d        dialect.Dialect
		expected string
	}{
		{dialect.Postgres, `SELECT (LEFT(CAST("customers"."email" AS TEXT), 2) || '***' || SUBSTRING(CAST("customers"."email" AS TEXT) FROM POSITION('@' IN CAST("customers"."email" AS TEXT)))) AS "email" FROM "public"."customers" LIMIT 10`},
		{dialect.MySQL, "SELECT CONCAT(LEFT(CAST(`customers`.`email` AS CHAR), 2), '***', SUBSTRING(CAST(`customers`.`email` AS CHAR), LOCATE('@', CAST(`customers`.`email` AS CHAR)))) AS `email` FROM `public`.`customers` LIMIT 10"},
		{dialect.SQLServer, `SELECT CONCAT(LEFT(CAST([customers].[email] AS NVARCHAR(256)), 2), '***', SUBSTRING(CAST([customers].[email] AS NVARCHAR(256)), CHARINDEX('@', CAST([customers].[email] AS NVARCHAR(256))), LEN(CAST([customers].[email] AS NVARCHAR(256))))) AS [email] FROM [public].[customers] ORDER BY (SELECT NULL) OFFSET 0 ROWS FETCH NEXT 10 ROWS ONLY`},
		{dialect.ClickHouse, "SELECT concat(substring(toString(`customers`.`email`), 1, 2), '***', substring(toString(`customers`.`email`), position(toString(`customers`.`email`), '@'))) AS `email` FROM `public`.`customers` LIMIT 10"},
	}

	model := &semantic.SemanticModel{
		Name:       "customers",
		BaseSchema: "public",
		BaseTable:  "customers",
		Dimensions: []semantic.Dimension{
			{Name: "email", ColumnRef: "customers.email", Type: "text"},
		},
	}
	cfg := &PIIMaskingConfig{
		ColumnAccess: map[string]string{"customers.email": "masked"},
		ColumnTypes:  map[string]string{"customers.email": pii.TypeEmail},
	}

	for _, tc := range cases {
		t.Run(tc.d.Name(), func(t *testing.T) {
			lq := LogicalQuery{
				ModelID: "customers",
				Select:  []SelectItem{{Type: "dimension", Name: "email"}},
				Limit:   10,
			}
			cq, err := NewCompiler(tc.d).CompileWithPermissions(context.Background(), &lq, model, nil, cfg)
			if err != nil {
				t.Fatalf("compile failed: %v", err)
			}
			if got := normalizeSQL(cq.SQL); got != normalizeSQL(tc.expected) {
				t.Errorf("PII masked SQL mismatch (%s).\nExpected:\n%s\n\nGot:\n%s", tc.d.Name(), normalizeSQL(tc.expected), got)
			}
		})
	}
}
