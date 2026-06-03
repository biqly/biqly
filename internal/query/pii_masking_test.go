package query

import (
	"context"
	"strings"
	"testing"

	"github.com/biqly/biqly/internal/dialect"
	"github.com/biqly/biqly/internal/security"
	"github.com/biqly/biqly/internal/security/pii"
	"github.com/biqly/biqly/internal/semantic"
	pkgmetadata "github.com/biqly/biqly/pkg/metadata"
)

func piiTestModel() *semantic.SemanticModel {
	return &semantic.SemanticModel{
		Name:       "customers",
		BaseSchema: "public",
		BaseTable:  "customers",
		Dimensions: []semantic.Dimension{
			{Name: "email", ColumnRef: "customers.email", Type: "text"},
			{Name: "phone", ColumnRef: "customers.phone", Type: "text"},
			{Name: "country", ColumnRef: "customers.country", Type: "text"},
		},
		Metrics: []semantic.Metric{
			{Name: "count", Expression: "customers.id", Aggregation: "count"},
		},
	}
}

func piiTestConfig() *PIIMaskingConfig {
	return &PIIMaskingConfig{
		ColumnAccess: map[string]string{
			"customers.email": pii.AccessMasked,
			"customers.phone": pii.AccessHidden,
		},
		ColumnTypes: map[string]string{
			"customers.email": pii.TypeEmail,
			"customers.phone": pii.TypePhone,
		},
	}
}

func TestCompileWithPermissions_MaskedColumn(t *testing.T) {
	lq := LogicalQuery{
		ModelID: "customers",
		Select:  []SelectItem{{Type: "dimension", Name: "email"}},
		Limit:   10,
	}
	compiler := NewCompiler(dialect.PostgresDialect{})
	cq, err := compiler.CompileWithPermissions(context.Background(), &lq, piiTestModel(), nil, piiTestConfig())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !containsStr(cq.SQL, "LEFT(") || !containsStr(cq.SQL, "'***'") {
		t.Errorf("expected email masking expression in SQL: %s", cq.SQL)
	}
	// Alias must stay stable so result column names don't change.
	if !containsStr(cq.SQL, `AS "email"`) {
		t.Errorf("expected preserved alias in SQL: %s", cq.SQL)
	}
}

func TestCompileWithPermissions_HiddenColumn(t *testing.T) {
	lq := LogicalQuery{
		ModelID: "customers",
		Select:  []SelectItem{{Type: "dimension", Name: "phone"}},
		Limit:   10,
	}
	compiler := NewCompiler(dialect.PostgresDialect{})
	cq, err := compiler.CompileWithPermissions(context.Background(), &lq, piiTestModel(), nil, piiTestConfig())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !containsStr(cq.SQL, `'***' AS "phone"`) {
		t.Errorf("expected hidden literal projection in SQL: %s", cq.SQL)
	}
	if strings.Contains(cq.SQL, `"customers"."phone"`) {
		t.Errorf("hidden column must not be referenced in SQL: %s", cq.SQL)
	}
}

func TestCompileWithPermissions_RawAndUnlistedColumnsUntouched(t *testing.T) {
	lq := LogicalQuery{
		ModelID: "customers",
		Select:  []SelectItem{{Type: "dimension", Name: "country"}},
		Limit:   10,
	}
	compiler := NewCompiler(dialect.PostgresDialect{})
	cq, err := compiler.CompileWithPermissions(context.Background(), &lq, piiTestModel(), nil, piiTestConfig())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if containsStr(cq.SQL, "'***'") {
		t.Errorf("unlisted column must not be masked: %s", cq.SQL)
	}
}

