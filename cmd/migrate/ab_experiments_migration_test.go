package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestABExperimentsMigrationFiles(t *testing.T) {
	up := readMigrationForTest(t, "migrations/042a_add_ab_experiments.up.sql")
	down := readMigrationForTest(t, "migrations/042b_add_ab_experiments.down.sql")

	upMustContain := []string{
		"CREATE TABLE IF NOT EXISTS ab_experiments",
		"id TEXT PRIMARY KEY DEFAULT gen_random_uuid()::text",
		"status TEXT NOT NULL DEFAULT 'draft'",
		"CHECK (status IN ('draft', 'running', 'paused', 'completed'))",
		"CREATE INDEX IF NOT EXISTS idx_ab_exp_status",
		"CREATE TABLE IF NOT EXISTS ab_variants",
		"experiment_id TEXT NOT NULL REFERENCES ab_experiments(id) ON DELETE CASCADE",
		"traffic_pct INT NOT NULL DEFAULT 50 CHECK (traffic_pct >= 0 AND traffic_pct <= 100)",
		"UNIQUE (experiment_id, name)",
		"ADD COLUMN IF NOT EXISTS ab_experiment_id TEXT",
		"ADD COLUMN IF NOT EXISTS ab_variant_id TEXT",
	}
	for _, want := range upMustContain {
		if !strings.Contains(up, want) {
			t.Fatalf("up migration missing %q", want)
		}
	}

	downMustContain := []string{
		"DROP TABLE IF EXISTS ab_variants",
		"DROP TABLE IF EXISTS ab_experiments",
		"DROP COLUMN IF EXISTS ab_variant_id",
		"DROP COLUMN IF EXISTS ab_experiment_id",
	}
	for _, want := range downMustContain {
		if !strings.Contains(down, want) {
			t.Fatalf("down migration missing %q", want)
		}
	}
}

func readMigrationForTest(t *testing.T, path string) string {
	t.Helper()

	b, err := os.ReadFile(filepath.Clean(filepath.Join("..", "..", path)))
	if err != nil {
		t.Fatalf("read migration %s: %v", path, err)
	}
	return string(b)
}
