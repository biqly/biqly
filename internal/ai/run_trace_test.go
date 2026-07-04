package ai

import (
	"strings"
	"sync"
	"testing"
)

func TestRunRecorderOrdersSteps(t *testing.T) {
	rec := NewRunRecorder()
	rec.Record("table_route", RunStepStatusOK, 0, 12, "")
	rec.Record("prompt_build", RunStepStatusOK, 0, 3, "tier=minimal")
	rec.Record("llm_generate", RunStepStatusFailed, 1, 850, "provider timeout")

	steps := rec.Steps()
	if len(steps) != 3 {
		t.Fatalf("expected 3 steps, got %d", len(steps))
	}
	for i, step := range steps {
		if step.Seq != i+1 {
			t.Errorf("step %d: expected seq %d, got %d", i, i+1, step.Seq)
		}
	}
	if steps[2].Status != RunStepStatusFailed || steps[2].Attempt != 1 || steps[2].Detail != "provider timeout" {
		t.Errorf("unexpected failed step: %+v", steps[2])
	}
}

func TestRunRecorderNilSafe(t *testing.T) {
	var rec *RunRecorder
	rec.Record("llm_generate", RunStepStatusOK, 0, 1, "")
	if steps := rec.Steps(); steps != nil {
		t.Fatalf("expected nil steps from nil recorder, got %v", steps)
	}

	var opts *processOptions
	opts.observeStep("llm_generate", 1, RunStepStatusOK, 0, "")

	opts = &processOptions{}
	opts.observeStep("llm_generate", 1, RunStepStatusOK, 0, "")
}

func TestRunRecorderTruncatesDetail(t *testing.T) {
	rec := NewRunRecorder()
	rec.Record("parse_validate", RunStepStatusFailed, 0, 1, strings.Repeat("ü", maxRunStepDetailRunes+50))

	steps := rec.Steps()
	if got := len([]rune(steps[0].Detail)); got != maxRunStepDetailRunes+1 {
		t.Fatalf("expected detail truncated to %d runes plus ellipsis, got %d", maxRunStepDetailRunes, got)
	}
	if !strings.HasSuffix(steps[0].Detail, "…") {
		t.Fatal("expected truncated detail to end with ellipsis")
	}
}

func TestRunRecorderConcurrentRecords(t *testing.T) {
	rec := NewRunRecorder()
	const workers = 16
	var wg sync.WaitGroup
	wg.Add(workers)
	for i := range workers {
		go func() {
			defer wg.Done()
			rec.Record("llm_generate", RunStepStatusOK, i, 5, "")
		}()
	}
	wg.Wait()

	steps := rec.Steps()
	if len(steps) != workers {
		t.Fatalf("expected %d steps, got %d", workers, len(steps))
	}
	seen := make(map[int]bool, workers)
	for _, step := range steps {
		if seen[step.Seq] {
			t.Fatalf("duplicate seq %d", step.Seq)
		}
		seen[step.Seq] = true
	}
}

func TestObserveStepFansOutToObserverAndRecorder(t *testing.T) {
	var observed []string
	rec := NewRunRecorder()
	opts := &processOptions{
		stepObserver: func(step string, _ int64) { observed = append(observed, step) },
		runRecorder:  rec,
	}

	opts.observeStep("sql_dry_run", 42, RunStepStatusOK, 2, "")

	if len(observed) != 1 || observed[0] != "sql_dry_run" {
		t.Fatalf("expected step observer call, got %v", observed)
	}
	steps := rec.Steps()
	if len(steps) != 1 || steps[0].Kind != "sql_dry_run" || steps[0].DurationMs != 42 || steps[0].Attempt != 2 {
		t.Fatalf("unexpected recorded step: %+v", steps)
	}
}
