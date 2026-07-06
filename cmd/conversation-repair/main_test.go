package main

import (
	"bytes"
	"context"
	"database/sql"
	"database/sql/driver"
	"fmt"
	"io"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/biqly/biqly/internal/metadata"
	"github.com/bytedance/sonic"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- Minimal mock SQL driver (mirrors internal/metadata's test driver) ---

type queryMock struct {
	Pattern string // substring match on the normalized query
	Cols    []string
	Rows    [][]driver.Value
	Err     error
}

type mockState struct {
	mu      sync.Mutex
	calls   []string
	queries []queryMock
}

func normalize(q string) string {
	return strings.Join(strings.Fields(q), " ")
}

func (s *mockState) logCall(op string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.calls = append(s.calls, normalize(op))
}

func (s *mockState) callsMatching(substr string) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	n := 0
	for _, c := range s.calls {
		if strings.Contains(strings.ToLower(c), strings.ToLower(substr)) {
			n++
		}
	}
	return n
}

func (s *mockState) rowsFor(query string) (driver.Rows, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	q := strings.ToLower(normalize(query))
	for _, qm := range s.queries {
		if strings.Contains(q, strings.ToLower(qm.Pattern)) {
			if qm.Err != nil {
				return nil, qm.Err
			}
			rows := make([][]driver.Value, len(qm.Rows))
			copy(rows, qm.Rows)
			return &mockRows{cols: qm.Cols, rows: rows}, nil
		}
	}
	return nil, fmt.Errorf("no mock query matched: %s", query)
}

type mockDriver struct {
	mu     sync.Mutex
	states map[string]*mockState
}

func (d *mockDriver) Open(name string) (driver.Conn, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	state, ok := d.states[name]
	if !ok {
		return nil, fmt.Errorf("no mock state registered for %s", name)
	}
	return &mockConn{state: state}, nil
}

var repairMockDriver = &mockDriver{states: make(map[string]*mockState)}

func init() {
	sql.Register("repair_mock", repairMockDriver)
}

func setupMockDB(t *testing.T) (*sql.DB, *mockState) {
	t.Helper()
	name := fmt.Sprintf("repair-%s-%d", t.Name(), time.Now().UnixNano())
	state := &mockState{}
	repairMockDriver.mu.Lock()
	repairMockDriver.states[name] = state
	repairMockDriver.mu.Unlock()

	db, err := sql.Open("repair_mock", name)
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = db.Close()
		repairMockDriver.mu.Lock()
		delete(repairMockDriver.states, name)
		repairMockDriver.mu.Unlock()
	})
	return db, state
}

type mockConn struct {
	state *mockState
}

func (c *mockConn) Prepare(query string) (driver.Stmt, error) {
	return &mockStmt{conn: c, query: query}, nil
}

func (*mockConn) Close() error { return nil }

func (c *mockConn) Begin() (driver.Tx, error) {
	c.state.logCall("BEGIN")
	return &mockTx{conn: c}, nil
}

//nolint:unparam // signature fixed by driver.ConnBeginTx
func (c *mockConn) BeginTx(context.Context, driver.TxOptions) (driver.Tx, error) {
	c.state.logCall("BEGIN TX")
	return &mockTx{conn: c}, nil
}

func (c *mockConn) QueryContext(_ context.Context, query string, _ []driver.NamedValue) (driver.Rows, error) {
	c.state.logCall("QUERY: " + query)
	return c.state.rowsFor(query)
}

//nolint:unparam // signature fixed by driver.ExecerContext
func (c *mockConn) ExecContext(_ context.Context, query string, _ []driver.NamedValue) (driver.Result, error) {
	c.state.logCall("EXEC: " + query)
	return driver.RowsAffected(1), nil
}

type mockStmt struct {
	conn  *mockConn
	query string
}

func (*mockStmt) Close() error  { return nil }
func (*mockStmt) NumInput() int { return -1 }

func (s *mockStmt) Exec([]driver.Value) (driver.Result, error) {
	s.conn.state.logCall("EXEC: " + s.query)
	return driver.RowsAffected(1), nil
}

func (s *mockStmt) Query([]driver.Value) (driver.Rows, error) {
	s.conn.state.logCall("QUERY: " + s.query)
	return s.conn.state.rowsFor(s.query)
}

type mockTx struct {
	conn *mockConn
}

func (tx *mockTx) Commit() error {
	tx.conn.state.logCall("COMMIT")
	return nil
}

