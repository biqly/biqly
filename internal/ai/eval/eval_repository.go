package eval

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	platformdb "github.com/biqly/biqly/internal/platform/db"
	"github.com/google/uuid"
)

// EvalResultRecord is the persisted form of an eval run result.
type EvalResultRecord struct {
	ID               string    `json:"id"`
	RunID            string    `json:"run_id"`
	Provider         string    `json:"provider"`
	Model            string    `json:"model"`
	ContextVersion   int       `json:"context_version"`
	ContextUpdatedAt time.Time `json:"context_updated_at"`
	CaseID           string    `json:"case_id"`
	Question         string    `json:"question"`
	ExpectedLQ       string    `json:"expected_lq"`
	GotLQ            string    `json:"got_lq"`
	Match            bool      `json:"match"`
	Reason           string    `json:"reason"`
	Confidence       float64   `json:"confidence"`
	LatencyMs        int64     `json:"latency_ms"`
	TokenCount       int       `json:"token_count"`
	CreatedAt        time.Time `json:"created_at"`
}

// EvalRunSummary is the aggregate summary of an eval run.
type EvalRunSummary struct {
	RunID                       string         `json:"run_id"`
	Provider                    string         `json:"provider"`
	Model                       string         `json:"model"`
	ContextVersion              int            `json:"context_version"`
	TotalCases                  int            `json:"total_cases"`
	Passed                      int            `json:"passed"`
	Failed                      int            `json:"failed"`
	PassRate                    float64        `json:"pass_rate"`
	AvgConfidence               float64        `json:"avg_confidence"`
	AvgLatencyMs                float64        `json:"avg_latency_ms"`
	TotalTokens                 int            `json:"total_tokens"`
	StartedAt                   time.Time      `json:"started_at"`
	CompletedAt                 time.Time      `json:"completed_at"`
	PromptTemplateVersions      map[string]int `json:"prompt_template_versions,omitempty"`
	PromptTemplateBundleVersion int            `json:"prompt_template_bundle_version,omitempty"`
}

// RegressionReport compares two eval runs and identifies regressions.
type RegressionReport struct {
	BaselineRunID string             `json:"baseline_run_id"`
	CurrentRunID  string             `json:"current_run_id"`
	NewFailures   []RegressionChange `json:"new_failures"`
	FixedFailures []RegressionChange `json:"fixed_failures"`
	ChangedCases  []RegressionChange `json:"changed_cases"`
}

// RegressionChange describes a single case that changed between runs.
type RegressionChange struct {
	CaseID    string `json:"case_id"`
	Question  string `json:"question"`
	WasMatch  bool   `json:"was_match"`
	IsMatch   bool   `json:"is_match"`
	WasReason string `json:"was_reason"`
	IsReason  string `json:"is_reason"`
}

// EvalRepository stores and retrieves eval run results.
type EvalRepository struct {
	db *sql.DB
}

// NewEvalRepository creates a new eval repository.
func NewEvalRepository(db *sql.DB) *EvalRepository {
	return &EvalRepository{db: db}
}

// SaveRunResults persists all results from one eval run.
func (r *EvalRepository) SaveRunResults(ctx context.Context, runID, provider, model string, contextVersion int, contextUpdatedAt time.Time, results []EvalResultWithMetrics) error {
	if len(results) == 0 {
		return nil
	}

	return platformdb.RunInTx(ctx, r.db, func(tx *sql.Tx) error {
		return r.saveRunResultsTx(ctx, tx, runID, provider, model, contextVersion, contextUpdatedAt, results)
	})
}

