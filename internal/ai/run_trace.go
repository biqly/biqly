package ai

import (
	"slices"
	"sync"
	"unicode/utf8"
)

const (
	RunStepStatusOK     = "ok"
	RunStepStatusFailed = "failed"

	maxRunStepDetailRunes = 300
)

// RunStep is one recorded phase of an AI query run (table_route, prompt_build,
// llm_generate, parse_validate, sql_dry_run…). Steps are append-only and
// ordered by Seq; Detail carries a short, PII-free note such as a truncated
// error message — never prompt or result content.
type RunStep struct {
	Seq        int    `json:"seq"`
	Kind       string `json:"kind"`
	Status     string `json:"status"`
	Attempt    int    `json:"attempt,omitempty"`
	DurationMs int64  `json:"duration_ms"`
	Detail     string `json:"detail,omitempty"`
}

// RunRecorder collects RunSteps for a single AI run. It is safe for concurrent
// use: multi-candidate generation records llm_generate steps from N goroutines.
// A nil *RunRecorder is a no-op.
type RunRecorder struct {
	mu    sync.Mutex
	steps []RunStep
}

func NewRunRecorder() *RunRecorder { return &RunRecorder{} }

func (r *RunRecorder) Record(kind, status string, attempt int, durationMs int64, detail string) {
	if r == nil {
		return
	}
	detail = truncateRunes(detail, maxRunStepDetailRunes)
	r.mu.Lock()
	defer r.mu.Unlock()
	r.steps = append(r.steps, RunStep{
		Seq:        len(r.steps) + 1,
		Kind:       kind,
		Status:     status,
		Attempt:    attempt,
		DurationMs: durationMs,
		Detail:     detail,
	})
}

func (r *RunRecorder) Steps() []RunStep {
	if r == nil {
		return nil
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	return slices.Clone(r.steps)
}

func truncateRunes(s string, maxRunes int) string {
	if utf8.RuneCountInString(s) <= maxRunes {
		return s
	}
	runes := []rune(s)
	return string(runes[:maxRunes]) + "…"
}

// WithRunRecorder attaches a per-run step recorder that captures a typed,
// ordered timeline of the pipeline (persisted as AIMetadata.RunSteps).
func WithRunRecorder(rec *RunRecorder) ProcessOption {
	return func(o *processOptions) { o.runRecorder = rec }
}

// observeStep fans a pipeline step out to both the Prometheus step observer
// (kind + latency only, unchanged behavior) and the run recorder (typed step
// with status/attempt/detail).
func (o *processOptions) observeStep(kind string, durationMs int64, status string, attempt int, detail string) {
	if o == nil {
		return
	}
	if o.stepObserver != nil {
		o.stepObserver(kind, durationMs)
	}
	o.runRecorder.Record(kind, status, attempt, durationMs, detail)
}

func runStepStatus(err error) string {
	if err != nil {
		return RunStepStatusFailed
	}
	return RunStepStatusOK
}

func runStepDetail(err error) string {
	if err != nil {
		return err.Error()
	}
	return ""
}
