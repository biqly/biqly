// Package main is the Biqly admin CLI (enrich-context and future subcommands).
package main

import (
	"context"
	"database/sql"
	"flag"
	"log/slog"
	"os"

	"github.com/biqly/biqly/internal/ai/enrichcontext"
	"github.com/biqly/biqly/internal/metadata"
	"github.com/biqly/biqly/internal/semantic"
	"github.com/bytedance/sonic"
	_ "github.com/jackc/pgx/v5/stdlib"
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}
	switch os.Args[1] {
	case "enrich-context":
		runEnrichContext(os.Args[2:])
	default:
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	slog.Info("usage: biqly enrich-context --datasource <id> --model <id> [--dry-run] [--suggest]")
}

func runEnrichContext(args []string) {
	fs := flag.NewFlagSet("enrich-context", flag.ExitOnError)
	datasourceID := fs.String("datasource", "", "Datasource ID")
	modelID := fs.String("model", "", "Semantic model ID")
	dryRun := fs.Bool("dry-run", false, "Analyze only; do not apply suggestions")
	suggest := fs.Bool("suggest", true, "Ask the LLM for enrichment suggestions")
	dsn := fs.String("dsn", os.Getenv("BI_METADATA_DB_DSN"), "Metadata PostgreSQL DSN")
	_ = fs.Parse(args)

	if *datasourceID == "" || *modelID == "" {
		slog.Error("--datasource and --model are required")
		os.Exit(1)
	}
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

	metaRepo := metadata.NewRepository(db)
	semanticRepo := semantic.NewRepository(db)
	svc := enrichcontext.NewService(metaRepo, semanticRepo, nil, nil, nil, nil)

	result, err := svc.Analyze(ctx, enrichcontext.AnalyzeRequest{
		DatasourceID: *datasourceID,
		ModelID:      *modelID,
		Suggest:      *suggest,
	})
	if err != nil {
		slog.Error("analyze failed", "error", err)
		os.Exit(1)
	}

	out, err := sonic.ConfigStd.MarshalIndent(result, "", "  ")
	if err != nil {
		slog.Error("encode result", "error", err)
		os.Exit(1)
	}
	if _, err := os.Stdout.Write(append(out, '\n')); err != nil {
		slog.Error("write result", "error", err)
		os.Exit(1)
	}

	if *dryRun {
		slog.Info("enrich-context dry-run complete", "gaps", len(result.Gaps), "suggestions", len(result.Suggestions))
		return
	}
	if len(result.Suggestions) == 0 {
		slog.Info("enrich-context complete (nothing to apply)", "gaps", len(result.Gaps))
		return
	}

	items := make([]enrichcontext.ApplyItem, 0, len(result.Suggestions))
	for _, s := range result.Suggestions {
		items = append(items, enrichcontext.ApplyItem{GapID: s.GapID, Value: s.Text})
	}
	applyResult, err := svc.Apply(ctx, enrichcontext.ApplyRequest{
		DatasourceID: *datasourceID,
		ModelID:      *modelID,
		Items:        items,
	})
	if err != nil {
		slog.Error("apply failed", "error", err)
		os.Exit(1)
	}
	slog.Info("enrich-context applied", "applied", applyResult.Applied, "skipped", applyResult.Skipped, "errors", len(applyResult.Errors))
}