func (r *EvalRepository) saveRunResultsTx(ctx context.Context, tx *sql.Tx, runID, provider, model string, contextVersion int, contextUpdatedAt time.Time, results []EvalResultWithMetrics) error {
	for _, res := range results {
		gotLQ := ""
		if res.Got != nil {
			data, _ := json.Marshal(res.Got)
			gotLQ = string(data)
		}
		expectedLQ := ""
		if data, err := json.Marshal(res.Case.Expected); err == nil {
			expectedLQ = string(data)
		}

		_, err := tx.ExecContext(ctx,
			`INSERT INTO eval_results (
				id, run_id, provider, model, context_version, context_updated_at,
				case_id, question, expected_lq, got_lq, match, reason,
				confidence, latency_ms, token_count
			) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15)`,
			uuid.New().String(), runID, provider, model, contextVersion, contextUpdatedAt,
			res.Case.ID, res.Case.Question, expectedLQ, gotLQ, res.Match, res.Reason,
			res.Confidence, res.LatencyMs, res.TokenCount,
		)
		if err != nil {
			return fmt.Errorf("insert eval result for case %s: %w", res.Case.ID, err)
		}
	}

	// Store run summary
	var totalTokens, passed, failed int
	var totalConfidence, totalLatency float64
	var promptTemplateVersions map[string]int
	var promptTemplateBundleVersion int
	for _, res := range results {
		if res.Match {
			passed++
		} else {
			failed++
		}
		totalTokens += res.TokenCount
		totalConfidence += res.Confidence
		totalLatency += float64(res.LatencyMs)
		if len(promptTemplateVersions) == 0 && len(res.PromptTemplateVersions) > 0 {
			promptTemplateVersions = res.PromptTemplateVersions
			promptTemplateBundleVersion = res.PromptTemplateBundleVersion
		}
	}
	n := float64(len(results))
	if promptTemplateVersions == nil {
		promptTemplateVersions = map[string]int{}
	}
	promptTemplateVersionsJSON, err := json.Marshal(promptTemplateVersions)
	if err != nil {
		return fmt.Errorf("marshal prompt template versions: %w", err)
	}

	_, err = tx.ExecContext(ctx,
		`INSERT INTO eval_runs (
			run_id, provider, model, context_version, total_cases, passed, failed,
			avg_confidence, avg_latency_ms, total_tokens, completed_at,
			prompt_template_versions, prompt_template_bundle_version
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12::jsonb,$13)
		ON CONFLICT (run_id) DO UPDATE SET
			provider = EXCLUDED.provider,
			model = EXCLUDED.model,
			context_version = EXCLUDED.context_version,
			total_cases = EXCLUDED.total_cases,
			passed = EXCLUDED.passed,
			failed = EXCLUDED.failed,
			avg_confidence = EXCLUDED.avg_confidence,
			avg_latency_ms = EXCLUDED.avg_latency_ms,
			total_tokens = EXCLUDED.total_tokens,
			completed_at = EXCLUDED.completed_at,
			prompt_template_versions = EXCLUDED.prompt_template_versions,
			prompt_template_bundle_version = EXCLUDED.prompt_template_bundle_version`,
		runID, provider, model, contextVersion, len(results), passed, failed,
		totalConfidence/n, totalLatency/n, totalTokens, time.Now(),
		string(promptTemplateVersionsJSON), promptTemplateBundleVersion,
	)
	if err != nil {
		return fmt.Errorf("insert eval run summary: %w", err)
	}

	return nil
}

// GetRunResults returns all results for a given run.
func (r *EvalRepository) GetRunResults(ctx context.Context, runID string) ([]EvalResultRecord, error) {
	return platformdb.QuerySliceErr(ctx, r.db, "query eval results",
		`SELECT id, run_id, provider, model, context_version, context_updated_at,
			case_id, question, expected_lq, got_lq, match, reason,
			confidence, latency_ms, token_count, created_at
		FROM eval_results WHERE run_id = $1 ORDER BY case_id`,
		[]any{runID}, scanEvalResultRecord)
}

// GetRunSummary returns the aggregate summary for a run.
func (r *EvalRepository) GetRunSummary(ctx context.Context, runID string) (*EvalRunSummary, error) {
	var s EvalRunSummary
	err := r.db.QueryRowContext(ctx,
		`SELECT run_id, provider, model, context_version, total_cases, passed, failed,
			avg_confidence, avg_latency_ms, total_tokens, completed_at,
			prompt_template_versions, prompt_template_bundle_version
		FROM eval_runs WHERE run_id = $1`,
		runID,
	).Scan(&s.RunID, &s.Provider, &s.Model, &s.ContextVersion, &s.TotalCases, &s.Passed, &s.Failed, &s.AvgConfidence, &s.AvgLatencyMs, &s.TotalTokens, &s.CompletedAt, promptTemplateVersionsScanner(&s.PromptTemplateVersions), &s.PromptTemplateBundleVersion)
	if err != nil {
		return nil, fmt.Errorf("query eval run summary: %w", err)
	}
	if s.TotalCases > 0 {
		s.PassRate = float64(s.Passed) / float64(s.TotalCases) * 100
	}
	return &s, nil
}

// ListRuns returns all eval runs, most recent first.
func (r *EvalRepository) ListRuns(ctx context.Context, limit int) ([]EvalRunSummary, error) {
	if limit <= 0 {
		limit = 20
	}
	return platformdb.QuerySliceErr(ctx, r.db, "list eval runs",
		`SELECT run_id, provider, model, context_version, total_cases, passed, failed,
			avg_confidence, avg_latency_ms, total_tokens, completed_at,
			prompt_template_versions, prompt_template_bundle_version
		FROM eval_runs ORDER BY completed_at DESC LIMIT $1`,
		[]any{limit}, scanEvalRunSummary)
}

