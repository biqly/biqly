package metadata

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func msg(id, role, content string, t time.Time) RepairMessage {
	return RepairMessage{ID: id, Role: role, Content: content, CreatedAt: t}
}

func TestDetectReplayChainValidPrefixChain(t *testing.T) {
	base := time.Date(2026, 7, 6, 10, 0, 0, 0, time.UTC)

	// Batch 1: [U1, A1] at t=0
	// Batch 2: [U1, A1, U2] at t=0.5s (includes U1,A1 as prefix + new U2)
	b1u1 := msg("m1", "user", "hello", base)
	b1a1 := msg("m2", "assistant", "hi there", base.Add(10*time.Millisecond))
	b2u1 := msg("m3", "user", "hello", base.Add(500*time.Millisecond))
	b2a1 := msg("m4", "assistant", "hi there", base.Add(510*time.Millisecond))
	b2u2 := msg("m5", "user", "how many?", base.Add(520*time.Millisecond))

	messages := []RepairMessage{b1u1, b1a1, b2u1, b2a1, b2u2}

	candidate, ok := DetectReplayChain("conv-1", messages, repairBatchGap)
	require.True(t, ok)
	assert.Equal(t, "conv-1", candidate.ConversationID)
	assert.NotEmpty(t, candidate.CanonicalHash)
	assert.ElementsMatch(t, []string{"m3", "m4", "m5"}, candidate.KeepIDs)
	assert.ElementsMatch(t, []string{"m1", "m2"}, candidate.ReplayIDs)
}

func TestDetectReplayChainAmbiguousDifferentContent(t *testing.T) {
	base := time.Date(2026, 7, 6, 10, 0, 0, 0, time.UTC)

	// Batch 1: [U1="hello"] at t=0
	// Batch 2: [U1="goodbye"] at t=0.5s — same ordinal, different content
	b1u1 := msg("m1", "user", "hello", base)
	b2u1 := msg("m2", "user", "goodbye", base.Add(500*time.Millisecond))

	messages := []RepairMessage{b1u1, b2u1}

	_, ok := DetectReplayChain("conv-1", messages, repairBatchGap)
	assert.False(t, ok, "different content at same ordinal should not be a valid replay chain")
}

func TestDetectReplayChainRejectsPartialFinalBatch(t *testing.T) {
	base := time.Date(2026, 7, 6, 10, 0, 0, 0, time.UTC)

	// Batch 1: [U1, A1, U2]
	// Batch 2: [U1, A1] — shorter than batch 1, so no unique longest final batch
	b1u1 := msg("m1", "user", "hello", base)
	b1a1 := msg("m2", "assistant", "hi", base.Add(10*time.Millisecond))
	b1u2 := msg("m3", "user", "second", base.Add(20*time.Millisecond))
	b2u1 := msg("m4", "user", "hello", base.Add(500*time.Millisecond))
	b2a1 := msg("m5", "assistant", "hi", base.Add(510*time.Millisecond))

	messages := []RepairMessage{b1u1, b1a1, b1u2, b2u1, b2a1}

	_, ok := DetectReplayChain("conv-1", messages, repairBatchGap)
	assert.False(t, ok, "a final batch shorter than a prior batch is not a valid replay chain")
}

func TestDetectReplayChainLegitimateRepeatedText(t *testing.T) {
	base := time.Date(2026, 7, 6, 10, 0, 0, 0, time.UTC)

	// Two batches, each with different content at the same ordinal — ambiguous.
	b1u1 := msg("m1", "user", "hello", base)
	b1a1 := msg("m2", "assistant", "response-A", base.Add(10*time.Millisecond))
	b2u1 := msg("m3", "user", "hello", base.Add(500*time.Millisecond))
	b2a1 := msg("m4", "assistant", "response-B", base.Add(510*time.Millisecond))

	// Different assistant content at same ordinal is NOT a prefix chain.
	messages := []RepairMessage{b1u1, b1a1, b2u1, b2a1}
	_, ok := DetectReplayChain("conv-1", messages, repairBatchGap)
	assert.False(t, ok)
}

func TestDetectReplayChainSingleBatchNotAChain(t *testing.T) {
	base := time.Date(2026, 7, 6, 10, 0, 0, 0, time.UTC)

	// All messages within one batch — not a replay chain.
	messages := []RepairMessage{
		msg("m1", "user", "hello", base),
		msg("m2", "assistant", "hi", base.Add(10*time.Millisecond)),
	}

	_, ok := DetectReplayChain("conv-1", messages, repairBatchGap)
	assert.False(t, ok)
}

func TestDetectReplayChainEmptyMessages(t *testing.T) {
	_, ok := DetectReplayChain("conv-1", nil, repairBatchGap)
	assert.False(t, ok)
}

func TestDetectReplayChainCanonicalHashStable(t *testing.T) {
	base := time.Date(2026, 7, 6, 10, 0, 0, 0, time.UTC)
	resp := json.RawMessage(`{"sql":"SELECT 1"}`)

	b1u1 := RepairMessage{ID: "m1", Role: "user", Content: "hello", CreatedAt: base}
	b2u1 := RepairMessage{ID: "m2", Role: "user", Content: "hello", CreatedAt: base.Add(500 * time.Millisecond)}
	b2a1 := RepairMessage{ID: "m3", Role: "assistant", Content: "hi", CreatedAt: base.Add(510 * time.Millisecond), Response: resp}

	c1, ok1 := DetectReplayChain("conv-1", []RepairMessage{b1u1, b2u1, b2a1}, repairBatchGap)
	require.True(t, ok1)

	// Reorder input — hash should be the same.
	c2, ok2 := DetectReplayChain("conv-1", []RepairMessage{b2a1, b2u1, b1u1}, repairBatchGap)
	require.True(t, ok2)

	assert.Equal(t, c1.CanonicalHash, c2.CanonicalHash)
}

func TestDetectReplayChainJSONKeyOrderIndependent(t *testing.T) {
	base := time.Date(2026, 7, 6, 10, 0, 0, 0, time.UTC)
	resp1 := json.RawMessage(`{"a":1,"b":2}`)
	resp2 := json.RawMessage(`{"b":2,"a":1}`)

	b1u1 := RepairMessage{ID: "m1", Role: "user", Content: "q", CreatedAt: base, Response: resp1}
	b2u1 := RepairMessage{ID: "m2", Role: "user", Content: "q", CreatedAt: base.Add(500 * time.Millisecond), Response: resp2}
	b2a1 := RepairMessage{ID: "m3", Role: "assistant", Content: "a", CreatedAt: base.Add(510 * time.Millisecond)}

	// resp1 and resp2 are semantically identical despite different key order.
	_, ok := DetectReplayChain("conv-1", []RepairMessage{b1u1, b2u1, b2a1}, repairBatchGap)
	assert.True(t, ok, "JSON with different key order but same content should be treated as equal")
}
