// Command eval-live runs the nightly golden NL→LogicalQuery suite against a real LLM,
// compares results to a baseline snapshot, and exits non-zero on drift or low pass rate.
package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"time"

	"github.com/biqly/biqly/internal/ai"
	evalpkg "github.com/biqly/biqly/internal/ai/eval"
	"github.com/biqly/biqly/internal/query"
)

func main() {
	suiteName := flag.String("suite", evalpkg.NightlySuiteName, "Suite id for reporting")
	baselinePath := flag.String("baseline", "testdata/eval/nightly_baseline.json", "Baseline snapshot JSON path")
	outputPath := flag.String("output", "eval-live-report.json", "Write combined run + drift report here")
	minPassRate := flag.Float64("min-pass-rate", evalpkg.DefaultLiveMinPassRate, "Minimum pass rate (0-1) before failing")
	failOnDrift := flag.Bool("fail-on-drift", true, "Exit non-zero when baseline comparison finds new failures")
	flag.Parse()

	cfg := evalpkg.LiveAIConfigFromEnv()
	if !cfg.ResolvedQuery().Configured() {
		slog.Error("live eval requires BI_AI_MODEL (or BI_AI_QUERY_MODEL) and BI_AI_API_KEY or BI_AI_BASE_URL")
		os.Exit(2)
	}

	cases := evalpkg.NightlyCases()
	opts := evalpkg.SuiteOptions{
		Cases: cases,
		Modes: evalpkg.ModeLogical | evalpkg.ModeExecution,
	}

	svc := ai.NewService(&cfg, query.NewValidator(1000))
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Minute)
	defer cancel()

	slog.Info("starting live eval",
		"suite", *suiteName,
		"cases", len(cases),
		"model", cfg.Connection.Model,
		"provider", cfg.Connection.Provider,
	)

	result := evalpkg.RunGoldenSuite(ctx, svc, opts)
	snapshot := evalpkg.SnapshotFromSuite(*suiteName, cfg.Connection.Provider, cfg.Connection.Model, result, opts)

	report := evalpkg.LiveRunReport{Snapshot: snapshot}
	if strings.TrimSpace(*baselinePath) != "" {
		baseline, err := evalpkg.LoadRunSnapshot(*baselinePath)
		if err != nil {
			slog.Error("load baseline", "path", *baselinePath, "error", err)
			os.Exit(1)
		}
		report.Drift = evalpkg.CompareSnapshots(baseline, snapshot)
		printDrift(report.Drift)
	}

	if err := evalpkg.SaveLiveRunReport(*outputPath, report); err != nil {
		slog.Error("write report", "path", *outputPath, "error", err)
		os.Exit(1)
	}
	slog.Info("live eval finished",
		"pass_rate", fmt.Sprintf("%.2f", snapshot.PassRate),
		"passed", snapshot.Passed,
		"failed", snapshot.Failed,
		"report", *outputPath,
	)

	exitCode := 0
	if snapshot.PassRate < *minPassRate {
		slog.Error("pass rate below threshold",
			"pass_rate", snapshot.PassRate,
			"min", *minPassRate,
		)
		exitCode = 1
	}
	if *failOnDrift && report.Drift != nil && len(report.Drift.NewFailures) > 0 {
		slog.Error("new failures vs baseline", "count", len(report.Drift.NewFailures))
		exitCode = 1
	}
	if exitCode != 0 {
		os.Exit(exitCode)
	}
}

func printDrift(drift *evalpkg.RegressionReport) {
	if drift == nil {
		return
	}
	slog.Info("drift summary",
		"new_failures", len(drift.NewFailures),
		"fixed_failures", len(drift.FixedFailures),
		"changed_cases", len(drift.ChangedCases),
	)
	for _, ch := range drift.NewFailures {
		slog.Warn("regression", "case_id", ch.CaseID, "question", ch.Question, "reason", ch.IsReason)
	}
}
