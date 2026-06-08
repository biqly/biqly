package drift

import (
	"context"
	"testing"

	"github.com/biqly/biqly/internal/metadata"
	"github.com/biqly/biqly/internal/semantic"
)

func TestDetectorCompare_CleanSchema(t *testing.T) {
	detector := NewDetector()
	ctx := context.Background()

	model := semantic.SemanticModel{
		ID:           "model-1",
		DatasourceID: "ds-1",
		BaseSchema:   "public",
		BaseTable:    "users",
		Dimensions: []semantic.Dimension{
			{Name: "id", ColumnRef: "public.users.id", Type: "number", IsActive: true},
			{Name: "name", ColumnRef: "public.users.name", Type: "text", IsActive: true},
		},
		Metrics: []semantic.Metric{
			{Name: "total_users", Expression: "count(public.users.id)", IsActive: true},
		},
	}

	tables := []metadata.Table{
		{SchemaName: "public", TableName: "users"},
	}

	columns := []metadata.Column{
		{SchemaName: "public", TableName: "users", ColumnName: "id", DataType: "integer"},
		{SchemaName: "public", TableName: "users", ColumnName: "name", DataType: "varchar"},
	}

	report, err := detector.Compare(ctx, model, columns, tables)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if report == nil {
		t.Fatal("Compare() report = nil, want explicit no-drift report")
	}
	if len(report.Drifts) != 0 {
		t.Fatalf("Compare() drifts = %+v, want none", report.Drifts)
	}
}

func TestDetectorCompare_DroppedResources(t *testing.T) {
	detector := NewDetector()
	ctx := context.Background()

	for _, tc := range []struct {
		name     string
		tables   []metadata.Table
		columns  []metadata.Column
		wantType DriftType
	}{
		{
			name: "Schema Dropped",
			tables: []metadata.Table{
				{SchemaName: "other", TableName: "users"},
			},
			columns: []metadata.Column{
				{SchemaName: "other", TableName: "users", ColumnName: "id", DataType: "integer"},
			},
			wantType: DriftTypeSchemaDropped,
		},
		{
			name: "Table Dropped",
			tables: []metadata.Table{
				{SchemaName: "public", TableName: "orders"},
			},
			columns: []metadata.Column{
				{SchemaName: "public", TableName: "orders", ColumnName: "id", DataType: "integer"},
			},
			wantType: DriftTypeTableDropped,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			model := semantic.SemanticModel{
				ID:           "model-1",
				DatasourceID: "ds-1",
				BaseSchema:   "public",
				BaseTable:    "users",
			}
			report, err := detector.Compare(ctx, model, tc.columns, tc.tables)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if report == nil {
				t.Fatal("expected report, got nil")
			}
			if len(report.Drifts) != 1 || report.Drifts[0].Type != tc.wantType {
				t.Fatalf("expected %s drift, got: %+v", tc.wantType, report.Drifts)
			}
			if report.Severity != SeverityCritical {
				t.Fatalf("expected Critical severity, got: %s", report.Severity)
			}
		})
	}
}

func TestDetectorCompare_ColumnDropped(t *testing.T) {
	detector := NewDetector()
	ctx := context.Background()

	model := semantic.SemanticModel{
		ID:           "model-1",
		DatasourceID: "ds-1",
		BaseSchema:   "public",
		BaseTable:    "users",
		Dimensions: []semantic.Dimension{
			{Name: "age", ColumnRef: "public.users.age", Type: "number", IsActive: true},
		},
	}

	tables := []metadata.Table{
		{SchemaName: "public", TableName: "users"},
	}

	columns := []metadata.Column{
		{SchemaName: "public", TableName: "users", ColumnName: "id", DataType: "integer"},
	}

	report, err := detector.Compare(ctx, model, columns, tables)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if report == nil {
		t.Fatal("expected report, got nil")
	}
	hasColumnDropped := false
	for _, drift := range report.Drifts {
		if drift.Type == DriftTypeColumnDropped {
			hasColumnDropped = true
		}
	}
	if !hasColumnDropped {
		t.Fatalf("expected ColumnDropped drift, got: %+v", report.Drifts)
	}
}

func TestDetectorCompare_TypeChanged(t *testing.T) {
	detector := NewDetector()
	ctx := context.Background()

	model := semantic.SemanticModel{
		ID:           "model-1",
		DatasourceID: "ds-1",
		BaseSchema:   "public",
		BaseTable:    "users",
		Dimensions: []semantic.Dimension{
			{Name: "id", ColumnRef: "public.users.id", Type: "number", IsActive: true},
		},
	}

	tables := []metadata.Table{
		{SchemaName: "public", TableName: "users"},
	}

	columns := []metadata.Column{
		{SchemaName: "public", TableName: "users", ColumnName: "id", DataType: "varchar"},
	}

	report, err := detector.Compare(ctx, model, columns, tables)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if report == nil {
		t.Fatal("expected report, got nil")
	}
	if len(report.Drifts) != 1 || report.Drifts[0].Type != DriftTypeTypeChanged {
		t.Fatalf("expected TypeChanged drift, got: %+v", report.Drifts)
	}
	if report.Severity != SeverityWarning {
		t.Fatalf("expected Warning severity, got: %s", report.Severity)
	}
}

func TestDetectorCompare_BrokenJoin(t *testing.T) {
	detector := NewDetector()
	ctx := context.Background()

	model := semantic.SemanticModel{
		ID:           "model-1",
		DatasourceID: "ds-1",
		BaseSchema:   "public",
		BaseTable:    "users",
		Joins: []semantic.Join{
			{
				Name:       "users_to_orders",
				FromTable:  "users",
				FromColumn: "id",
				ToTable:    "orders",
				ToColumn:   "user_id",
				IsActive:   true,
			},
		},
	}

	tables := []metadata.Table{
		{SchemaName: "public", TableName: "users"},
	}

	columns := []metadata.Column{
		{SchemaName: "public", TableName: "users", ColumnName: "id", DataType: "integer"},
	}

	report, err := detector.Compare(ctx, model, columns, tables)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if report == nil {
		t.Fatal("expected report, got nil")
	}
	hasJoinBroken := false
	for _, drift := range report.Drifts {
		if drift.Type == DriftTypeJoinBroken {
			hasJoinBroken = true
		}
	}
	if !hasJoinBroken {
		t.Fatalf("expected JoinBroken drift, got: %+v", report.Drifts)
	}
	if report.Severity != SeverityWarning {
		t.Fatalf("expected Warning severity, got: %s", report.Severity)
	}
}
