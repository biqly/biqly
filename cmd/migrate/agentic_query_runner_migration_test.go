package main

import (
	"strings"
	"testing"
)

func TestAgenticQueryRunnerMigrationFiles(t *testing.T) {
	up := readMigrationForTest(t, "migrations/065a_agentic_query_runner.up.sql")
	down := readMigrationForTest(t, "migrations/065a_agentic_query_runner.down.sql")

	upMustContain := []string{
		"ALTER TABLE agent_runs",
		"ADD COLUMN IF NOT EXISTS job_id UUID",
		"ADD COLUMN IF NOT EXISTS runtime_state JSONB NOT NULL DEFAULT '{}'::jsonb",
		"ADD COLUMN IF NOT EXISTS terminal_version INTEGER NOT NULL DEFAULT 0",
		"ADD COLUMN IF NOT EXISTS query_execute_started BOOLEAN NOT NULL DEFAULT false",
		"CREATE UNIQUE INDEX IF NOT EXISTS ux_agent_runs_job",
		"ON agent_runs(job_id) WHERE job_id IS NOT NULL",
		"ADD CONSTRAINT ux_agent_steps_run_seq UNIQUE (run_id, seq)",
		"CREATE TABLE IF NOT EXISTS agent_shadow_comparisons",
		"job_id UUID NOT NULL",
		"legacy_run_id UUID",
		"agent_run_id UUID",
		"category TEXT NOT NULL",
		"detail JSONB NOT NULL DEFAULT '{}'::jsonb",
	}
	for _, want := range upMustContain {
		if !strings.Contains(up, want) {
			t.Errorf("up migration missing %q", want)
		}
	}

	downMustContain := []string{
		"DROP TABLE IF EXISTS agent_shadow_comparisons",
		"DROP CONSTRAINT IF EXISTS ux_agent_steps_run_seq",
		"DROP INDEX IF EXISTS ux_agent_runs_job",
		"DROP COLUMN IF EXISTS query_execute_started",
		"DROP COLUMN IF EXISTS terminal_version",
		"DROP COLUMN IF EXISTS runtime_state",
		"DROP COLUMN IF EXISTS job_id",
	}
	for _, want := range downMustContain {
		if !strings.Contains(down, want) {
			t.Errorf("down migration missing %q", want)
		}
	}
}