func (tx *mockTx) Rollback() error {
	tx.conn.state.logCall("ROLLBACK")
	return nil
}

type mockRows struct {
	cols []string
	rows [][]driver.Value
	pos  int
}

func (r *mockRows) Columns() []string { return r.cols }
func (*mockRows) Close() error        { return nil }

func (r *mockRows) Next(dest []driver.Value) error {
	if r.pos >= len(r.rows) {
		return io.EOF
	}
	row := r.rows[r.pos]
	for i := range dest {
		if i < len(row) {
			dest[i] = row[i]
		} else {
			dest[i] = nil
		}
	}
	r.pos++
	return nil
}

// --- Fixtures ---

var repairBase = time.Date(2026, 7, 6, 10, 0, 0, 0, time.UTC)

// replayChainRows builds message rows forming an ordered-prefix replay chain:
// batch 1 [m1, m2] replayed inside batch 2 [m3, m4, m5].
// Columns match LoadRepairMessages: id, role, content, ai_response, result_summary, created_at.
func replayChainRows() [][]driver.Value {
	return [][]driver.Value{
		{"m1", "user", "hello", "", "", repairBase},
		{"m2", "assistant", "hi", "", "", repairBase.Add(10 * time.Millisecond)},
		{"m3", "user", "hello", "", "", repairBase.Add(500 * time.Millisecond)},
		{"m4", "assistant", "hi", "", "", repairBase.Add(510 * time.Millisecond)},
		{"m5", "user", "more", "", "", repairBase.Add(520 * time.Millisecond)},
	}
}

// replayChainHash computes the canonical hash the detector derives for the fixture.
func replayChainHash(t *testing.T) string {
	t.Helper()
	messages := []metadata.RepairMessage{
		{ID: "m1", Role: "user", Content: "hello", CreatedAt: repairBase},
		{ID: "m2", Role: "assistant", Content: "hi", CreatedAt: repairBase.Add(10 * time.Millisecond)},
		{ID: "m3", Role: "user", Content: "hello", CreatedAt: repairBase.Add(500 * time.Millisecond)},
		{ID: "m4", Role: "assistant", Content: "hi", CreatedAt: repairBase.Add(510 * time.Millisecond)},
		{ID: "m5", Role: "user", Content: "more", CreatedAt: repairBase.Add(520 * time.Millisecond)},
	}
	candidate, ok := metadata.DetectReplayChain("conv-1", messages, metadata.RepairBatchGap())
	require.True(t, ok)
	return candidate.CanonicalHash
}

const loadMessagesPattern = "ORDER BY created_at ASC, id ASC"

// --- Command tests ---

func TestConversationRepairDetectDryRunIsReadOnly(t *testing.T) {
	db, state := setupMockDB(t)
	repo := metadata.NewRepository(db)

	state.queries = []queryMock{
		{Pattern: "DISTINCT conversation_id", Cols: []string{"conversation_id"}, Rows: [][]driver.Value{{"conv-1"}}},
		{Pattern: loadMessagesPattern, Cols: []string{"id", "role", "content", "ai_response", "result_summary", "created_at"}, Rows: replayChainRows()},
	}

	var out bytes.Buffer
	require.NoError(t, runDetect(context.Background(), repo, true, &out))

	var report struct {
		DetectMode   string `json:"detect_mode"`
		Candidates   int    `json:"candidates"`
		TotalScanned int    `json:"total_scanned"`
	}
	require.NoError(t, sonic.Unmarshal(out.Bytes(), &report))
	assert.Equal(t, "dry-run", report.DetectMode)
	assert.Equal(t, 1, report.Candidates)
	assert.Equal(t, 1, report.TotalScanned)

	// Dry-run must not write anything.
	assert.Zero(t, state.callsMatching("INSERT INTO conversation_repair_runs"))
	assert.Zero(t, state.callsMatching("EXEC:"))
}

func TestConversationRepairDetectPersistsRun(t *testing.T) {
	db, state := setupMockDB(t)
	repo := metadata.NewRepository(db)

	state.queries = []queryMock{
		{Pattern: "DISTINCT conversation_id", Cols: []string{"conversation_id"}, Rows: [][]driver.Value{{"conv-1"}}},
		{Pattern: loadMessagesPattern, Cols: []string{"id", "role", "content", "ai_response", "result_summary", "created_at"}, Rows: replayChainRows()},
		{Pattern: "INSERT INTO conversation_repair_runs", Cols: []string{"id"}, Rows: [][]driver.Value{{"run-1"}}},
	}

	var out bytes.Buffer
	require.NoError(t, runDetect(context.Background(), repo, false, &out))

	var report struct {
		DetectMode string `json:"detect_mode"`
		Candidates int    `json:"candidates"`
	}
	require.NoError(t, sonic.Unmarshal(out.Bytes(), &report))
	assert.Equal(t, "detect", report.DetectMode)
	assert.Equal(t, 1, report.Candidates)
	assert.Equal(t, 1, state.callsMatching("INSERT INTO conversation_repair_runs"))
}

