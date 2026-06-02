package semantic

import (
	"context"
	"database/sql"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	_ "github.com/jackc/pgx/v5/stdlib"
)

// openCompositeTestDB connects to the metadata DB and skips when unavailable or
// when the composite tables (migration 037a) have not been applied.
func openCompositeTestDB(t *testing.T) *sql.DB {
	t.Helper()
	dsn := os.Getenv("BI_METADATA_DB_DSN")
	if dsn == "" {
		//nolint:gosec // local test default DSN only
		dsn = "postgres://bi_user:bi_password@localhost:5432/bi_metadata?sslmode=disable"
	}
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Skip("skipping composite integration; DB not available:", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		t.Skip("skipping composite integration; ping failed:", err)
	}
	var exists bool
	if err := db.QueryRowContext(ctx, `SELECT to_regclass('public.composite_models') IS NOT NULL`).Scan(&exists); err != nil || !exists {
		t.Skip("skipping composite integration; composite tables not migrated")
	}
	return db
}

// seedComponentModels inserts a datasource and two minimal semantic_models rows
// so composite component FKs are satisfied. It returns datasourceID and the two
// component model IDs, and registers cleanup.
func seedComponentModels(t *testing.T, db *sql.DB) (datasourceID, ordersID, customersID string) {
	t.Helper()
	ctx := context.Background()
	datasourceID = uuid.NewString()
	ordersID = uuid.NewString()
	customersID = uuid.NewString()

	_, err := db.ExecContext(ctx,
		`INSERT INTO datasources (id, name, type, dsn_encrypted) VALUES ($1, $2, 'postgres', 'enc')`,
		datasourceID, "composite-it-"+datasourceID[:8])
	if err != nil {
		t.Fatalf("seed datasource: %v", err)
	}
	for _, m := range []struct{ id, name, table string }{
		{ordersID, "orders", "orders"},
		{customersID, "customers", "customers"},
	} {
		_, err := db.ExecContext(ctx,
			`INSERT INTO semantic_models (id, datasource_id, name, base_schema, base_table) VALUES ($1, $2, $3, 'public', $4)`,
			m.id, datasourceID, m.name, m.table)
		if err != nil {
			t.Fatalf("seed semantic model %s: %v", m.name, err)
		}
	}
	t.Cleanup(func() {
		// Cascades clean up composite + semantic rows.
		_, _ = db.ExecContext(context.Background(), `DELETE FROM datasources WHERE id = $1`, datasourceID)
	})
	return datasourceID, ordersID, customersID
}

// publishedComponentProvider serves in-memory published component models so the
// publish flow does not need a fully-seeded semantic schema.
func publishedComponentProvider(ordersID, customersID string) stubComponentProvider {
	return stubComponentProvider{models: map[string]*SemanticModel{
		ordersID: {
			ID: ordersID, Name: "orders", BaseSchema: "public", BaseTable: "orders",
			Status: ModelStatusPublished,
			Dimensions: []Dimension{
				{Name: "customer_id", ColumnRef: "customer_id", Type: "number"},
				{Name: "order_date", ColumnRef: "created_at", Type: "date"},
			},
			Metrics: []Metric{{Name: "total_revenue", Expression: "total_amount", Aggregation: "sum"}},
		},
		customersID: {
			ID: customersID, Name: "customers", BaseSchema: "public", BaseTable: "customers",
			Status: ModelStatusPublished,
			Dimensions: []Dimension{
				{Name: "id", ColumnRef: "id", Type: "number"},
				{Name: "customer_region", ColumnRef: "region", Type: "text"},
			},
			Metrics: []Metric{{Name: "customer_count", Expression: "id", Aggregation: "count_distinct"}},
		},
	}}
}

