// Package main implements the conversation-repair operator CLI.
//
// It detects and reversibly removes replay-chain duplicate messages from
// historical conversation snapshots. Default behavior is read-only; mutations
// require explicit --run-id confirmation.
package main

import (
	"context"
	"database/sql"
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"os"
	"time"

	"github.com/biqly/biqly/internal/metadata"
	"github.com/bytedance/sonic"
	_ "github.com/jackc/pgx/v5/stdlib"
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	cmd := os.Args[1]
	fs := flag.NewFlagSet(cmd, flag.ExitOnError)
	dryRun := fs.Bool("dry-run", false, "report only; do not modify data (detect command)")
	conversationID := fs.String("conversation-id", "", "conversation id (report command)")
	runID := fs.String("run-id", "", "repair run id (archive/apply/restore/purge commands)")
	confirmPurge := fs.String("confirm-purge", "", "required for purge: must match the run id")
	_ = fs.Parse(os.Args[2:])

	dsn := os.Getenv("BI_METADATA_DB_DSN")
	if dsn == "" {
		fmt.Fprintln(os.Stderr, "BI_METADATA_DB_DSN is required")
		os.Exit(1)
	}

	db, err := sql.Open("pgx", dsn)
	if err != nil {
		slog.Error("failed to open database", "error", err)
		os.Exit(1)
	}
	defer func() { _ = db.Close() }()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := db.PingContext(ctx); err != nil {
		slog.Error("database not available", "error", err)
		os.Exit(1)
	}

	repo := metadata.NewRepository(db)

	switch cmd {
	case "detect":
		err = runDetect(ctx, repo, *dryRun, os.Stdout)
	case "report":
		err = runReport(ctx, repo, *conversationID, os.Stdout)
	case "archive":
		err = runArchive(ctx, repo, *runID, os.Stdout)
	case "apply":
		err = runApply(ctx, repo, *runID, os.Stdout)
	case "restore":
		err = runRestore(ctx, repo, *runID, os.Stdout)
	case "purge":
		if *confirmPurge != *runID {
			fmt.Fprintln(os.Stderr, "purge requires --confirm-purge <run-id>")
			os.Exit(1)
		}
		err = runPurge(ctx, repo, *runID, os.Stdout)
	default:
		printUsage()
		os.Exit(1)
	}

	if err != nil {
		slog.Error("command failed", "cmd", cmd, "error", err)
		os.Exit(1)
	}
}

func runDetect(ctx context.Context, repo *metadata.Repository, dryRun bool, out io.Writer) error {
	// Find all conversations and check each for replay chains.
	rows, err := repo.DB().QueryContext(ctx, `SELECT DISTINCT conversation_id FROM ai_conversation_messages`)
	if err != nil {
		return fmt.Errorf("list conversations: %w", err)
	}
	defer func() { _ = rows.Close() }()

	type detectResult struct {
		ConversationID string `json:"conversation_id"`
		HasChain       bool   `json:"has_chain"`
		KeepCount      int    `json:"keep_count"`
		ReplayCount    int    `json:"replay_count"`
		CanonicalHash  string `json:"canonical_hash,omitempty"`
		Reason         string `json:"reason,omitempty"`
	}

	var results []detectResult
	var candidates int
	for rows.Next() {
		var convID string
		if err := rows.Scan(&convID); err != nil {
			return fmt.Errorf("scan conversation id: %w", err)
		}

		messages, err := repo.LoadRepairMessages(ctx, convID)
		if err != nil {
			return fmt.Errorf("load messages for %s: %w", convID, err)
		}

		candidate, ok := metadata.DetectReplayChain(convID, messages, metadata.RepairBatchGap())
		if !ok {
			results = append(results, detectResult{ConversationID: convID, HasChain: false})
			continue
		}

		candidates++
		results = append(results, detectResult{
			ConversationID: convID,
			HasChain:       true,
			KeepCount:      len(candidate.KeepIDs),
			ReplayCount:    len(candidate.ReplayIDs),
			CanonicalHash:  candidate.CanonicalHash,
			Reason:         candidate.Reason,
		})

		// Persist a detect run so apply can reference it later; skipped in dry-run.
		if !dryRun {
			_, err := repo.CreateRepairRun(ctx, "detect", candidate.CanonicalHash, len(candidate.ReplayIDs))
			if err != nil {
				return fmt.Errorf("create detect run for %s: %w", convID, err)
			}
		}
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate conversations: %w", err)
	}

	mode := "detect"
	if dryRun {
		mode = "dry-run"
	}
	report := map[string]any{
		"detect_mode":   mode,
		"candidates":    candidates,
		"total_scanned": len(results),
		"results":       results,
	}
	return writeJSON(out, report)
}

func runReport(ctx context.Context, repo *metadata.Repository, conversationID string, out io.Writer) error {
	if conversationID == "" {
		return errors.New("--conversation-id is required for report")
	}

	messages, err := repo.LoadRepairMessages(ctx, conversationID)
	if err != nil {
		return fmt.Errorf("load messages: %w", err)
	}

	candidate, ok := metadata.DetectReplayChain(conversationID, messages, metadata.RepairBatchGap())

	report := map[string]any{
		"conversation_id": conversationID,
		"message_count":   len(messages),
		"has_chain":       ok,
	}
	if ok {
		report["candidate"] = candidate
	}
	return writeJSON(out, report)
}

