// Package main exports Biqly SFT datasets (train/validation/hard_eval JSONL) for Gemma fine-tuning.
package main

import (
	"context"
	"database/sql"
	"flag"
	"log/slog"
	"os"

	"github.com/biqly/biqly/internal/ai"
	"github.com/biqly/biqly/internal/config"
	"github.com/biqly/biqly/internal/metadata"
	"github.com/biqly/biqly/internal/query"
	"github.com/biqly/biqly/internal/semantic"
	_ "github.com/jackc/pgx/v5/stdlib"
)

func main() {
	out := flag.String("out", "data/biqly-gemma4", "Output directory for JSONL files")
	dsn := flag.String("dsn", os.Getenv("BI_METADATA_DB_DSN"), "Metadata PostgreSQL DSN")
	trainRatio := flag.Float64("train-ratio", 0.8, "Fraction of examples for train.jsonl")
	valRatio := flag.Float64("validation-ratio", 0.1, "Fraction for validation.jsonl (remainder → hard_eval)")
	maxPrompt := flag.Int("max-prompt-runes", 80000, "PromptBuilder max runes cap")
	minConf := flag.Float64("min-history-confidence", 0.7, "Minimum confidence for ai_query_history rows")
	includeGolden := flag.Bool("include-golden", true, "Include built-in DefaultGoldenCases()")
	flag.Parse()

	if *dsn == "" {
		slog.Error("BI_METADATA_DB_DSN is required (or pass -dsn)")
		os.Exit(1)
	}

	ctx := context.Background()
	db, err := sql.Open("pgx", *dsn)
	if err != nil {
		slog.Error("open metadata db", "error", err)
		os.Exit(1)
	}
	defer func() { _ = db.Close() }()
	if err := db.PingContext(ctx); err != nil {
		slog.Error("ping metadata db", "error", err)
		os.Exit(1)
	}

	cfg, err := config.Load()
	if err != nil {
		slog.Warn("config load failed; using defaults for validator", "error", err)
		cfg = &config.Config{}
	}
	if cfg.Query.MaxRows <= 0 {
		cfg.Query.MaxRows = 10000
	}

	metaRepo := metadata.NewRepository(db)
	semanticRepo := semantic.NewRepository(db)
	exporter := ai.NewSFTExporter(metaRepo, semanticRepo, query.NewValidator(cfg.Query.MaxRows))

	result, err := exporter.Export(ctx, ai.SFTExportOptions{
		OutDir:          *out,
		TrainRatio:      *trainRatio,
		ValidationRatio: *valRatio,
		MaxPromptRunes:  *maxPrompt,
		MinHistoryConf:  *minConf,
		IncludeGolden:   *includeGolden,
	})
	if err != nil {
		slog.Error("export failed", "error", err)
		os.Exit(1)
	}

	slog.Info("sft export completed",
		"out", *out,
		"train", result.TrainCount,
		"validation", result.ValidationCount,
		"hard_eval", result.HardEvalCount,
		"skipped", result.Skipped,
	)
	for _, e := range result.Errors {
		slog.Warn("sft export warning", "warning", e)
	}
	if result.TrainCount+result.ValidationCount+result.HardEvalCount == 0 {
		slog.Error("no records exported — add few_shot_examples or successful ai_query_history rows")
		os.Exit(1)
	}
}