func TestCompileWithPermissions_NilConfigBackwardCompatible(t *testing.T) {
	lq := LogicalQuery{
		ModelID: "customers",
		Select:  []SelectItem{{Type: "dimension", Name: "email"}},
		Limit:   10,
	}
	compiler := NewCompiler(dialect.PostgresDialect{})
	cq, err := compiler.CompileWithPermissions(context.Background(), &lq, piiTestModel(), nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if containsStr(cq.SQL, "'***'") {
		t.Errorf("nil config must not mask: %s", cq.SQL)
	}
}

func TestCompileWithPermissions_FilterOnHiddenColumnRejected(t *testing.T) {
	lq := LogicalQuery{
		ModelID: "customers",
		Select:  []SelectItem{{Type: "dimension", Name: "country"}},
		Filters: []Filter{{Field: "phone", Operator: OpEq, Value: "05551234567"}},
		Limit:   10,
	}
	compiler := NewCompiler(dialect.PostgresDialect{})
	_, err := compiler.CompileWithPermissions(context.Background(), &lq, piiTestModel(), nil, piiTestConfig())
	if err == nil {
		t.Fatal("expected error for filter on hidden PII column")
	}
	if !strings.Contains(err.Error(), "hidden") {
		t.Errorf("expected hidden PII error, got: %v", err)
	}
}

func TestCompileWithPermissions_FilterOnMaskedColumnAllowed(t *testing.T) {
	lq := LogicalQuery{
		ModelID: "customers",
		Select:  []SelectItem{{Type: "dimension", Name: "country"}},
		Filters: []Filter{{Field: "email", Operator: OpEq, Value: "a@example.com"}},
		Limit:   10,
	}
	compiler := NewCompiler(dialect.PostgresDialect{})
	if _, err := compiler.CompileWithPermissions(context.Background(), &lq, piiTestModel(), nil, piiTestConfig()); err != nil {
		t.Fatalf("masked column filters must be allowed: %v", err)
	}
}

func TestCompileWithPermissions_FilterOnFullStrategyColumnRejected(t *testing.T) {
	config := piiTestConfig()
	config.ColumnStrategies = map[string]string{"customers.email": pii.MaskingStrategyFull}
	lq := LogicalQuery{
		ModelID: "customers",
		Select:  []SelectItem{{Type: "dimension", Name: "country"}},
		Filters: []Filter{{Field: "email", Operator: OpEq, Value: "target@example.com"}},
		Limit:   10,
	}

	compiler := NewCompiler(dialect.PostgresDialect{})
	_, err := compiler.CompileWithPermissions(context.Background(), &lq, piiTestModel(), nil, config)
	if err == nil {
		t.Fatal("expected error for filter on full-masked PII column")
	}
	if !strings.Contains(err.Error(), "hidden") {
		t.Errorf("expected hidden PII error, got: %v", err)
	}
}

func TestCompileWithPermissions_GroupByAndOrderByUseMaskedExpr(t *testing.T) {
	lq := LogicalQuery{
		ModelID: "customers",
		Select: []SelectItem{
			{Type: "dimension", Name: "email"},
			{Type: "metric", Name: "count"},
		},
		GroupBy: []GroupBy{{Field: "email"}},
		OrderBy: []OrderBy{{Field: "email", Direction: "asc"}},
		Limit:   10,
	}
	compiler := NewCompiler(dialect.PostgresDialect{})
	cq, err := compiler.CompileWithPermissions(context.Background(), &lq, piiTestModel(), nil, piiTestConfig())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// The masked expression must appear in GROUP BY and ORDER BY, never the
	// bare column.
	if got := strings.Count(cq.SQL, "LEFT(CAST("); got < 3 {
		t.Errorf("expected masked expr in SELECT, GROUP BY and ORDER BY (3+ occurrences), got %d: %s", got, cq.SQL)
	}
}

func TestCompileWithPermissions_GroupByFullStrategyColumnRejected(t *testing.T) {
	config := piiTestConfig()
	config.ColumnStrategies = map[string]string{"customers.email": pii.MaskingStrategyFull}
	lq := LogicalQuery{
		ModelID: "customers",
		Select: []SelectItem{
			{Type: "dimension", Name: "email"},
			{Type: "metric", Name: "count"},
		},
		GroupBy: []GroupBy{{Field: "email"}},
		Limit:   10,
	}

	compiler := NewCompiler(dialect.PostgresDialect{})
	_, err := compiler.CompileWithPermissions(context.Background(), &lq, piiTestModel(), nil, config)
	if err == nil {
		t.Fatal("expected error for grouping by full-masked PII column")
	}
	if !strings.Contains(err.Error(), "hidden") {
		t.Errorf("expected hidden PII error, got: %v", err)
	}
}

func TestCompileWithPermissions_OrderByFullStrategyColumnRejected(t *testing.T) {
	config := piiTestConfig()
	config.ColumnStrategies = map[string]string{"customers.email": pii.MaskingStrategyFull}
	lq := LogicalQuery{
		ModelID: "customers",
		Select:  []SelectItem{{Type: "dimension", Name: "country"}},
		OrderBy: []OrderBy{{Field: "email", Direction: "asc"}},
		Limit:   10,
	}

	compiler := NewCompiler(dialect.PostgresDialect{})
	_, err := compiler.CompileWithPermissions(context.Background(), &lq, piiTestModel(), nil, config)
	if err == nil {
		t.Fatal("expected error for sorting by full-masked PII column")
	}
	if !strings.Contains(err.Error(), "hidden") {
		t.Errorf("expected hidden PII error, got: %v", err)
	}
}

func TestCompileWithPermissions_UnknownMaskingStrategyFailsClosed(t *testing.T) {
	config := piiTestConfig()
	config.ColumnStrategies = map[string]string{"customers.email": "surprise"}
	lq := LogicalQuery{
		ModelID: "customers",
		Select:  []SelectItem{{Type: "dimension", Name: "email"}},
		Limit:   10,
	}

	compiler := NewCompiler(dialect.PostgresDialect{})
	cq, err := compiler.CompileWithPermissions(context.Background(), &lq, piiTestModel(), nil, config)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !containsStr(cq.SQL, `'***' AS "email"`) {
		t.Errorf("unknown strategy must fail closed to hidden literal: %s", cq.SQL)
	}
	if strings.Contains(cq.SQL, "LEFT(") {
		t.Errorf("unknown strategy must not fall back to partial masking: %s", cq.SQL)
	}
}

func TestCompileWithPermissions_MaskingWithRowFilters(t *testing.T) {
	lq := LogicalQuery{
		ModelID: "customers",
		Select:  []SelectItem{{Type: "dimension", Name: "email"}},
		Limit:   10,
	}
	rowFilters := []security.RowFilter{
		{Field: "country", Operator: "eq", Value: "TR"},
	}
	compiler := NewCompiler(dialect.PostgresDialect{})
	cq, err := compiler.CompileWithPermissions(context.Background(), &lq, piiTestModel(), rowFilters, piiTestConfig())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !containsStr(cq.SQL, "WHERE") || !containsStr(cq.SQL, "country") {
		t.Errorf("expected row filter in SQL: %s", cq.SQL)
	}
	if !containsStr(cq.SQL, "LEFT(") {
		t.Errorf("expected masking alongside row filters: %s", cq.SQL)
	}
}

// TestCompileWithPermissions_RoleBasedAccess exercises the full policy path:
// stored PII annotations -> role defaults -> masking config -> compiled SQL.
// Admin sees raw values, analyst masked, viewer hidden (sensitive types).
func TestCompileWithPermissions_RoleBasedAccess(t *testing.T) {
	emailType := pii.TypeEmail
	tcknType := pii.TypeTCKimlikNo
	piiCols := []pkgmetadata.Column{
		{SchemaName: "public", TableName: "customers", ColumnName: "email", PIIType: &emailType},
		{SchemaName: "public", TableName: "customers", ColumnName: "tckn", PIIType: &tcknType},
	}
	model := piiTestModel()
	model.Dimensions = append(model.Dimensions, semantic.Dimension{
		Name: "tckn", ColumnRef: "customers.tckn", Type: "text",
	})
	lq := LogicalQuery{
		ModelID: "customers",
		Select: []SelectItem{
			{Type: "dimension", Name: "email"},
			{Type: "dimension", Name: "tckn"},
		},
		Limit: 10,
	}

	compile := func(role string) string {
		access, types := pii.BuildColumnAccessMaps(role, piiCols, nil)
		cfg := &PIIMaskingConfig{ColumnAccess: access, ColumnTypes: types}
		lqCopy := lq
		cq, err := NewCompiler(dialect.PostgresDialect{}).CompileWithPermissions(context.Background(), &lqCopy, model, nil, cfg)
		if err != nil {
			t.Fatalf("compile as %s: %v", role, err)
		}
		return cq.SQL
	}

	adminSQL := compile(pii.RoleAdmin)
	if strings.Contains(adminSQL, "'***'") || strings.Contains(adminSQL, "LEFT(") {
		t.Errorf("admin must see raw values: %s", adminSQL)
	}

	analystSQL := compile(pii.RoleAnalyst)
	if !strings.Contains(analystSQL, "LEFT(") {
		t.Errorf("analyst must see masked values: %s", analystSQL)
	}
	if strings.Contains(analystSQL, `"customers"."email" AS "email"`) {
		t.Errorf("analyst must not project raw email: %s", analystSQL)
	}

	viewerSQL := compile(pii.RoleViewer)
	if !strings.Contains(viewerSQL, `'***' AS "tckn"`) {
		t.Errorf("viewer must see tckn hidden: %s", viewerSQL)
	}
	if strings.Contains(viewerSQL, `"customers"."tckn"`) {
		t.Errorf("viewer SQL must not reference tckn column at all: %s", viewerSQL)
	}
	if !strings.Contains(viewerSQL, "LEFT(") {
		t.Errorf("viewer must see email masked: %s", viewerSQL)
	}
}

func TestCompileWithPermissions_MaskingAcrossDialects(t *testing.T) {
	dialects := []dialect.Dialect{dialect.Postgres, dialect.MySQL, dialect.SQLServer, dialect.ClickHouse}
	for _, d := range dialects {
		t.Run(d.Name(), func(t *testing.T) {
			lq := LogicalQuery{
				ModelID: "customers",
				Select:  []SelectItem{{Type: "dimension", Name: "email"}},
				Limit:   10,
			}
			compiler := NewCompiler(d)
			cq, err := compiler.CompileWithPermissions(context.Background(), &lq, piiTestModel(), nil, piiTestConfig())
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !containsStr(cq.SQL, "'***'") {
				t.Errorf("expected mask literal in %s SQL: %s", d.Name(), cq.SQL)
			}
		})
	}
}

func TestCompileWithPermissions_MaskingStrategyOverride(t *testing.T) {
	config := &PIIMaskingConfig{
		ColumnAccess: map[string]string{
			"customers.email": pii.AccessMasked,
			"customers.phone": pii.AccessMasked,
		},
		ColumnTypes: map[string]string{
			"customers.email": pii.TypeEmail,
			"customers.phone": pii.TypePhone,
		},
		ColumnStrategies: map[string]string{
			"customers.phone": pii.MaskingStrategyFull,
			"customers.email": pii.MaskingStrategyPartial,
		},
	}

	lq := LogicalQuery{
		ModelID: "customers",
		Select: []SelectItem{
			{Type: "dimension", Name: "email"},
			{Type: "dimension", Name: "phone"},
		},
		Limit: 10,
	}

	compiler := NewCompiler(dialect.PostgresDialect{})
	cq, err := compiler.CompileWithPermissions(context.Background(), &lq, piiTestModel(), nil, config)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Email has "partial" strategy -> should use LEFT(CAST(...
	if !containsStr(cq.SQL, "LEFT(") {
		t.Errorf("expected partial masking for email: %s", cq.SQL)
	}

	// Phone has "full" strategy -> should be hidden literal '***'
	if !containsStr(cq.SQL, `'***' AS "phone"`) {
		t.Errorf("expected full masking (hidden literal) for phone: %s", cq.SQL)
	}
	if strings.Contains(cq.SQL, `"customers"."phone"`) {
		t.Errorf("full masking column must not be referenced in SQL: %s", cq.SQL)
	}
}