func runArchive(ctx context.Context, repo *metadata.Repository, runID string, out io.Writer) error {
	if runID == "" {
		return errors.New("--run-id is required for archive")
	}
	run, err := repo.GetRepairRun(ctx, runID)
	if err != nil {
		return fmt.Errorf("get repair run: %w", err)
	}
	return writeJSON(out, map[string]any{
		"run_id":     run.ID,
		"mode":       run.Mode,
		"status":     run.Status,
		"candidates": run.CandidateCount,
		"canonical":  run.CanonicalHash,
		"message":    "run is ready for apply",
	})
}

func runApply(ctx context.Context, repo *metadata.Repository, runID string, out io.Writer) error {
	if runID == "" {
		return errors.New("--run-id is required for apply")
	}
	run, err := repo.GetRepairRun(ctx, runID)
	if err != nil {
		return fmt.Errorf("get repair run: %w", err)
	}
	if run.Status != "pending" {
		return fmt.Errorf("run %s is not pending (status=%s)", runID, run.Status)
	}

	// Re-detect the chain: the canonical hash in the run identifies the
	// target conversation, guarding against data changes since detection.
	convID, err := findConversationByHash(ctx, repo, run.CanonicalHash)
	if err != nil {
		return fmt.Errorf("find conversation by hash: %w", err)
	}
	messages, err := repo.LoadRepairMessages(ctx, convID)
	if err != nil {
		return fmt.Errorf("load messages for %s: %w", convID, err)
	}

	candidate, ok := metadata.DetectReplayChain(convID, messages, metadata.RepairBatchGap())
	if !ok {
		return fmt.Errorf("no replay chain detected for conversation %s", convID)
	}
	if candidate.CanonicalHash != run.CanonicalHash {
		return errors.New("canonical hash changed since detection")
	}

	if err := repo.ApplyRepairRun(ctx, runID, candidate); err != nil {
		return fmt.Errorf("apply repair: %w", err)
	}

	return writeJSON(out, map[string]any{
		"run_id":       runID,
		"conversation": convID,
		"applied":      len(candidate.ReplayIDs),
		"kept":         len(candidate.KeepIDs),
		"canonical":    candidate.CanonicalHash,
	})
}

func runRestore(ctx context.Context, repo *metadata.Repository, runID string, out io.Writer) error {
	if runID == "" {
		return errors.New("--run-id is required for restore")
	}
	if err := repo.RestoreRepairRun(ctx, runID); err != nil {
		return fmt.Errorf("restore: %w", err)
	}
	return writeJSON(out, map[string]any{
		"run_id":  runID,
		"message": "soft-delete markers cleared; messages restored",
	})
}

func runPurge(ctx context.Context, repo *metadata.Repository, runID string, out io.Writer) error {
	if runID == "" {
		return errors.New("--run-id is required for purge")
	}
	if err := repo.PurgeRepairRun(ctx, runID); err != nil {
		return fmt.Errorf("purge: %w", err)
	}
	return writeJSON(out, map[string]any{
		"run_id":  runID,
		"message": "archived messages physically deleted; archive rows retained for audit",
	})
}

// findConversationByHash scans conversations for one whose current replay chain
// produces the given canonical hash.
func findConversationByHash(ctx context.Context, repo *metadata.Repository, hash string) (string, error) {
	rows, err := repo.DB().QueryContext(ctx, `SELECT DISTINCT conversation_id FROM ai_conversation_messages WHERE deleted_at IS NULL`)
	if err != nil {
		return "", fmt.Errorf("list conversations: %w", err)
	}
	defer func() { _ = rows.Close() }()

	for rows.Next() {
		var convID string
		if err := rows.Scan(&convID); err != nil {
			return "", fmt.Errorf("scan conversation id: %w", err)
		}
		messages, err := repo.LoadRepairMessages(ctx, convID)
		if err != nil {
			return "", fmt.Errorf("load messages for %s: %w", convID, err)
		}
		candidate, ok := metadata.DetectReplayChain(convID, messages, metadata.RepairBatchGap())
		if ok && candidate.CanonicalHash == hash {
			return convID, nil
		}
	}
	if err := rows.Err(); err != nil {
		return "", fmt.Errorf("iterate conversations: %w", err)
	}
	return "", fmt.Errorf("no conversation found with canonical hash %s", hash)
}

func printUsage() {
	fmt.Fprintln(os.Stderr, `Usage: conversation-repair <command> [flags]

Commands:
  detect  --dry-run                       Scan all conversations for replay chains
  report  --conversation-id <id>          Report on one conversation
  archive --run-id <id>                   Show archive status for a run
  apply   --run-id <id>                   Apply soft-delete (requires prior detect)
  restore --run-id <id>                   Clear soft-delete markers
  purge   --run-id <id> --confirm-purge <id>  Physically delete archived rows

Environment:
  BI_METADATA_DB_DSN   PostgreSQL connection string`)
}

// writeJSON emits one newline-terminated JSON report to the output stream.
func writeJSON(out io.Writer, v any) error {
	data, err := sonic.Marshal(v)
	if err != nil {
		return fmt.Errorf("encode report: %w", err)
	}
	data = append(data, '\n')
	_, err = out.Write(data)
	return err
}