func TestConversationRepairReportSingleConversation(t *testing.T) {
	db, state := setupMockDB(t)
	repo := metadata.NewRepository(db)

	state.queries = []queryMock{
		{Pattern: loadMessagesPattern, Cols: []string{"id", "role", "content", "ai_response", "result_summary", "created_at"}, Rows: replayChainRows()},
	}

	var out bytes.Buffer
	require.NoError(t, runReport(context.Background(), repo, "conv-1", &out))

	var report struct {
		ConversationID string                    `json:"conversation_id"`
		MessageCount   int                       `json:"message_count"`
		HasChain       bool                      `json:"has_chain"`
		Candidate      *metadata.RepairCandidate `json:"candidate"`
	}
	require.NoError(t, sonic.Unmarshal(out.Bytes(), &report))
	assert.Equal(t, "conv-1", report.ConversationID)
	assert.Equal(t, 5, report.MessageCount)
	assert.True(t, report.HasChain)
	require.NotNil(t, report.Candidate)
	assert.ElementsMatch(t, []string{"m1", "m2"}, report.Candidate.ReplayIDs)
	assert.ElementsMatch(t, []string{"m3", "m4", "m5"}, report.Candidate.KeepIDs)
}

func TestConversationRepairReportRequiresConversationID(t *testing.T) {
	db, _ := setupMockDB(t)
	repo := metadata.NewRepository(db)

	err := runReport(context.Background(), repo, "", &bytes.Buffer{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "--conversation-id is required")
}

func TestConversationRepairApplyArchivesAndSoftDeletes(t *testing.T) {
	db, state := setupMockDB(t)
	repo := metadata.NewRepository(db)
	hash := replayChainHash(t)

	state.queries = []queryMock{
		// ApplyRepairRun's re-verify lock — must come before the broader run lookup pattern.
		{Pattern: "FOR UPDATE", Cols: []string{"canonical_hash"}, Rows: [][]driver.Value{{hash}}},
		// GetRepairRun: a pending detect run with the fixture's canonical hash.
		{
			Pattern: "FROM conversation_repair_runs",
			Cols:    []string{"id", "mode", "status", "candidate_count", "repaired_count", "skipped_count", "canonical_hash", "created_at", "completed_at"},
			Rows:    [][]driver.Value{{"run-1", "detect", "pending", 2, 0, 0, hash, repairBase, nil}},
		},
		// findConversationByHash scan.
		{Pattern: "DISTINCT conversation_id", Cols: []string{"conversation_id"}, Rows: [][]driver.Value{{"conv-1"}}},
		{Pattern: loadMessagesPattern, Cols: []string{"id", "role", "content", "ai_response", "result_summary", "created_at"}, Rows: replayChainRows()},
		// Full-row load for the archive step.
		{
			Pattern: "to_jsonb",
			Cols:    []string{"id", "conversation_id", "remote_id", "role", "content", "created_at", "ordinal", "full_row_json"},
			Rows:    [][]driver.Value{{"m1", "conv-1", "", "user", "hello", repairBase, int64(0), []byte(`{}`)}},
		},
	}

	var out bytes.Buffer
	require.NoError(t, runApply(context.Background(), repo, "run-1", &out))

	var report struct {
		RunID        string `json:"run_id"`
		Conversation string `json:"conversation"`
		Applied      int    `json:"applied"`
		Kept         int    `json:"kept"`
		Canonical    string `json:"canonical"`
	}
	require.NoError(t, sonic.Unmarshal(out.Bytes(), &report))
	assert.Equal(t, "run-1", report.RunID)
	assert.Equal(t, "conv-1", report.Conversation)
	assert.Equal(t, 2, report.Applied)
	assert.Equal(t, 3, report.Kept)
	assert.Equal(t, hash, report.Canonical)

	// Every replay message is archived with its full row, then soft-deleted.
	assert.Equal(t, 2, state.callsMatching("INSERT INTO conversation_message_repair_archive"))
	assert.Equal(t, 2, state.callsMatching("SET deleted_at = now()"))
	// Apply must never hard-delete.
	assert.Zero(t, state.callsMatching("DELETE FROM ai_conversation_messages"))
	assert.Equal(t, 1, state.callsMatching("COMMIT"))
}

func TestConversationRepairApplyRejectsNonPendingRun(t *testing.T) {
	db, state := setupMockDB(t)
	repo := metadata.NewRepository(db)

	state.queries = []queryMock{
		{
			Pattern: "FROM conversation_repair_runs",
			Cols:    []string{"id", "mode", "status", "candidate_count", "repaired_count", "skipped_count", "canonical_hash", "created_at", "completed_at"},
			Rows:    [][]driver.Value{{"run-1", "detect", "completed", 2, 2, 0, "some-hash", repairBase, repairBase}},
		},
	}

	err := runApply(context.Background(), repo, "run-1", &bytes.Buffer{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not pending")
	assert.Zero(t, state.callsMatching("SET deleted_at"))
}

func TestConversationRepairApplyRejectsChangedHash(t *testing.T) {
	db, state := setupMockDB(t)
	repo := metadata.NewRepository(db)
	hash := replayChainHash(t)

	// The stored run references a hash that no current conversation produces.
	state.queries = []queryMock{
		{
			Pattern: "FROM conversation_repair_runs",
			Cols:    []string{"id", "mode", "status", "candidate_count", "repaired_count", "skipped_count", "canonical_hash", "created_at", "completed_at"},
			Rows:    [][]driver.Value{{"run-1", "detect", "pending", 2, 0, 0, "stale-" + hash, repairBase, nil}},
		},
		{Pattern: "DISTINCT conversation_id", Cols: []string{"conversation_id"}, Rows: [][]driver.Value{{"conv-1"}}},
		{Pattern: loadMessagesPattern, Cols: []string{"id", "role", "content", "ai_response", "result_summary", "created_at"}, Rows: replayChainRows()},
	}

	err := runApply(context.Background(), repo, "run-1", &bytes.Buffer{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "find conversation by hash")
	assert.Zero(t, state.callsMatching("SET deleted_at"))
}

func TestConversationRepairRestoreClearsSoftDeletes(t *testing.T) {
	db, state := setupMockDB(t)
	repo := metadata.NewRepository(db)

	var out bytes.Buffer
	require.NoError(t, runRestore(context.Background(), repo, "run-1", &out))

	assert.Equal(t, 1, state.callsMatching("SET deleted_at = NULL"))
	assert.Equal(t, 1, state.callsMatching("SET status = 'pending'"))
	assert.Zero(t, state.callsMatching("DELETE FROM"))
}

func TestConversationRepairPurgeHardDeletesOnlySoftDeleted(t *testing.T) {
	db, state := setupMockDB(t)
	repo := metadata.NewRepository(db)

	var out bytes.Buffer
	require.NoError(t, runPurge(context.Background(), repo, "run-1", &out))

	// Purge deletes only rows already soft-deleted by this run, then marks the run.
	assert.Equal(t, 1, state.callsMatching("DELETE FROM ai_conversation_messages WHERE deleted_by_repair_run_id"))
	assert.Equal(t, 1, state.callsMatching("deleted_at IS NOT NULL"))
	assert.Equal(t, 1, state.callsMatching("SET mode = 'purge'"))
	assert.Equal(t, 1, state.callsMatching("COMMIT"))
	// Archive rows are retained for audit.
	assert.Zero(t, state.callsMatching("DELETE FROM conversation_message_repair_archive"))
}

func TestConversationRepairMutationsRequireRunID(t *testing.T) {
	db, _ := setupMockDB(t)
	repo := metadata.NewRepository(db)
	ctx := context.Background()

	for name, run := range map[string]func() error{
		"archive": func() error { return runArchive(ctx, repo, "", &bytes.Buffer{}) },
		"apply":   func() error { return runApply(ctx, repo, "", &bytes.Buffer{}) },
		"restore": func() error { return runRestore(ctx, repo, "", &bytes.Buffer{}) },
		"purge":   func() error { return runPurge(ctx, repo, "", &bytes.Buffer{}) },
	} {
		t.Run(name, func(t *testing.T) {
			err := run()
			require.Error(t, err)
			assert.Contains(t, err.Error(), "--run-id is required")
		})
	}
}
