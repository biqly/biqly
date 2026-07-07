// Package agent defines the wire contracts for the agentic query-runner
// service: schema-versioned Job/Step/Result/Error envelopes exchanged over
// the NATS agent queue, and their range/shape validation.
package agent

import (
	"errors"
	"fmt"
	"strings"
	"time"
)

// Schema versions gate wire compatibility for each envelope kind. A build
// only accepts the exact version(s) it understands; see ErrUnsupportedSchemaVersion.
const (
	JobSchemaV1    = "agent-job.v1"
	StepSchemaV1   = "agent-step.v1"
	ResultSchemaV1 = "agent-result.v1"
	ErrorSchemaV1  = "agent-error.v1"
)

// Mode is the agent run's blast radius. Shadow runs compute a result for
// comparison without surfacing it to the user; active runs surface results.
const (
	ModeShadow = "shadow"
	ModeActive = "active"
)

// Range bounds shared by job validation and config loading. Mirrored (not
// imported) in internal/config so the two packages stay decoupled.
const (
	MinMaxSteps = 1
	MaxMaxSteps = 6

	MinClarificationRounds = 0
	MaxClarificationRounds = 2

	MinTimeout = 1 * time.Second
	MaxTimeout = 45 * time.Second

	MinMaxRows = 1
	MaxMaxRows = 1000
)

// ErrUnsupportedSchemaVersion is returned when an envelope declares a schema
// from the right family (e.g. "agent-job.*") but a version this build does
// not understand — distinct from an envelope of the wrong kind entirely.
var ErrUnsupportedSchemaVersion = errors.New("unsupported schema version")

// Job is the envelope published to start an agent run.
type Job struct {
	Schema                 string    `json:"schema"`
	JobID                  string    `json:"job_id"`
	RunID                  string    `json:"run_id"`
	ConversationID         string    `json:"conversation_id,omitempty"`
	DatasourceID           string    `json:"datasource_id"`
	ModelID                string    `json:"model_id,omitempty"`
	UserID                 string    `json:"user_id"`
	WorkspaceID            string    `json:"workspace_id,omitempty"`
	Question               string    `json:"question"`
	Mode                   string    `json:"mode"`
	MaxSteps               int       `json:"max_steps"`
	MaxClarificationRounds int       `json:"max_clarification_rounds"`
	TimeoutSeconds         int       `json:"timeout_seconds"`
	MaxRows                int       `json:"max_rows"`
	CreatedAt              time.Time `json:"created_at"`
}

// Step is the envelope published after each planner tool-call attempt.
type Step struct {
	Schema     string    `json:"schema"`
	JobID      string    `json:"job_id"`
	RunID      string    `json:"run_id"`
	Seq        int       `json:"seq"`
	Tool       string    `json:"tool"`
	Status     string    `json:"status"`
	Attempt    int       `json:"attempt"`
	DurationMs int       `json:"duration_ms"`
	Detail     string    `json:"detail,omitempty"`
	CreatedAt  time.Time `json:"created_at"`
}

// Result is the envelope published when a run completes successfully.
type Result struct {
	Schema     string    `json:"schema"`
	JobID      string    `json:"job_id"`
	RunID      string    `json:"run_id"`
	Status     string    `json:"status"`
	Confidence float64   `json:"confidence"`
	Answer     string    `json:"answer"`
	RowCount   int       `json:"row_count"`
	CreatedAt  time.Time `json:"created_at"`
}

// Error is the envelope published when a run fails.
type Error struct {
	Schema     string    `json:"schema"`
	JobID      string    `json:"job_id"`
	RunID      string    `json:"run_id"`
	ReasonCode string    `json:"reason_code"`
	Message    string    `json:"message"`
	Retryable  bool      `json:"retryable"`
	CreatedAt  time.Time `json:"created_at"`
}

// IsValidMode reports whether mode is a recognized agent run mode.
func IsValidMode(mode string) bool {
	return mode == ModeShadow || mode == ModeActive
}