func TestIntegration_CompositeCRUD(t *testing.T) {
	db := openCompositeTestDB(t)
	ctx := context.Background()
	datasourceID, ordersID, customersID := seedComponentModels(t, db)
	repo := NewCompositeRepository(db)

	composite := &CompositeModel{
		DatasourceID: datasourceID,
		Name:         "orders_customers_crud",
		Status:       ModelStatusDraft,
	}
	if err := repo.CreateComposite(ctx, composite); err != nil {
		t.Fatalf("create composite: %v", err)
	}
	if composite.ID == "" {
		t.Fatal("expected composite ID to be set")
	}

	if err := repo.AddComponent(ctx, composite.ID, ComponentModelRef{ModelID: ordersID, Alias: "ord", Role: ComponentRolePrimary}); err != nil {
		t.Fatalf("add primary component: %v", err)
	}
	if err := repo.AddComponent(ctx, composite.ID, ComponentModelRef{ModelID: customersID, Alias: "cust", Role: ComponentRoleSecondary}); err != nil {
		t.Fatalf("add secondary component: %v", err)
	}
	if err := repo.AddCrossModelJoin(ctx, composite.ID, CrossModelJoin{
		Name: "ord_cust", FromModel: "ord", FromDimension: "customer_id",
		ToModel: "cust", ToDimension: "id", JoinType: "LEFT",
		Relationship: RelationshipManyToOne, IsActive: true,
	}); err != nil {
		t.Fatalf("add cross join: %v", err)
	}
	if err := repo.SetCanonicalDate(ctx, composite.ID, &CanonicalDateRef{ModelAlias: "ord", DimensionName: "order_date"}); err != nil {
		t.Fatalf("set canonical date: %v", err)
	}

	full, err := repo.GetFullComposite(ctx, composite.ID)
	if err != nil {
		t.Fatalf("get full composite: %v", err)
	}
	if len(full.Components) != 2 {
		t.Fatalf("expected 2 components, got %d", len(full.Components))
	}
	if len(full.CrossModelJoins) != 1 {
		t.Fatalf("expected 1 cross join, got %d", len(full.CrossModelJoins))
	}
	if full.CanonicalDate == nil || full.CanonicalDate.DimensionName != "order_date" {
		t.Fatalf("expected canonical date order_date, got %+v", full.CanonicalDate)
	}

	if err := repo.DeleteComposite(ctx, composite.ID); err != nil {
		t.Fatalf("delete composite: %v", err)
	}
	if _, err := repo.GetComposite(ctx, composite.ID); err == nil {
		t.Fatal("expected error fetching deleted composite")
	}
}

func TestIntegration_CompositePublishRollback(t *testing.T) {
	db := openCompositeTestDB(t)
	ctx := context.Background()
	datasourceID, ordersID, customersID := seedComponentModels(t, db)
	repo := NewCompositeRepository(db)
	provider := publishedComponentProvider(ordersID, customersID)

	composite := &CompositeModel{
		DatasourceID: datasourceID,
		Name:         "orders_customers_publish",
		Status:       ModelStatusDraft,
	}
	if err := repo.CreateComposite(ctx, composite); err != nil {
		t.Fatalf("create composite: %v", err)
	}
	mustAdd := func(ref ComponentModelRef) {
		if err := repo.AddComponent(ctx, composite.ID, ref); err != nil {
			t.Fatalf("add component %s: %v", ref.Alias, err)
		}
	}
	mustAdd(ComponentModelRef{ModelID: ordersID, Alias: "ord", Role: ComponentRolePrimary})
	mustAdd(ComponentModelRef{ModelID: customersID, Alias: "cust", Role: ComponentRoleSecondary})
	if err := repo.AddCrossModelJoin(ctx, composite.ID, CrossModelJoin{
		Name: "ord_cust", FromModel: "ord", FromDimension: "customer_id",
		ToModel: "cust", ToDimension: "id", JoinType: "LEFT",
		Relationship: RelationshipManyToOne, IsActive: true,
	}); err != nil {
		t.Fatalf("add cross join: %v", err)
	}
	if err := repo.SetCanonicalDate(ctx, composite.ID, &CanonicalDateRef{ModelAlias: "ord", DimensionName: "order_date"}); err != nil {
		t.Fatalf("set canonical date: %v", err)
	}

	// First publish -> version 1.
	res1, err := repo.PublishComposite(ctx, composite.ID, "tester", provider)
	if err != nil {
		t.Fatalf("publish v1: %v", err)
	}
	if !res1.Validation.Valid {
		t.Fatalf("expected valid publish, errors: %v", res1.Validation.Errors)
	}
	if res1.Version != 1 {
		t.Fatalf("expected version 1, got %d", res1.Version)
	}
	if res1.Resolved == nil || len(res1.Resolved.Dimensions) == 0 {
		t.Fatal("expected resolved model with dimensions")
	}

	// Published resolved fetch works.
	resolved, err := repo.GetPublishedResolvedComposite(ctx, composite.ID)
	if err != nil {
		t.Fatalf("get published resolved: %v", err)
	}
	if resolved == nil {
		t.Fatal("expected published resolved composite")
	}

	// Second publish -> version 2.
	res2, err := repo.PublishComposite(ctx, composite.ID, "tester", provider)
	if err != nil {
		t.Fatalf("publish v2: %v", err)
	}
	if res2.Version != 2 {
		t.Fatalf("expected version 2, got %d", res2.Version)
	}

	// Rollback to version 1 -> new version 3.
	resRb, err := repo.RollbackComposite(ctx, composite.ID, 1, "tester")
	if err != nil {
		t.Fatalf("rollback: %v", err)
	}
	if resRb.Version != 3 {
		t.Fatalf("expected rollback to produce version 3, got %d", resRb.Version)
	}

	current, err := repo.GetComposite(ctx, composite.ID)
	if err != nil {
		t.Fatalf("get composite after rollback: %v", err)
	}
	if current.Version != 3 {
		t.Fatalf("expected current version 3, got %d", current.Version)
	}
}
