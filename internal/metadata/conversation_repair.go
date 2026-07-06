package metadata

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/bytedance/sonic"
)

// Production batch gap: legacy rows lack request/remote IDs, so we group
// rapid sequential inserts into candidate POST batches using a fixed boundary.
const repairBatchGap = 250 * time.Millisecond

// RepairBatchGap returns the production batch gap for replay-chain detection.
func RepairBatchGap() time.Duration {
	return repairBatchGap
}

// RepairMessage is a flat message representation for the replay-chain detector.
type RepairMessage struct {
	ID         string
	Role       string
	Content    string
	Response   json.RawMessage
	Summary    string
	CreatedAt  time.Time
	Provenance string
}

// RepairCandidate describes a detected replay chain and what should be kept vs. removed.
type RepairCandidate struct {
	ConversationID string   `json:"conversation_id"`
	CanonicalHash  string   `json:"canonical_hash"`
	KeepIDs        []string `json:"keep_ids"`
	ReplayIDs      []string `json:"replay_ids"`
	Reason         string   `json:"reason"`
}

// ErrRepairAmbiguous is returned when the message history cannot be unambiguously
// attributed to a replay chain.
var ErrRepairAmbiguous = errors.New("ambiguous replay chain")

// DetectReplayChain groups messages into batches by a time gap, verifies that
// each earlier batch is an exact ordered prefix of the next, and identifies the
// final (longest) batch as the canonical snapshot. Earlier proven-prefix batches
// are replay copies.
//
// Returns (candidate, true) when an unambiguous chain is detected, or
// (_, false) when the history is ambiguous or not a replay chain.
func DetectReplayChain(conversationID string, messages []RepairMessage, batchGap time.Duration) (RepairCandidate, bool) {
	if len(messages) < 2 {
		return RepairCandidate{ConversationID: conversationID}, false
	}

	batches := groupIntoBatches(messages, batchGap)
	if len(batches) < 2 {
		return RepairCandidate{ConversationID: conversationID}, false
	}

	// Verify ordered-prefix chain: each batch must be an exact prefix of the next.
	for i := 1; i < len(batches); i++ {
		if !isOrderedPrefix(batches[i-1], batches[i]) {
			return RepairCandidate{ConversationID: conversationID}, false
		}
	}

	// The final batch must be the unique longest.
	finalBatch := batches[len(batches)-1]
	for i := 0; i < len(batches)-1; i++ {
		if len(batches[i]) >= len(finalBatch) {
			// A prior batch is equal or longer — not a unique longest final batch.
			if len(batches[i]) == len(finalBatch) {
				// Equal length: check if content differs (ambiguous) or is identical (redundant prefix)
				if !messagesEqual(batches[i], finalBatch) {
					return RepairCandidate{ConversationID: conversationID}, false
				}
			} else {
				return RepairCandidate{ConversationID: conversationID}, false
			}
		}
	}

	// Collect keep and replay IDs.
	keepIDs := make([]string, 0, len(finalBatch))
	for _, m := range finalBatch {
		keepIDs = append(keepIDs, m.ID)
	}

	replayIDs := make([]string, 0)
	for i := 0; i < len(batches)-1; i++ {
		for _, m := range batches[i] {
			replayIDs = append(replayIDs, m.ID)
		}
	}

	if len(replayIDs) == 0 {
		return RepairCandidate{ConversationID: conversationID}, false
	}

	canonicalHash := computeCanonicalHash(finalBatch)

	return RepairCandidate{
		ConversationID: conversationID,
		CanonicalHash:  canonicalHash,
		KeepIDs:        keepIDs,
		ReplayIDs:      replayIDs,
		Reason:         "ordered-prefix replay chain detected",
	}, true
}

// groupIntoBatches splits messages into batches where consecutive messages
// within batchGap of each other belong to the same batch.
func groupIntoBatches(messages []RepairMessage, batchGap time.Duration) [][]RepairMessage {
	if len(messages) == 0 {
		return nil
	}
	// Sort by created_at, then by ID for stable ordering.
	sorted := make([]RepairMessage, len(messages))
	copy(sorted, messages)
	sortRepairMessages(sorted)

	batches := [][]RepairMessage{{sorted[0]}}
	for i := 1; i < len(sorted); i++ {
		prev := sorted[i-1]
		curr := sorted[i]
		if curr.CreatedAt.Sub(prev.CreatedAt) > batchGap {
			batches = append(batches, []RepairMessage{curr})
		} else {
			batches[len(batches)-1] = append(batches[len(batches)-1], curr)
		}
	}
	return batches
}

// isOrderedPrefix checks that `shorter` is an exact ordered prefix of `longer`
// — same messages in the same positions (by role, content, response, summary, provenance).
func isOrderedPrefix(shorter, longer []RepairMessage) bool {
	if len(shorter) > len(longer) {
		return false
	}
	for i := range shorter {
		if !repairMessageEqual(shorter[i], longer[i]) {
			return false
		}
	}
	return true
}

func messagesEqual(a, b []RepairMessage) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if !repairMessageEqual(a[i], b[i]) {
			return false
		}
	}
	return true
}