// GenerateRegressionReport compares two runs and produces a regression report.
func (r *EvalRepository) GenerateRegressionReport(ctx context.Context, baselineRunID, currentRunID string) (*RegressionReport, error) {
	baseline, err := r.GetRunResults(ctx, baselineRunID)
	if err != nil {
		return nil, fmt.Errorf("get baseline results: %w", err)
	}
	current, err := r.GetRunResults(ctx, currentRunID)
	if err != nil {
		return nil, fmt.Errorf("get current results: %w", err)
	}

	baselineMap := make(map[string]EvalResultRecord)
	for _, res := range baseline {
		baselineMap[res.CaseID] = res
	}

	currentMap := make(map[string]EvalResultRecord)
	for _, res := range current {
		currentMap[res.CaseID] = res
	}

	var report RegressionReport
	report.BaselineRunID = baselineRunID
	report.CurrentRunID = currentRunID

	// Check current cases against baseline
	for caseID, cur := range currentMap {
		base, exists := baselineMap[caseID]
		if !exists {
			// New case - if it fails, it's a new failure
			if !cur.Match {
				report.NewFailures = append(report.NewFailures, RegressionChange{
					CaseID:   caseID,
					Question: cur.Question,
					WasMatch: true, // no prior data = assumed pass
					IsMatch:  false,
					IsReason: cur.Reason,
				})
			}
			continue
		}

		// Both exist - compare
		if base.Match && !cur.Match {
			report.NewFailures = append(report.NewFailures, RegressionChange{
				CaseID:    caseID,
				Question:  cur.Question,
				WasMatch:  true,
				IsMatch:   false,
				WasReason: base.Reason,
				IsReason:  cur.Reason,
			})
		} else if !base.Match && cur.Match {
			report.FixedFailures = append(report.FixedFailures, RegressionChange{
				CaseID:    caseID,
				Question:  cur.Question,
				WasMatch:  false,
				IsMatch:   true,
				WasReason: base.Reason,
			})
		} else if base.Reason != cur.Reason {
			report.ChangedCases = append(report.ChangedCases, RegressionChange{
				CaseID:    caseID,
				Question:  cur.Question,
				WasMatch:  base.Match,
				IsMatch:   cur.Match,
				WasReason: base.Reason,
				IsReason:  cur.Reason,
			})
		}
	}

	// Check baseline cases no longer in current
	for caseID, base := range baselineMap {
		if _, exists := currentMap[caseID]; !exists && !base.Match {
			// Was failing, now missing - could be fixed or removed
			report.FixedFailures = append(report.FixedFailures, RegressionChange{
				CaseID:    caseID,
				Question:  base.Question,
				WasMatch:  false,
				IsMatch:   true,
				WasReason: base.Reason,
			})
		}
	}

	return &report, nil
}

// EvalResultWithMetrics extends EvalResult with runtime metrics for persistence.
type EvalResultWithMetrics struct {
	EvalResult
	Confidence                  float64        `json:"confidence"`
	LatencyMs                   int64          `json:"latency_ms"`
	TokenCount                  int            `json:"token_count"`
	PromptTemplateVersions      map[string]int `json:"prompt_template_versions,omitempty"`
	PromptTemplateBundleVersion int            `json:"prompt_template_bundle_version,omitempty"`
}

func scanEvalResultRecord(s platformdb.Scanner) (EvalResultRecord, error) {
	var rec EvalResultRecord
	if err := s.Scan(&rec.ID, &rec.RunID, &rec.Provider, &rec.Model, &rec.ContextVersion, &rec.ContextUpdatedAt, &rec.CaseID, &rec.Question, &rec.ExpectedLQ, &rec.GotLQ, &rec.Match, &rec.Reason, &rec.Confidence, &rec.LatencyMs, &rec.TokenCount, &rec.CreatedAt); err != nil {
		return rec, fmt.Errorf("scan eval result: %w", err)
	}
	return rec, nil
}

func scanEvalRunSummary(s platformdb.Scanner) (EvalRunSummary, error) {
	var summary EvalRunSummary
	if err := s.Scan(
		&summary.RunID,
		&summary.Provider,
		&summary.Model,
		&summary.ContextVersion,
		&summary.TotalCases,
		&summary.Passed,
		&summary.Failed,
		&summary.AvgConfidence,
		&summary.AvgLatencyMs,
		&summary.TotalTokens,
		&summary.CompletedAt,
		promptTemplateVersionsScanner(&summary.PromptTemplateVersions),
		&summary.PromptTemplateBundleVersion,
	); err != nil {
		return summary, fmt.Errorf("scan eval run: %w", err)
	}
	if summary.TotalCases > 0 {
		summary.PassRate = float64(summary.Passed) / float64(summary.TotalCases) * 100
	}
	return summary, nil
}

type promptTemplateVersionScanTarget struct {
	dest *map[string]int
}

func promptTemplateVersionsScanner(dest *map[string]int) promptTemplateVersionScanTarget {
	return promptTemplateVersionScanTarget{dest: dest}
}

func (s promptTemplateVersionScanTarget) Scan(src any) error {
	if s.dest == nil {
		return nil
	}
	if src == nil {
		*s.dest = nil
		return nil
	}
	var b []byte
	switch v := src.(type) {
	case []byte:
		b = v
	case string:
		b = []byte(v)
	default:
		return fmt.Errorf("unsupported prompt template versions type %T", src)
	}
	if len(b) == 0 {
		*s.dest = nil
		return nil
	}
	return json.Unmarshal(b, s.dest)
}
