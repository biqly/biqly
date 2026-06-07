package eval

import (
	"fmt"
	"os"
	"time"

	"github.com/bytedance/sonic"
)

// DefaultLiveMinPassRate is the minimum full-suite pass rate for nightly live LLM eval.
const DefaultLiveMinPassRate = 0.85

// CaseSnapshot records one case outcome for baseline comparison.
type CaseSnapshot struct {
	Question string `json:"question,omitempty"`
	Match    bool   `json:"match"`
	Reason   string `json:"reason,omitempty"`
}

// RunSnapshot is a portable eval run summary used for drift detection in CI.
type RunSnapshot struct {
	Suite    string                  `json:"suite"`
	RunID    string                  `json:"run_id,omitempty"`
	Provider string                  `json:"provider,omitempty"`
	Model    string                  `json:"model,omitempty"`
	PassRate float64                 `json:"pass_rate"`
	Total    int                     `json:"total"`
	Passed   int                     `json:"passed"`
	Failed   int                     `json:"failed"`
	Cases    map[string]CaseSnapshot `json:"cases"`
	At       time.Time               `json:"at,omitempty"`
}

// LiveRunReport combines a live suite result with optional drift vs a baseline snapshot.
type LiveRunReport struct {
	Snapshot RunSnapshot       `json:"snapshot"`
	Drift    *RegressionReport `json:"drift,omitempty"`
}

// SnapshotFromSuite builds a RunSnapshot from a suite run.
func SnapshotFromSuite(suite, provider, model string, result *SuiteResult, opts SuiteOptions) RunSnapshot {
	if result == nil {
		return RunSnapshot{Suite: suite, Provider: provider, Model: model, At: time.Now().UTC()}
	}
	snap := RunSnapshot{
		Suite:    suite,
		Provider: provider,
		Model:    model,
		PassRate: result.PassRate,
		Total:    result.Total,
		Passed:   result.Passed,
		Failed:   result.Failed,
		Cases:    make(map[string]CaseSnapshot, len(result.Cases)),
		At:       time.Now().UTC(),
	}
	for _, c := range result.Cases {
		reason := c.LogicalReason
		if reason == "" && !c.ExecutionMatch {
			reason = c.ExecutionReason
		}
		if c.Err != nil {
			reason = c.Err.Error()
		}
		snap.Cases[c.Case.ID] = CaseSnapshot{
			Question: c.Case.Question,
			Match:    c.Pass(opts),
			Reason:   reason,
		}
	}
	return snap
}

// CompareSnapshots diffs baseline and current case outcomes (same logic as DB regression).
func CompareSnapshots(baseline, current RunSnapshot) *RegressionReport {
	base := make(map[string]caseOutcome, len(baseline.Cases))
	for id, c := range baseline.Cases {
		base[id] = caseOutcome(c)
	}
	cur := make(map[string]caseOutcome, len(current.Cases))
	for id, c := range current.Cases {
		cur[id] = caseOutcome(c)
	}
	return buildRegressionReport(baseline.RunID, current.RunID, base, cur)
}

// LoadRunSnapshot reads a JSON baseline or prior run snapshot from disk.
func LoadRunSnapshot(path string) (RunSnapshot, error) {
	//nolint:gosec // caller supplies trusted repo-relative baseline path in CI
	data, err := os.ReadFile(path)
	if err != nil {
		return RunSnapshot{}, fmt.Errorf("read snapshot %q: %w", path, err)
	}
	var snap RunSnapshot
	if err := sonic.ConfigStd.Unmarshal(data, &snap); err != nil {
		return RunSnapshot{}, fmt.Errorf("parse snapshot %q: %w", path, err)
	}
	if snap.Cases == nil {
		snap.Cases = map[string]CaseSnapshot{}
	}
	return snap, nil
}

// SaveRunSnapshot writes a run snapshot JSON file.
func SaveRunSnapshot(path string, snap RunSnapshot) error {
	if snap.Cases == nil {
		snap.Cases = map[string]CaseSnapshot{}
	}
	return writeJSONFile(path, snap)
}

// SaveLiveRunReport writes the combined nightly report JSON.
func SaveLiveRunReport(path string, report LiveRunReport) error {
	return writeJSONFile(path, report)
}

func writeJSONFile(path string, v any) error {
	data, err := sonic.ConfigStd.MarshalIndent(v, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal json: %w", err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return fmt.Errorf("write json %q: %w", path, err)
	}
	return nil
}

// BaselineSnapshotFromCases builds an all-pass baseline for a case list.
func BaselineSnapshotFromCases(suite string, cases []GoldenCase) RunSnapshot {
	snap := RunSnapshot{
		Suite:    suite,
		PassRate: 1,
		Total:    len(cases),
		Passed:   len(cases),
		Cases:    make(map[string]CaseSnapshot, len(cases)),
	}
	for _, c := range cases {
		snap.Cases[c.ID] = CaseSnapshot{
			Question: c.Question,
			Match:    true,
		}
	}
	return snap
}