func repairMessageEqual(a, b RepairMessage) bool {
	return a.Role == b.Role &&
		a.Content == b.Content &&
		a.Summary == b.Summary &&
		a.Provenance == b.Provenance &&
		jsonBytesEqual(a.Response, b.Response)
}

func jsonBytesEqual(a, b json.RawMessage) bool {
	if len(a) == 0 && len(b) == 0 {
		return true
	}
	if len(a) == 0 || len(b) == 0 {
		return false
	}
	// Canonicalize before comparison for key-order independence.
	ca := canonicalJSON(a)
	cb := canonicalJSON(b)
	return hex.EncodeToString(ca) == hex.EncodeToString(cb)
}

// canonicalJSONAPI sorts map keys so equivalent payloads hash identically
// regardless of original key order.
var canonicalJSONAPI = sonic.Config{SortMapKeys: true}.Froze()

func canonicalJSON(raw json.RawMessage) []byte {
	var v any
	if err := sonic.Unmarshal(raw, &v); err != nil {
		return raw
	}
	out, err := canonicalJSONAPI.Marshal(v)
	if err != nil {
		return raw
	}
	return out
}

func computeCanonicalHash(messages []RepairMessage) string {
	h := sha256.New()
	for _, m := range messages {
		h.Write([]byte(m.Role))
		h.Write([]byte(m.Content))
		h.Write([]byte(m.Summary))
		h.Write([]byte(m.Provenance))
		h.Write(canonicalJSON(m.Response))
	}
	return hex.EncodeToString(h.Sum(nil))
}

func sortRepairMessages(msgs []RepairMessage) {
	// Simple insertion sort by CreatedAt, then ID — stable for small lists.
	for i := 1; i < len(msgs); i++ {
		for j := i; j > 0; j-- {
			if msgs[j].CreatedAt.Before(msgs[j-1].CreatedAt) ||
				(msgs[j].CreatedAt.Equal(msgs[j-1].CreatedAt) && msgs[j].ID < msgs[j-1].ID) {
				msgs[j], msgs[j-1] = msgs[j-1], msgs[j]
			} else {
				break
			}
		}
	}
}

// RepairRunRow is one persisted repair run.
type RepairRunRow struct {
	ID             string
	Mode           string
	Status         string
	CandidateCount int
	RepairedCount  int
	SkippedCount   int
	CanonicalHash  string
	CreatedAt      time.Time
	CompletedAt    sql.NullTime
}

// CreateRepairRun inserts a new repair run and returns its id.
func (r *Repository) CreateRepairRun(ctx context.Context, mode, canonicalHash string, candidateCount int) (string, error) {
	var id string
	err := r.db.QueryRowContext(ctx, `
		INSERT INTO conversation_repair_runs (mode, status, candidate_count, canonical_hash)
		VALUES ($1, 'pending', $2, $3)
		RETURNING id::text
	`, mode, candidateCount, canonicalHash).Scan(&id)
	if err != nil {
		return "", fmt.Errorf("create repair run: %w", err)
	}
	return id, nil
}

// CompleteRepairRun marks a repair run as completed and records counts.
func (r *Repository) CompleteRepairRun(ctx context.Context, runID string, repairedCount, skippedCount int) error {
	_, err := r.db.ExecContext(ctx, `
		UPDATE conversation_repair_runs
		SET status = 'completed', repaired_count = $2, skipped_count = $3, completed_at = now()
		WHERE id = $1
	`, runID, repairedCount, skippedCount)
	if err != nil {
		return fmt.Errorf("complete repair run: %w", err)
	}
	return nil
}

