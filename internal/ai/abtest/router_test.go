package abtest

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestTrafficRouterResolveVariantFallback(t *testing.T) {
	ctx := context.Background()
	runner := &fakeDBRunner{
		queryContext: func(context.Context, string, ...any) (rowsScanner, error) {
			return newFakeRows(nil), nil // no running experiments
		},
	}
	repo := newRepositoryWithRunner(runner)
	router := NewTrafficRouter(repo)

	variant, err := router.ResolveVariant(ctx, "user-1", "system_rules", "en", 5)
	if err != nil {
		t.Fatalf("ResolveVariant error = %v, want nil", err)
	}
	if variant.TemplateVersion != 5 {
		t.Errorf("ResolveVariant returned version = %d, want default version 5", variant.TemplateVersion)
	}
	if variant.ExperimentID != "" {
		t.Errorf("ResolveVariant returned non-empty experiment ID = %q, want empty", variant.ExperimentID)
	}
}

func TestTrafficRouterResolveVariantSelection(t *testing.T) {
	ctx := context.Background()
	now := time.Now()
	runner := &fakeDBRunner{
		queryContext: func(_ context.Context, query string, _ ...any) (rowsScanner, error) {
			if strings.Contains(query, "FROM ab_experiments") {
				return newFakeRows([][]any{
					{
						"exp-1", "System Rules Test", "desc", "system_rules", "en", string(ExperimentStatusRunning),
						sql.NullTime{Time: now, Valid: true}, sql.NullTime{}, sql.NullString{}, now, now,
					},
				}), nil
			}
			if strings.Contains(query, "FROM ab_variants") {
				return newFakeRows([][]any{
					{"var-1", "exp-1", "control", 2, 50, true},
					{"var-2", "exp-1", "treatment", 3, 50, false},
				}), nil
			}
			return nil, errors.New("unexpected query")
		},
	}
	repo := newRepositoryWithRunner(runner)
	router := NewTrafficRouter(repo)

	// Since resolving variants is deterministic based on hashing user_id + experiment_id:
	// We can test that different users hit different buckets and consistently get those assignments.
	variant1, err := router.ResolveVariant(ctx, "user-a", "system_rules", "en", 1)
	if err != nil {
		t.Fatalf("ResolveVariant user-a: %v", err)
	}

	variant2, err := router.ResolveVariant(ctx, "user-b", "system_rules", "en", 1)
	if err != nil {
		t.Fatalf("ResolveVariant user-b: %v", err)
	}

	if variant1.ExperimentID != "exp-1" || variant2.ExperimentID != "exp-1" {
		t.Errorf("expected experiment ID exp-1, got %q and %q", variant1.ExperimentID, variant2.ExperimentID)
	}

	// Verify assignments remain stable for the same user
	variant1Again, err := router.ResolveVariant(ctx, "user-a", "system_rules", "en", 1)
	if err != nil {
		t.Fatalf("ResolveVariant user-a again: %v", err)
	}
	if variant1.ID != variant1Again.ID {
		t.Errorf("ResolveVariant assigned user-a to %q then %q, want deterministic result", variant1.ID, variant1Again.ID)
	}
}

func TestTrafficRouterCacheTTL(t *testing.T) {
	ctx := context.Background()
	now := time.Now()
	queryCount := 0
	runner := &fakeDBRunner{
		queryContext: func(_ context.Context, query string, _ ...any) (rowsScanner, error) {
			if strings.Contains(query, "FROM ab_experiments") {
				queryCount++
				return newFakeRows([][]any{
					{
						"exp-1", "System Rules Test", "desc", "system_rules", "en", string(ExperimentStatusRunning),
						sql.NullTime{Time: now, Valid: true}, sql.NullTime{}, sql.NullString{}, now, now,
					},
				}), nil
			}
			if strings.Contains(query, "FROM ab_variants") {
				return newFakeRows([][]any{
					{"var-1", "exp-1", "control", 2, 100, true},
				}), nil
			}
			return nil, errors.New("unexpected query")
		},
	}
	repo := newRepositoryWithRunner(runner)
	router := NewTrafficRouter(repo)

	// first lookup - triggers DB query
	_, err := router.ResolveVariant(ctx, "user-1", "system_rules", "en", 1)
	if err != nil {
		t.Fatal(err)
	}
	if queryCount != 1 {
		t.Fatalf("expected 1 query, got %d", queryCount)
	}

	// second lookup - should hit cache, not DB
	_, err = router.ResolveVariant(ctx, "user-1", "system_rules", "en", 1)
	if err != nil {
		t.Fatal(err)
	}
	if queryCount != 1 {
		t.Fatalf("expected cached query, but query count is %d", queryCount)
	}

	// manual invalidate
	router.Invalidate("system_rules", "en")

	// third lookup - should hit DB again
	_, err = router.ResolveVariant(ctx, "user-1", "system_rules", "en", 1)
	if err != nil {
		t.Fatal(err)
	}
	if queryCount != 2 {
		t.Fatalf("expected cache invalidation to trigger DB query, got %d queries", queryCount)
	}
}

func TestExperimentTrackerAddVariantAndGetVariants(t *testing.T) {
	tracker := &ExperimentTracker{}
	variant1 := Variant{ID: "var-1", ExperimentID: "exp-1", Name: "control", TemplateVersion: 2, TrafficPct: 50, IsControl: true}
	variant2 := Variant{ID: "var-2", ExperimentID: "exp-1", Name: "treatment", TemplateVersion: 3, TrafficPct: 50}

	tracker.AddVariant(variant1)
	tracker.AddVariant(variant2)

	got := tracker.GetVariants()
	if len(got) != 2 {
		t.Fatalf("GetVariants() len = %d, want 2", len(got))
	}
	if got[0].ID != "var-1" || got[1].ID != "var-2" {
		t.Errorf("GetVariants() = %v, want [var-1 var-2]", got)
	}
}

func TestExperimentTrackerGetVariantsEmpty(t *testing.T) {
	tracker := &ExperimentTracker{}
	got := tracker.GetVariants()
	if got != nil {
		t.Fatalf("GetVariants() for empty tracker = %v, want nil", got)
	}
}

func TestExperimentTrackerGetVariantsIsCopy(t *testing.T) {
	tracker := &ExperimentTracker{}
	tracker.AddVariant(Variant{ID: "var-1", ExperimentID: "exp-1"})

	got := tracker.GetVariants()
	got[0].ID = "mutated"
	// Internal state should be unchanged
	original := tracker.GetVariants()
	if original[0].ID != "var-1" {
		t.Errorf("GetVariants returned non-copy, internal state mutated: %q", original[0].ID)
	}
}

func TestWithExperimentTrackerAndTrackerFromContext(t *testing.T) {
	ctx := context.Background()
	tracker := &ExperimentTracker{}
	variant := Variant{ID: "var-1", ExperimentID: "exp-1"}

	ctx = WithExperimentTracker(ctx, tracker)
	tracker.AddVariant(variant)

	got := TrackerFromContext(ctx)
	if got == nil {
		t.Fatal("TrackerFromContext returned nil")
	}
	variants := got.GetVariants()
	if len(variants) != 1 || variants[0].ID != "var-1" {
		t.Fatalf("TrackerFromContext variants = %v, want [var-1]", variants)
	}
}

func TestTrackerFromContextNoTracker(t *testing.T) {
	ctx := context.Background()
	got := TrackerFromContext(ctx)
	if got != nil {
		t.Fatal("TrackerFromContext on context without tracker should return nil")
	}
}