// validateSchema checks an envelope's declared schema against the one this
// build expects. A same-family mismatch (differing only by version suffix)
// reports ErrUnsupportedSchemaVersion; any other mismatch is a shape error.
func validateSchema(got, want string) error {
	if got == want {
		return nil
	}
	gotFamily, gotOK := schemaFamily(got)
	wantFamily, wantOK := schemaFamily(want)
	if gotOK && wantOK && gotFamily == wantFamily {
		return fmt.Errorf("%w: %q (this build supports %q)", ErrUnsupportedSchemaVersion, got, want)
	}
	return fmt.Errorf("schema must be %q, got %q", want, got)
}

func schemaFamily(schema string) (string, bool) {
	idx := strings.LastIndex(schema, ".v")
	if idx < 0 {
		return "", false
	}
	return schema[:idx], true
}

func requireNonEmpty(field, value string) error {
	if strings.TrimSpace(value) == "" {
		return fmt.Errorf("%s is required", field)
	}
	return nil
}

func requireIntRange(field string, value, lo, hi int) error {
	if value < lo || value > hi {
		return fmt.Errorf("%s must be between %d and %d, got %d", field, lo, hi, value)
	}
	return nil
}

// ValidateJob checks a Job envelope's schema version, required IDs, and
// range-bounded fields.
func ValidateJob(j Job) error {
	if err := validateSchema(j.Schema, JobSchemaV1); err != nil {
		return err
	}
	if err := requireNonEmpty("job_id", j.JobID); err != nil {
		return err
	}
	if err := requireNonEmpty("run_id", j.RunID); err != nil {
		return err
	}
	if err := requireNonEmpty("datasource_id", j.DatasourceID); err != nil {
		return err
	}
	if err := requireNonEmpty("user_id", j.UserID); err != nil {
		return err
	}
	if !IsValidMode(j.Mode) {
		return fmt.Errorf("mode must be %q or %q, got %q", ModeShadow, ModeActive, j.Mode)
	}
	if err := requireIntRange("max_steps", j.MaxSteps, MinMaxSteps, MaxMaxSteps); err != nil {
		return err
	}
	if err := requireIntRange("max_clarification_rounds", j.MaxClarificationRounds,
		MinClarificationRounds, MaxClarificationRounds); err != nil {
		return err
	}
	if err := requireIntRange("timeout_seconds", j.TimeoutSeconds,
		int(MinTimeout.Seconds()), int(MaxTimeout.Seconds())); err != nil {
		return err
	}
	if err := requireIntRange("max_rows", j.MaxRows, MinMaxRows, MaxMaxRows); err != nil {
		return err
	}
	return nil
}

// ValidateStep checks a Step envelope's schema version and required IDs.
func ValidateStep(s Step) error {
	if err := validateSchema(s.Schema, StepSchemaV1); err != nil {
		return err
	}
	if err := requireNonEmpty("job_id", s.JobID); err != nil {
		return err
	}
	if err := requireNonEmpty("run_id", s.RunID); err != nil {
		return err
	}
	if err := requireNonEmpty("tool", s.Tool); err != nil {
		return err
	}
	if s.Seq < 1 {
		return fmt.Errorf("seq must be >= 1, got %d", s.Seq)
	}
	return nil
}

// ValidateResult checks a Result envelope's schema version and required IDs.
func ValidateResult(r Result) error {
	if err := validateSchema(r.Schema, ResultSchemaV1); err != nil {
		return err
	}
	if err := requireNonEmpty("job_id", r.JobID); err != nil {
		return err
	}
	if err := requireNonEmpty("run_id", r.RunID); err != nil {
		return err
	}
	if r.Confidence < 0 || r.Confidence > 1 {
		return fmt.Errorf("confidence must be between 0 and 1, got %g", r.Confidence)
	}
	return nil
}

// ValidateError checks an Error envelope's schema version and required IDs.
func ValidateError(e Error) error {
	if err := validateSchema(e.Schema, ErrorSchemaV1); err != nil {
		return err
	}
	if err := requireNonEmpty("job_id", e.JobID); err != nil {
		return err
	}
	if err := requireNonEmpty("run_id", e.RunID); err != nil {
		return err
	}
	if err := requireNonEmpty("reason_code", e.ReasonCode); err != nil {
		return err
	}
	return nil
}