// GetRepairRun returns one repair run by id.
func (r *Repository) GetRepairRun(ctx context.Context, runID string) (*RepairRunRow, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT id::text, mode, status, candidate_count, repaired_count, skipped_count,
		       canonical_hash, created_at, completed_at
		FROM conversation_repair_runs
		WHERE id = $1
	`, runID)
	var rr RepairRunRow
	if err := row.Scan(&rr.ID, &rr.Mode, &rr.Status, &rr.CandidateCount, &rr.RepairedCount,
		&rr.SkippedCount, &rr.CanonicalHash, &rr.CreatedAt, &rr.CompletedAt); err != nil {
		return nil, fmt.Errorf("get repair run: %w", err)
	}
	return &rr, nil
}

// LoadRepairMessages fetches all non-deleted messages for a conversation,
// ordered by created_at and id, for replay-chain analysis.
func (r *Repository) LoadRepairMessages(ctx context.Context, conversationID string) ([]RepairMessage, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id::text, role, content, COALESCE(ai_response::text, ''),
		       COALESCE(result_summary, ''), created_at
		FROM ai_conversation_messages
		WHERE conversation_id = $1 AND deleted_at IS NULL
		ORDER BY created_at ASC, id ASC
	`, conversationID)
	if err != nil {
		return nil, fmt.Errorf("load repair messages: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var messages []RepairMessage
	for rows.Next() {
		var m RepairMessage
		var responseText string
		if err := rows.Scan(&m.ID, &m.Role, &m.Content, &responseText, &m.Summary, &m.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan repair message: %w", err)
		}
		if responseText != "" {
			m.Response = json.RawMessage(responseText)
		}
		messages = append(messages, m)
	}
	return messages, rows.Err()
}

// ApplyRepairRun archives the replay messages and soft-deletes them in one transaction.
// It locks the conversation, re-verifies the canonical hash, and never hard-deletes.
func (r *Repository) ApplyRepairRun(ctx context.Context, runID string, candidate RepairCandidate) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin repair tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	// Lock and re-verify canonical hash under FOR UPDATE.
	var currentHash string
	err = tx.QueryRowContext(ctx, `
		SELECT canonical_hash FROM conversation_repair_runs
		WHERE id = $1 AND status = 'pending'
		FOR UPDATE
	`, runID).Scan(&currentHash)
	if err != nil {
		return fmt.Errorf("lock repair run: %w", err)
	}
	if currentHash != candidate.CanonicalHash {
		return fmt.Errorf("canonical hash mismatch: expected %s, got %s", currentHash, candidate.CanonicalHash)
	}

	// Archive each replay message, then soft-delete.
	for _, msgID := range candidate.ReplayIDs {
		var id, role, content, convID string
		var createdAt time.Time
		var remoteID sql.NullString
		var ordinal sql.NullInt64
		var fullRowJSON []byte

		err = tx.QueryRowContext(ctx, `
			SELECT id::text, conversation_id, COALESCE(remote_id, ''), role, content,
			       created_at, COALESCE(ordinal, 0), to_jsonb(m)
			FROM ai_conversation_messages m
			WHERE id = $1
		`, msgID).Scan(&id, &convID, &remoteID, &role, &content, &createdAt, &ordinal, &fullRowJSON)
		if err != nil {
			return fmt.Errorf("load message %s for archive: %w", msgID, err)
		}

		contentHash := computeCanonicalHash([]RepairMessage{{Role: role, Content: content}})
		_, err = tx.ExecContext(ctx, `
			INSERT INTO conversation_message_repair_archive
			    (repair_run_id, original_message_id, conversation_id, remote_id, role,
			     content, content_hash, ordinal, created_at, full_row_json)
			VALUES ($1, $2, $3, NULLIF($4, ''), $5, $6, $7, $8, $9, $10)
			ON CONFLICT (repair_run_id, original_message_id) DO NOTHING
		`, runID, id, convID, remoteID.String, role, content, contentHash, ordinal, createdAt, fullRowJSON)
		if err != nil {
			return fmt.Errorf("archive message %s: %w", msgID, err)
		}

		_, err = tx.ExecContext(ctx, `
			UPDATE ai_conversation_messages
			SET deleted_at = now(), deleted_by_repair_run_id = $2::uuid
			WHERE id = $1 AND deleted_at IS NULL
		`, msgID, runID)
		if err != nil {
			return fmt.Errorf("soft-delete message %s: %w", msgID, err)
		}
	}

	if _, err := tx.ExecContext(ctx, `
		UPDATE conversation_repair_runs
		SET status = 'completed', repaired_count = $2, completed_at = now()
		WHERE id = $1
	`, runID, len(candidate.ReplayIDs)); err != nil {
		return fmt.Errorf("complete repair run: %w", err)
	}

	return tx.Commit()
}

// RestoreRepairRun clears the soft-delete markers for messages archived by a run.
func (r *Repository) RestoreRepairRun(ctx context.Context, runID string) error {
	_, err := r.db.ExecContext(ctx, `
		UPDATE ai_conversation_messages
		SET deleted_at = NULL, deleted_by_repair_run_id = NULL
		WHERE deleted_by_repair_run_id = $1::uuid
	`, runID)
	if err != nil {
		return fmt.Errorf("restore repair run: %w", err)
	}
	_, err = r.db.ExecContext(ctx, `
		UPDATE conversation_repair_runs
		SET status = 'pending', repaired_count = 0, completed_at = NULL
		WHERE id = $1
	`, runID)
	if err != nil {
		return fmt.Errorf("reset repair run status: %w", err)
	}
	return nil
}

// PurgeRepairRun physically deletes the archived messages for a completed run.
// This is a separate, explicitly invoked operation after observation.
func (r *Repository) PurgeRepairRun(ctx context.Context, runID string) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin purge tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	// Hard-delete soft-deleted messages attributed to this run.
	if _, err := tx.ExecContext(ctx, `
		DELETE FROM ai_conversation_messages
		WHERE deleted_by_repair_run_id = $1::uuid AND deleted_at IS NOT NULL
	`, runID); err != nil {
		return fmt.Errorf("purge messages: %w", err)
	}

	// Keep the archive rows for audit trail; just mark run as purged.
	if _, err := tx.ExecContext(ctx, `
		UPDATE conversation_repair_runs
		SET mode = 'purge', status = 'completed', completed_at = now()
		WHERE id = $1
	`, runID); err != nil {
		return fmt.Errorf("mark purge complete: %w", err)
	}

	return tx.Commit()
}
