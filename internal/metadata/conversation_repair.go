package metadata

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"time"
)

// Production batch gap: legacy rows lack request/remote IDs, so we group
// rapid sequential inserts into candidate POST batches using a fixed boundary.
const repairBatchGap = 250 * time.Millisecond

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
	ConversationID string
	CanonicalHash  string
	KeepIDs        []string
	ReplayIDs      []string
	Reason         string
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

func canonicalJSON(raw json.RawMessage) []byte {
	var v any
	// Use encoding/json for canonicalization — sonic doesn't guarantee
	// stable key ordering for map[string]any, but encoding/json sorts keys.
	if err := json.Unmarshal(raw, &v); err != nil {
		return raw
	}
	out, err := json.Marshal(v)
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
