package metadata

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"encoding/json"
	"testing"
	"time"

	"github.com/biqly/biqly/internal/i18n"
	pkgmetadata "github.com/biqly/biqly/pkg/metadata"
	"github.com/stretchr/testify/assert"
)

// --- confirmed query memory helpers ---

func TestConfirmedQueriesAdminOrderClause(t *testing.T) {
	assert.Equal(t, "created_at DESC", confirmedQueriesAdminOrderClause("", ""))
	assert.Equal(t, "question ASC", confirmedQueriesAdminOrderClause("question", "asc"))
	assert.Equal(t, "is_active ASC", confirmedQueriesAdminOrderClause("status", "asc"))
	assert.Equal(t, "created_at DESC", confirmedQueriesAdminOrderClause("confirmed_at", "desc"))
	assert.Equal(t, "created_at DESC", confirmedQueriesAdminOrderClause("unknown", "desc"))
}

func TestUpsertSavedQueryExample(t *testing.T) {
	db, state := setupMockDB(t)
	repo := NewRepository(db)
	ctx := context.Background()

	state.execs = []execMock{
		{Pattern: "INSERT INTO ai_saved_queries", RowsAffected: 1},
	}

	err := repo.UpsertSavedQueryExample(ctx, ConfirmedQueryUpsert{
		DatasourceID:      "00000000-0000-0000-0000-000000000001",
		ModelID:           "00000000-0000-0000-0000-000000000002",
		UserID:            "user-1",
		QuestionHash:      "abc123",
		NLQuery:           "how many users?",
		SQLQuery:          "SELECT count(*) FROM users",
		SemanticModelHash: "model@v1",
		QuestionEmbedding: []float32{0.1, 0.2, 0.3},
	})
	assert.NoError(t, err)
}

func TestUpsertSavedQueryExample_EmptyModelID(t *testing.T) {
	db, state := setupMockDB(t)
	repo := NewRepository(db)
	ctx := context.Background()

	state.execs = []execMock{
		{Pattern: "INSERT INTO ai_saved_queries", RowsAffected: 1},
	}

	err := repo.UpsertSavedQueryExample(ctx, ConfirmedQueryUpsert{
		DatasourceID:      "00000000-0000-0000-0000-000000000001",
		QuestionHash:      "abc123",
		NLQuery:           "how many users?",
		SQLQuery:          "SELECT count(*) FROM users",
		SemanticModelHash: "model@v1",
	})
	assert.NoError(t, err)
}

func TestListActiveSavedQueryExamplesCoverage(t *testing.T) {
	db, state := setupMockDB(t)
	repo := NewRepository(db)
	ctx := context.Background()

	state.queries = []queryMock{
		{
			Pattern: "FROM ai_saved_queries",
			Cols:    []string{"id", "datasource_id", "model_id", "user_id", "question_hash", "nl_query", "sql_query", "semantic_model_hash", "question_embedding", "is_active"},
			Rows: [][]driver.Value{
				{"cq-1", "ds-1", "m-1", "u-1", "hash1", "total revenue?", "SELECT sum(revenue)", "model@1", []byte(`[0.1,0.2,0.3]`), true},
			},
		},
	}

	rows, err := repo.ListActiveSavedQueryExamples(ctx, "ds-1", "m-1", "model@1", 10)
	assert.NoError(t, err)
	assert.Len(t, rows, 1)
	assert.Equal(t, "total revenue?", rows[0].NLQuery)
	assert.Equal(t, []float32{0.1, 0.2, 0.3}, rows[0].QuestionEmbedding)
}

func TestListSavedQueryExamplesForAdmin(t *testing.T) {
	db, state := setupMockDB(t)
	repo := NewRepository(db)
	ctx := context.Background()
	now := time.Now()

	state.queries = []queryMock{
		{
			Pattern: "FROM ai_saved_queries WHERE datasource_id",
			Cols:    []string{"id", "datasource_id", "model_id", "user_id", "nl_query", "sql_query", "semantic_model_hash", "is_active", "confirmed_at"},
			Rows: [][]driver.Value{
				{"cq-1", "ds-1", "m-1", "u-1", "total revenue?", "SELECT sum(revenue)", "model@1", true, now},
			},
		},
	}

	rows, err := repo.ListSavedQueryExamplesForAdmin(ctx, ConfirmedQueriesAdminListParams{
		DatasourceID: "ds-1",
		Limit:        10,
		Offset:       0,
		SortBy:       "question",
		SortDir:      "asc",
	})
	assert.NoError(t, err)
	assert.Len(t, rows, 1)
	assert.Equal(t, "total revenue?", rows[0].NLQuery)
}

func TestCountSavedQueryExamplesForAdmin(t *testing.T) {
	db, state := setupMockDB(t)
	repo := NewRepository(db)
	ctx := context.Background()

	state.queries = []queryMock{
		{
			Pattern: "SELECT COUNT(*)::int FROM ai_saved_queries",
			Cols:    []string{"count"},
			Rows:    [][]driver.Value{{int64(5)}},
		},
	}

	count, err := repo.CountSavedQueryExamplesForAdmin(ctx, "ds-1")
	assert.NoError(t, err)
	assert.Equal(t, 5, count)
}

func TestSetSavedQueryExampleActive(t *testing.T) {
	db, state := setupMockDB(t)
	repo := NewRepository(db)
	ctx := context.Background()

	state.execs = []execMock{
		{Pattern: "UPDATE ai_saved_queries SET is_active", RowsAffected: 1},
	}

	n, err := repo.SetSavedQueryExampleActive(ctx, "cq-1", false)
	assert.NoError(t, err)
	assert.Equal(t, int64(1), n)
}

func TestDeactivateSavedQueryExamplesExceptHash(t *testing.T) {
	db, state := setupMockDB(t)
	repo := NewRepository(db)
	ctx := context.Background()

	state.execs = []execMock{
		{Pattern: "UPDATE ai_saved_queries", RowsAffected: 3},
	}

	n, err := repo.DeactivateSavedQueryExamplesExceptHash(ctx, "m-1", "new-hash")
	assert.NoError(t, err)
	assert.Equal(t, int64(3), n)
}

func TestGetLatestAIQueryHistoryForFeedback(t *testing.T) {
	db, state := setupMockDB(t)
	repo := NewRepository(db)
	ctx := context.Background()

	state.queries = []queryMock{
		{
			Pattern: "SELECT id::text, COALESCE(model_id::text, ''), logical_query",
			Cols:    []string{"id", "model_id", "logical_query"},
			Rows:    [][]driver.Value{{"aqh-1", "m-1", []byte(`{"version":"v1"}`)}},
		},
	}

	row, err := repo.GetLatestAIQueryHistoryForFeedback(ctx, "ds-1", "u-1", "how many users?")
	assert.NoError(t, err)
	assert.NotNil(t, row)
	assert.Equal(t, "aqh-1", row.ID)
	assert.Equal(t, []byte(`{"version":"v1"}`), row.LogicalQuery)
}

// --- ai_history_query.go ---

func TestListAIQueryHistoryFiltered(t *testing.T) {
	db, state := setupMockDB(t)
	repo := NewRepository(db)
	ctx := context.Background()
	now := time.Now()

	state.queries = []queryMock{
		{
			Pattern: "SELECT COUNT(*) FROM ai_query_history",
			Cols:    []string{"count"},
			Rows:    [][]driver.Value{{int64(1)}},
		},
		{
			Pattern: "SELECT id, datasource_id, model_id, user_id, question, prompt_context",
			Cols: []string{"id", "datasource_id", "model_id", "user_id", "question", "prompt_context",
				"ai_response", "logical_query", "confidence_score", "warnings", "outcome_status",
				"retry_count", "needs_clarification", "model_used", "prompt_tokens", "completion_tokens",
				"token_count", "cost_usd", "latency_ms", "created_at", "ab_experiment_id", "ab_variant_id",
				"memory_recall_used", "memory_recall_hit_count"},
			Rows: [][]driver.Value{
				{"aqh-1", "ds-1", "m-1", "u-1", "how many users?", []byte(`{}`), []byte(`{}`), []byte(`{"version":"v1"}`), 0.95, `{warn1}`, "success", int64(0), false, "gpt-4", int64(6), int64(4), int64(10), 0.05, int64(120), now, nil, nil, false, int64(0)},
			},
		},
	}

	result, err := repo.ListAIQueryHistoryFiltered(ctx, AIHistoryListFilter{
		UserID:       "u-1",
		DatasourceID: "ds-1",
		ModelID:      "m-1",
		Status:       "success",
		Search:       "users",
		Page:         1,
		PageSize:     10,
	})
	assert.NoError(t, err)
	assert.Equal(t, 1, result.Total)
	assert.Len(t, result.Entries, 1)
	assert.Equal(t, "aqh-1", result.Entries[0].ID)
}

func TestGetAIQueryHistoryByID(t *testing.T) {
	db, state := setupMockDB(t)
	repo := NewRepository(db)
	ctx := context.Background()
	now := time.Now()

	state.queries = []queryMock{
		{
			Pattern: "SELECT id, datasource_id, model_id, user_id, question, prompt_context",
			Cols: []string{"id", "datasource_id", "model_id", "user_id", "question", "prompt_context",
				"ai_response", "logical_query", "confidence_score", "warnings", "outcome_status",
				"retry_count", "needs_clarification", "model_used", "prompt_tokens", "completion_tokens",
				"token_count", "cost_usd", "latency_ms", "created_at", "ab_experiment_id", "ab_variant_id",
				"memory_recall_used", "memory_recall_hit_count"},
			Rows: [][]driver.Value{
				{"aqh-1", "ds-1", nil, "u-1", "question", []byte(`{}`), nil, nil, nil, `{}`, nil, nil, nil, nil, nil, nil, nil, nil, nil, now, nil, nil, false, int64(0)},
			},
		},
	}

	entry, err := repo.GetAIQueryHistoryByID(ctx, "aqh-1")
	assert.NoError(t, err)
	assert.NotNil(t, entry)
	assert.Equal(t, "aqh-1", entry.ID)
	assert.Nil(t, entry.ModelID)
}

// --- ai_jobs.go (additional coverage) ---

func TestListAIJobsAdmin(t *testing.T) {
	db, state := setupMockDB(t)
	repo := NewRepository(db)
	ctx := context.Background()
	now := time.Now()

	state.queries = []queryMock{
		{
			Pattern: "SELECT id, client_session_id, kind, status, phase, phase_message, progress_pct",
			Cols:    []string{"id", "client_session_id", "kind", "status", "phase", "phase_message", "progress_pct", "datasource_id", "scope_schemas", "progress_json", "request_json", "result_json", "error_message", "created_at", "updated_at", "started_at", "finished_at", "user_id", "locale"},
			Rows: [][]driver.Value{
				{"job-1", "sess-1", "describe", "running", "indexing", "indexing...", 50, "ds-1", "{schema1}", []byte(`{}`), []byte(`{}`), []byte(`{}`), "", now, now, now, nil, nil, nil},
			},
		},
	}

	jobs, err := repo.ListAIJobsAdmin(ctx, AIJobsAdminFilter{
		Status: "running",
		Limit:  10,
	})
	assert.NoError(t, err)
	assert.Len(t, jobs, 1)
}

func TestCountAIJobsAdmin(t *testing.T) {
	db, state := setupMockDB(t)
	repo := NewRepository(db)
	ctx := context.Background()

	state.queries = []queryMock{
		{
			Pattern: "SELECT COUNT(*) FROM ai_jobs WHERE",
			Cols:    []string{"count"},
			Rows:    [][]driver.Value{{int64(3)}},
		},
	}

	count, err := repo.CountAIJobsAdmin(ctx, AIJobsAdminFilter{Status: "running"})
	assert.NoError(t, err)
	assert.Equal(t, 3, count)
}

func TestFindConflictingEmbedMetadata(t *testing.T) {
	db, state := setupMockDB(t)
	repo := NewRepository(db)
	ctx := context.Background()
	now := time.Now()

	state.queries = []queryMock{
		{
			Pattern: "SELECT id, client_session_id, kind, status, phase, phase_message, progress_pct",
			Cols:    []string{"id", "client_session_id", "kind", "status", "phase", "phase_message", "progress_pct", "datasource_id", "scope_schemas", "progress_json", "request_json", "result_json", "error_message", "created_at", "updated_at", "started_at", "finished_at", "user_id", "locale"},
			Rows: [][]driver.Value{
				{"job-conflict", "sess-1", "embed_metadata", "running", "embedding", "embedding...", 50, "ds-1", "{schema1}", []byte(`{}`), []byte(`{}`), []byte(`{}`), "", now, now, now, nil, nil, nil},
			},
		},
	}

	job, err := repo.FindConflictingEmbedMetadata(ctx, "ds-1", "m-1")
	assert.NoError(t, err)
	assert.NotNil(t, job)
	assert.Equal(t, "job-conflict", job.ID)
}

func TestCancelAIJobsOwned(t *testing.T) {
	db, state := setupMockDB(t)
	repo := NewRepository(db)
	ctx := context.Background()

	state.execs = []execMock{
		{Pattern: "UPDATE ai_jobs SET status", RowsAffected: 2},
	}

	n, err := repo.CancelAIJobsOwned(ctx, []string{"job-1", "job-2"}, "user-1", "sess-1")
	assert.NoError(t, err)
	assert.Equal(t, 2, n)
}

// --- ai_nl_lexicon.go ---

func TestListNLLexicon(t *testing.T) {
	db, state := setupMockDB(t)
	repo := NewRepository(db)
	ctx := context.Background()
	now := time.Now()

	state.queries = []queryMock{
		{
			Pattern: "FROM ai_nl_lexicon",
			Cols:    []string{"locale", "domain", "key", "value", "is_active", "updated_at"},
			Rows: [][]driver.Value{
				{"en", "general", "greeting", []byte(`{"terms":["hello"]}`), true, now},
			},
		},
	}

	entries, err := repo.ListNLLexicon(ctx, "en", "general")
	assert.NoError(t, err)
	assert.Len(t, entries, 1)
	assert.Equal(t, "en", entries[0].Locale)
	assert.Equal(t, json.RawMessage(`{"terms":["hello"]}`), entries[0].Value)
}

func TestCountNLLexicon(t *testing.T) {
	db, state := setupMockDB(t)
	repo := NewRepository(db)
	ctx := context.Background()

	state.queries = []queryMock{
		{
			Pattern: "SELECT COUNT(*) FROM ai_nl_lexicon",
			Cols:    []string{"count"},
			Rows:    [][]driver.Value{{int64(10)}},
		},
	}

	count, err := repo.CountNLLexicon(ctx)
	assert.NoError(t, err)
	assert.Equal(t, 10, count)
}

// --- sft_export.go ---

func TestListFewShotSFTCandidates(t *testing.T) {
	db, state := setupMockDB(t)
	repo := NewRepository(db)
	ctx := context.Background()
	now := time.Now()

	state.queries = []queryMock{
		{
			Pattern: "FROM few_shot_examples",
			Cols:    []string{"id", "datasource_id", "model_id", "question", "logical_query", "tags", "dialect", "locale", "created_by", "created_at", "updated_at", "name", "description", "is_few_shot", "is_favorite"},
			Rows: [][]driver.Value{
				{"fe-1", "ds-1", "m-1", "how many users?", []byte(`{"version":"v1"}`), `{tag1}`, "postgres", "en", "admin", now, now, "example", "desc", true, true},
			},
		},
	}

	rows, err := repo.ListFewShotSFTCandidates(ctx)
	assert.NoError(t, err)
	assert.Len(t, rows, 1)
	assert.Equal(t, "few_shot", rows[0].Source)
}

func TestListPositiveAIHistorySFTCandidates(t *testing.T) {
	db, state := setupMockDB(t)
	repo := NewRepository(db)
	ctx := context.Background()

	state.queries = []queryMock{
		{
			Pattern: "SELECT question, logical_query, datasource_id::text, COALESCE(model_id::text, '')",
			Cols:    []string{"question", "logical_query", "datasource_id", "model_id"},
			Rows: [][]driver.Value{
				{"how many users?", []byte(`{"version":"v1"}`), "ds-1", "m-1"},
			},
		},
	}

	rows, err := repo.ListPositiveAIHistorySFTCandidates(ctx, 0.5)
	assert.NoError(t, err)
	assert.Len(t, rows, 1)
	assert.Equal(t, "history_positive", rows[0].Source)
}

// --- repository.go edge cases ---

func TestUpdateTableDisplayExpression(t *testing.T) {
	db, state := setupMockDB(t)
	repo := NewRepository(db)
	ctx := context.Background()

	state.execs = []execMock{
		{Pattern: "UPDATE tables SET display_expression", RowsAffected: 1},
	}

	expr := "name || ' (' || id || ')'"
	err := repo.UpdateTableDisplayExpression(ctx, "t-1", &expr)
	assert.NoError(t, err)

	// Test clearing the expression
	err = repo.UpdateTableDisplayExpression(ctx, "t-1", nil)
	assert.NoError(t, err)
}

// --- buildAIHistoryWhere edge cases ---

func TestBuildAIHistoryWhere_EmptyFilter(t *testing.T) {
	where, args := buildAIHistoryWhere(AIHistoryListFilter{})
	assert.Contains(t, where, "TRUE")
	assert.Empty(t, args)
}

func TestBuildAIHistoryWhere_StatusClarification(t *testing.T) {
	where, _ := buildAIHistoryWhere(AIHistoryListFilter{
		Status: "clarification",
	})
	assert.Contains(t, where, "needs_clarification = TRUE")
}

func TestBuildAIHistoryWhere_Search(t *testing.T) {
	_, args := buildAIHistoryWhere(AIHistoryListFilter{
		Search: "search term",
	})
	assert.Len(t, args, 1)
}

func TestAIHistoryFilter_Offset(t *testing.T) {
	f := AIHistoryListFilter{}
	assert.Equal(t, 0, f.offset())

	f.Page = 0
	assert.Equal(t, 0, f.offset())

	f.Page = 3
	f.PageSize = 20
	assert.Equal(t, 40, f.offset())
}

func TestListSuccessfulAIQueries_ZeroLimit(t *testing.T) {
	db, state := setupMockDB(t)
	repo := NewRepository(db)
	ctx := context.Background()

	// When limit is 0, should return nil
	rows, err := repo.ListSuccessfulAIQueries(ctx, "ds-1", nil, 0)
	assert.NoError(t, err)
	assert.Nil(t, rows)
	_ = state // state unused but needed for setup
}

func TestListAIQueryHistory_LimitZero(t *testing.T) {
	db, state := setupMockDB(t)
	repo := NewRepository(db)
	ctx := context.Background()

	state.queries = []queryMock{
		{
			Pattern: "FROM ai_query_history WHERE user_id",
			Cols:    []string{"id", "datasource_id", "model_id", "user_id", "question", "prompt_context", "ai_response", "logical_query", "confidence_score", "warnings", "outcome_status", "retry_count", "needs_clarification", "model_used", "prompt_tokens", "completion_tokens", "token_count", "cost_usd", "latency_ms", "created_at", "ab_experiment_id", "ab_variant_id", "memory_recall_used", "memory_recall_hit_count"},
			Rows:    [][]driver.Value{},
		},
	}

	// When limit <= 0, should default to 50
	rows, err := repo.ListAIQueryHistory(ctx, "u-1", 0)
	assert.NoError(t, err)
	assert.Empty(t, rows)
}

func TestListAIQueryHistory_WithoutUser(t *testing.T) {
	db, state := setupMockDB(t)
	repo := NewRepository(db)
	ctx := context.Background()
	now := time.Now()

	state.queries = []queryMock{
		{
			Pattern: "FROM ai_query_history ORDER BY",
			Cols:    []string{"id", "datasource_id", "model_id", "user_id", "question", "prompt_context", "ai_response", "logical_query", "confidence_score", "warnings", "outcome_status", "retry_count", "needs_clarification", "model_used", "prompt_tokens", "completion_tokens", "token_count", "cost_usd", "latency_ms", "created_at", "ab_experiment_id", "ab_variant_id", "memory_recall_used", "memory_recall_hit_count"},
			Rows: [][]driver.Value{
				{"qh-1", "ds-1", nil, "u-1", "question", []byte(`{}`), nil, nil, nil, `{}`, "success", int64(0), false, nil, nil, nil, nil, nil, nil, now, nil, nil, false, int64(0)},
			},
		},
	}

	entries, err := repo.ListAIQueryHistory(ctx, "", 10)
	assert.NoError(t, err)
	assert.Len(t, entries, 1)
}

// --- SemanticModelHash edge cases ---

func TestSemanticModelHash_EmptyModelID(t *testing.T) {
	assert.Equal(t, "", SemanticModelHash("", 1))
}

// =============================================================================
// TARGETED COVERAGE TESTS — Security Policy error and edge paths
// =============================================================================

// GetSecurityPolicy — covers error return (scan failure / no rows)
func TestGetSecurityPolicy_NotFound(t *testing.T) {
	db, state := setupMockDB(t)
	repo := NewRepository(db)
	ctx := context.Background()

	// QueryRowContext with no matching rows — Scan returns sql.ErrNoRows
	state.queries = []queryMock{
		{
			Pattern: "SELECT id, user_id, datasource_id, allowed_models, denied_fields, row_filters, pii_policy, created_at, updated_at FROM permissions WHERE id",
			Cols:    []string{"id", "user_id", "datasource_id", "allowed_models", "denied_fields", "row_filters", "pii_policy", "created_at", "updated_at"},
			Rows:    [][]driver.Value{}, // empty → ErrNoRows
		},
	}

	p, err := repo.GetSecurityPolicy(ctx, "nonexistent")
	assert.Error(t, err)
	assert.ErrorIs(t, err, sql.ErrNoRows)
	assert.Nil(t, p)
}

// GetSecurityPolicyByKeys — covers error return (no rows)
func TestGetSecurityPolicyByKeys_NotFound(t *testing.T) {
	db, state := setupMockDB(t)
	repo := NewRepository(db)
	ctx := context.Background()

	state.queries = []queryMock{
		{
			Pattern: "SELECT id, user_id, datasource_id, allowed_models, denied_fields, row_filters, pii_policy, created_at, updated_at FROM permissions WHERE user_id",
			Cols:    []string{"id", "user_id", "datasource_id", "allowed_models", "denied_fields", "row_filters", "pii_policy", "created_at", "updated_at"},
			Rows:    [][]driver.Value{},
		},
	}

	p, err := repo.GetSecurityPolicyByKeys(ctx, "nonexistent-user", "ds-1")
	assert.Error(t, err)
	assert.ErrorIs(t, err, sql.ErrNoRows)
	assert.Nil(t, p)
}

// DeleteSecurityPolicy — covers exec error path
func TestDeleteSecurityPolicy_Error(t *testing.T) {
	db, state := setupMockDB(t)
	repo := NewRepository(db)
	ctx := context.Background()

	state.execs = []execMock{
		{Pattern: "DELETE FROM permissions WHERE id", Err: assert.AnError},
	}

	err := repo.DeleteSecurityPolicy(ctx, "p-1")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "delete security policy")
}

// DeleteSecurityPolicyByKeys — covers exec error path
func TestDeleteSecurityPolicyByKeys_Error(t *testing.T) {
	db, state := setupMockDB(t)
	repo := NewRepository(db)
	ctx := context.Background()

	state.execs = []execMock{
		{Pattern: "DELETE FROM permissions WHERE user_id", Err: assert.AnError},
	}

	err := repo.DeleteSecurityPolicyByKeys(ctx, "role:viewer", "ds-1")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "delete security policy by keys")
}

// UpsertSecurityPolicy — covers exec error
func TestUpsertSecurityPolicy_ExecError(t *testing.T) {
	db, state := setupMockDB(t)
	repo := NewRepository(db)
	ctx := context.Background()

	state.execs = []execMock{
		{Pattern: "INSERT INTO permissions", Err: assert.AnError},
	}

	err := repo.UpsertSecurityPolicy(ctx, &SecurityPolicy{
		ID:           "p-1",
		UserID:       "role:viewer",
		DatasourceID: "ds-1",
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "upsert security policy")
}

// =============================================================================
// TARGETED COVERAGE TESTS — scanBusinessGlossaryRow edge paths
// =============================================================================

// scanBusinessGlossaryRow with a populated AI context (non-IsZero)
func TestScanBusinessGlossaryRow_WithAIContext(t *testing.T) {
	db, state := setupMockDB(t)
	repo := NewRepository(db)
	ctx := context.Background()
	now := time.Now()

	state.queries = []queryMock{
		{
			Pattern: "FROM business_glossary_terms",
			Cols:    []string{"id", "datasource_id", "model_id", "term", "definition", "maps_to_type", "maps_to_name", "aliases", "ai_context", "is_active", "created_at", "updated_at"},
			Rows: [][]driver.Value{
				{"bg-1", "ds-1", "m-1", "revenue", "total revenue", "column", "amount", `{income}`, []byte(`{"synonyms":["income","sales"],"unit":"USD","business_rules":["sum only"]}`), true, now, now},
			},
		},
	}

	rows, err := repo.ListBusinessGlossary(ctx, "ds-1", "m-1")
	assert.NoError(t, err)
	assert.Len(t, rows, 1)
	assert.NotNil(t, rows[0].AIContext)
	assert.Equal(t, []string{"income", "sales"}, rows[0].AIContext.Synonyms)
	assert.Equal(t, "USD", rows[0].AIContext.Unit)
}

// scanBusinessGlossaryRow with ai_context that IsZero → AIContext should be nil
func TestScanBusinessGlossaryRow_IsZeroAIContext(t *testing.T) {
	db, state := setupMockDB(t)
	repo := NewRepository(db)
	ctx := context.Background()
	now := time.Now()

	state.queries = []queryMock{
		{
			Pattern: "FROM business_glossary_terms",
			Cols:    []string{"id", "datasource_id", "model_id", "term", "definition", "maps_to_type", "maps_to_name", "aliases", "ai_context", "is_active", "created_at", "updated_at"},
			Rows: [][]driver.Value{
				{"bg-2", "ds-1", "", "term2", "def2", "table", "orders", `{}`, []byte(`{"synonyms":[],"unit":"","null_meaning":"","business_rules":[]}`), true, now, now},
			},
		},
	}

	rows, err := repo.ListBusinessGlossary(ctx, "ds-1", "")
	assert.NoError(t, err)
	assert.Len(t, rows, 1)
	assert.Nil(t, rows[0].AIContext)
}

// scanBusinessGlossaryRow with nil ai_context (SQL NULL) → AIContext nil
func TestScanBusinessGlossaryRow_NilAIContext(t *testing.T) {
	db, state := setupMockDB(t)
	repo := NewRepository(db)
	ctx := context.Background()
	now := time.Now()

	state.queries = []queryMock{
		{
			Pattern: "FROM business_glossary_terms",
			Cols:    []string{"id", "datasource_id", "model_id", "term", "definition", "maps_to_type", "maps_to_name", "aliases", "ai_context", "is_active", "created_at", "updated_at"},
			Rows: [][]driver.Value{
				{"bg-3", "ds-1", "m-1", "term3", "def3", "column", "id", `{}`, nil, true, now, now},
			},
		},
	}

	rows, err := repo.ListBusinessGlossary(ctx, "ds-1", "m-1")
	assert.NoError(t, err)
	assert.Len(t, rows, 1)
	assert.Nil(t, rows[0].AIContext)
}

// =============================================================================
// TARGETED COVERAGE TESTS — Business Glossary Insert / Update / Delete edges
// =============================================================================

// InsertBusinessGlossary with empty ModelID (triggers modelID = nil path)
func TestInsertBusinessGlossary_EmptyModelID(t *testing.T) {
	db, state := setupMockDB(t)
	repo := NewRepository(db)
	ctx := context.Background()

	state.queries = []queryMock{
		{
			Pattern: "INSERT INTO business_glossary_terms",
			Cols:    []string{"id"},
			Rows:    [][]driver.Value{{"bg-empty-model"}},
		},
	}

	id, err := repo.InsertBusinessGlossary(ctx, BusinessGlossaryInsert{
		DatasourceID: "ds-1",
		ModelID:      "", // empty → modelID = nil branch
		Term:         "test",
		Definition:   "desc",
		MapsToType:   "table",
		MapsToName:   "users",
		Aliases:      nil,
		AIContext:    nil,
	})
	assert.NoError(t, err)
	assert.Equal(t, "bg-empty-model", id)
}

// InsertBusinessGlossary with AIContext (non-nil, non-IsZero)
func TestInsertBusinessGlossary_WithAIContext(t *testing.T) {
	db, state := setupMockDB(t)
	repo := NewRepository(db)
	ctx := context.Background()

	state.queries = []queryMock{
		{
			Pattern: "INSERT INTO business_glossary_terms",
			Cols:    []string{"id"},
			Rows:    [][]driver.Value{{"bg-ai"}},
		},
	}

	id, err := repo.InsertBusinessGlossary(ctx, BusinessGlossaryInsert{
		DatasourceID: "ds-1",
		ModelID:      "m-1",
		Term:         "term",
		Definition:   "",
		MapsToType:   "column",
		MapsToName:   "amount",
		Aliases:      []string{"alias1"},
		AIContext: &pkgmetadata.GlossaryAIContext{
			Synonyms: []string{"alt"},
			Unit:     "USD",
		},
	})
	assert.NoError(t, err)
	assert.Equal(t, "bg-ai", id)
}

// UpdateBusinessGlossary with nil IsActive (triggers default active=true)
func TestUpdateBusinessGlossary_NilIsActive(t *testing.T) {
	db, state := setupMockDB(t)
	repo := NewRepository(db)
	ctx := context.Background()

	state.execs = []execMock{
		{Pattern: "UPDATE business_glossary_terms", RowsAffected: 1},
	}

	err := repo.UpdateBusinessGlossary(ctx, "bg-1", BusinessGlossaryUpdate{
		Term:       "updated",
		Definition: "updated def",
		MapsToType: "table",
		MapsToName: "orders",
		Aliases:    []string{"a", "b"},
		IsActive:   nil, // should default to true
		AIContext:  nil,
	})
	assert.NoError(t, err)
}

// UpdateBusinessGlossary with IsActive set to false
func TestUpdateBusinessGlossary_IsActiveFalse(t *testing.T) {
	db, state := setupMockDB(t)
	repo := NewRepository(db)
	ctx := context.Background()

	state.execs = []execMock{
		{Pattern: "UPDATE business_glossary_terms", RowsAffected: 1},
	}

	isActive := false
	err := repo.UpdateBusinessGlossary(ctx, "bg-1", BusinessGlossaryUpdate{
		Term:       "deactivated",
		Definition: "",
		MapsToType: "table",
		MapsToName: "users",
		Aliases:    []string{},
		IsActive:   &isActive,
		AIContext:  nil,
	})
	assert.NoError(t, err)
}

// DeleteBusinessGlossary returns false when no rows affected
func TestDeleteBusinessGlossary_NotFound(t *testing.T) {
	db, state := setupMockDB(t)
	repo := NewRepository(db)
	ctx := context.Background()

	state.execs = []execMock{
		{Pattern: "DELETE FROM business_glossary_terms WHERE id", RowsAffected: 0},
	}

	found, err := repo.DeleteBusinessGlossary(ctx, "nonexistent")
	assert.NoError(t, err)
	assert.False(t, found)
}

// DeleteBusinessGlossary — exec error
func TestDeleteBusinessGlossary_Error(t *testing.T) {
	db, state := setupMockDB(t)
	repo := NewRepository(db)
	ctx := context.Background()

	state.execs = []execMock{
		{Pattern: "DELETE FROM business_glossary_terms WHERE id", Err: assert.AnError},
	}

	found, err := repo.DeleteBusinessGlossary(ctx, "bg-1")
	assert.Error(t, err)
	assert.False(t, found)
}

// =============================================================================
// TARGETED COVERAGE TESTS — Prompt Templates
// =============================================================================

// GetPromptTemplateByVersion — happy path
func TestGetPromptTemplateByVersion_Happy(t *testing.T) {
	db, state := setupMockDB(t)
	repo := NewRepository(db)
	ctx := context.Background()

	state.queries = []queryMock{
		{
			Pattern: "SELECT content FROM ai_prompt_templates WHERE name",
			Cols:    []string{"content"},
			Rows:    [][]driver.Value{{"exact version content"}},
		},
	}

	content, err := repo.GetPromptTemplateByVersion(ctx, "system_prompt", "en", 2)
	assert.NoError(t, err)
	assert.Equal(t, "exact version content", content)
}

// GetPromptTemplateByVersion — not found (ErrNoRows → returns "", nil)
func TestGetPromptTemplateByVersion_NotFound(t *testing.T) {
	db, state := setupMockDB(t)
	repo := NewRepository(db)
	ctx := context.Background()

	state.queries = []queryMock{
		{
			Pattern: "SELECT content FROM ai_prompt_templates WHERE name",
			Cols:    []string{"content"},
			Rows:    [][]driver.Value{},
		},
	}

	content, err := repo.GetPromptTemplateByVersion(ctx, "nonexistent", "en", 99)
	assert.NoError(t, err)
	assert.Equal(t, "", content)
}

// GetPromptTemplateByVersion — empty locale (defaults to DefaultLocale)
func TestGetPromptTemplateByVersion_EmptyLocale(t *testing.T) {
	db, state := setupMockDB(t)
	repo := NewRepository(db)
	ctx := context.Background()

	state.queries = []queryMock{
		{
			Pattern: "SELECT content FROM ai_prompt_templates WHERE name",
			Cols:    []string{"content"},
			Rows:    [][]driver.Value{{"default locale content"}},
		},
	}

	content, err := repo.GetPromptTemplateByVersion(ctx, "system_prompt", "", 1)
	assert.NoError(t, err)
	assert.Equal(t, "default locale content", content)
}

// GetPromptTemplateVersion — empty locale (defaults to DefaultLocale)
func TestGetPromptTemplateVersion_EmptyLocale(t *testing.T) {
	db, state := setupMockDB(t)
	repo := NewRepository(db)
	ctx := context.Background()

	state.queries = []queryMock{
		{
			Pattern: "SELECT content, version FROM ai_prompt_templates",
			Cols:    []string{"content", "version"},
			Rows:    [][]driver.Value{{"content", int64(1)}},
		},
	}

	content, version, err := repo.GetPromptTemplateVersion(ctx, "system_prompt", "")
	assert.NoError(t, err)
	assert.Equal(t, "content", content)
	assert.Equal(t, 1, version)
}

// GetPromptTemplateVersion — no active row (ErrNoRows → returns "", 0, nil)
func TestGetPromptTemplateVersion_NotFound(t *testing.T) {
	db, state := setupMockDB(t)
	repo := NewRepository(db)
	ctx := context.Background()

	state.queries = []queryMock{
		{
			Pattern: "SELECT content, version FROM ai_prompt_templates",
			Cols:    []string{"content", "version"},
			Rows:    [][]driver.Value{},
		},
	}

	content, version, err := repo.GetPromptTemplateVersion(ctx, "nonexistent", "en")
	assert.NoError(t, err)
	assert.Equal(t, "", content)
	assert.Equal(t, 0, version)
}

// UpsertPromptTemplate with empty locale (defaults to DefaultLocale)
func TestUpsertPromptTemplate_EmptyLocale(t *testing.T) {
	db, state := setupMockDB(t)
	repo := NewRepository(db)
	ctx := context.Background()

	state.queries = []queryMock{
		{
			Pattern: "SELECT COALESCE(MAX(version), 0) + 1 FROM ai_prompt_templates",
			Cols:    []string{"next_version"},
			Rows:    [][]driver.Value{{int64(5)}},
		},
	}
	state.execs = []execMock{
		{Pattern: "UPDATE ai_prompt_templates SET is_active", RowsAffected: 1},
		{Pattern: "INSERT INTO ai_prompt_templates", RowsAffected: 1},
	}

	err := repo.UpsertPromptTemplate(ctx, "system_prompt", "", "new content with empty locale")
	assert.NoError(t, err)
}

// UpsertPromptTemplate — first version (COALESCE returns 0 + 1 = 1)
func TestUpsertPromptTemplate_FirstVersion(t *testing.T) {
	db, state := setupMockDB(t)
	repo := NewRepository(db)
	ctx := context.Background()

	state.queries = []queryMock{
		{
			Pattern: "SELECT COALESCE(MAX(version), 0) + 1 FROM ai_prompt_templates",
			Cols:    []string{"next_version"},
			Rows:    [][]driver.Value{{int64(1)}},
		},
	}
	state.execs = []execMock{
		{Pattern: "UPDATE ai_prompt_templates SET is_active", RowsAffected: 0},
		{Pattern: "INSERT INTO ai_prompt_templates", RowsAffected: 1},
	}

	// With locale explicitly set
	err := repo.UpsertPromptTemplate(ctx, "new_prompt", i18n.LocaleEN, "brand new prompt")
	assert.NoError(t, err)
}

// ListPromptTemplates — empty result
func TestListPromptTemplates_Empty(t *testing.T) {
	db, state := setupMockDB(t)
	repo := NewRepository(db)
	ctx := context.Background()

	state.queries = []queryMock{
		{
			Pattern: "SELECT name, locale, version, content, is_active, created_at, updated_at FROM ai_prompt_templates",
			Cols:    []string{"name", "locale", "version", "content", "is_active", "created_at", "updated_at"},
			Rows:    [][]driver.Value{},
		},
	}

	templates, err := repo.ListPromptTemplates(ctx)
	assert.NoError(t, err)
	assert.Empty(t, templates)
}

// ListPromptTemplates — multiple rows
func TestListPromptTemplates_Multiple(t *testing.T) {
	db, state := setupMockDB(t)
	repo := NewRepository(db)
	ctx := context.Background()
	now := time.Now()

	state.queries = []queryMock{
		{
			Pattern: "SELECT name, locale, version, content, is_active, created_at, updated_at FROM ai_prompt_templates",
			Cols:    []string{"name", "locale", "version", "content", "is_active", "created_at", "updated_at"},
			Rows: [][]driver.Value{
				{"system_prompt", "en", int64(2), "v2 content", true, now, now},
				{"system_prompt", "en", int64(1), "v1 content", false, now, now},
				{"user_prompt", "tr", int64(1), "tr content", true, now, now},
			},
		},
	}

	templates, err := repo.ListPromptTemplates(ctx)
	assert.NoError(t, err)
	assert.Len(t, templates, 3)
	assert.Equal(t, "system_prompt", templates[0].Name)
	assert.Equal(t, "v2 content", templates[0].Content)
	assert.True(t, templates[0].IsActive)
	assert.Equal(t, 2, templates[0].Version)
}

// UpsertPromptTemplate — error in next version query (within TX)
func TestUpsertPromptTemplate_NextVersionError(t *testing.T) {
	db, state := setupMockDB(t)
	repo := NewRepository(db)
	ctx := context.Background()

	state.queries = []queryMock{
		{
			Pattern: "SELECT COALESCE(MAX(version), 0) + 1 FROM ai_prompt_templates",
			Err:     assert.AnError,
		},
	}

	err := repo.UpsertPromptTemplate(ctx, "system_prompt", "en", "new content")
	assert.Error(t, err)
}

// UpsertPromptTemplate — error in deactivate step (within TX)
func TestUpsertPromptTemplate_DeactivateError(t *testing.T) {
	db, state := setupMockDB(t)
	repo := NewRepository(db)
	ctx := context.Background()

	state.queries = []queryMock{
		{
			Pattern: "SELECT COALESCE(MAX(version), 0) + 1 FROM ai_prompt_templates",
			Cols:    []string{"next_version"},
			Rows:    [][]driver.Value{{int64(2)}},
		},
	}
	state.execs = []execMock{
		{Pattern: "UPDATE ai_prompt_templates SET is_active", Err: assert.AnError},
	}

	err := repo.UpsertPromptTemplate(ctx, "system_prompt", "en", "new content")
	assert.Error(t, err)
}

// UpsertPromptTemplate — error in insert step (within TX)
func TestUpsertPromptTemplate_InsertError(t *testing.T) {
	db, state := setupMockDB(t)
	repo := NewRepository(db)
	ctx := context.Background()

	state.queries = []queryMock{
		{
			Pattern: "SELECT COALESCE(MAX(version), 0) + 1 FROM ai_prompt_templates",
			Cols:    []string{"next_version"},
			Rows:    [][]driver.Value{{int64(2)}},
		},
	}
	state.execs = []execMock{
		{Pattern: "UPDATE ai_prompt_templates SET is_active", RowsAffected: 1},
		{Pattern: "INSERT INTO ai_prompt_templates", Err: assert.AnError},
	}

	err := repo.UpsertPromptTemplate(ctx, "system_prompt", "en", "new content")
	assert.Error(t, err)
}

// scanBusinessGlossaryRow — invalid JSON in ai_context (unmarshal error)
func TestScanBusinessGlossaryRow_InvalidAIContextJSON(t *testing.T) {
	db, state := setupMockDB(t)
	repo := NewRepository(db)
	ctx := context.Background()
	now := time.Now()

	state.queries = []queryMock{
		{
			Pattern: "FROM business_glossary_terms",
			Cols:    []string{"id", "datasource_id", "model_id", "term", "definition", "maps_to_type", "maps_to_name", "aliases", "ai_context", "is_active", "created_at", "updated_at"},
			Rows: [][]driver.Value{
				{"bg-1", "ds-1", "m-1", "term", "def", "table", "users", `{}`, []byte(`{invalid json}`), true, now, now},
			},
		},
	}

	_, err := repo.ListBusinessGlossary(ctx, "ds-1", "m-1")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unmarshal")
}

// ListPromptTemplates — scan error within rows iteration
func TestListPromptTemplates_ScanError(t *testing.T) {
	db, state := setupMockDB(t)
	repo := NewRepository(db)
	ctx := context.Background()
	now := time.Now()

	state.queries = []queryMock{
		{
			Pattern: "SELECT name, locale, version, content, is_active, created_at, updated_at FROM ai_prompt_templates",
			Cols:    []string{"name", "locale", "version", "content", "is_active", "created_at", "updated_at"},
			Rows: [][]driver.Value{
				{"name", "en", "not-an-int", "content", "not-a-bool", now, now},
			},
		},
	}

	templates, err := repo.ListPromptTemplates(ctx)
	assert.Error(t, err)
	assert.Nil(t, templates)
}

// InsertBusinessGlossary — QueryRowContext error
func TestInsertBusinessGlossary_QueryError(t *testing.T) {
	db, state := setupMockDB(t)
	repo := NewRepository(db)
	ctx := context.Background()

	state.queries = []queryMock{
		{
			Pattern: "INSERT INTO business_glossary_terms",
			Rows:    nil, // no mock → will return error
			Err:     assert.AnError,
		},
	}

	id, err := repo.InsertBusinessGlossary(ctx, BusinessGlossaryInsert{
		DatasourceID: "ds-1",
		ModelID:      "m-1",
		Term:         "term",
		Definition:   "def",
		MapsToType:   "table",
		MapsToName:   "users",
	})
	assert.Error(t, err)
	assert.Equal(t, "", id)
}

// UpdateBusinessGlossary — ExecContext error
func TestUpdateBusinessGlossary_ExecError(t *testing.T) {
	db, state := setupMockDB(t)
	repo := NewRepository(db)
	ctx := context.Background()

	state.execs = []execMock{
		{Pattern: "UPDATE business_glossary_terms", Err: assert.AnError},
	}

	err := repo.UpdateBusinessGlossary(ctx, "bg-1", BusinessGlossaryUpdate{
		Term:       "t",
		Definition: "d",
		MapsToType: "table",
		MapsToName: "users",
	})
	assert.Error(t, err)
}

// scanSecurityPolicy — invalid row_filters JSON (unmarshal error)
func TestGetSecurityPolicy_InvalidRowFiltersJSON(t *testing.T) {
	db, state := setupMockDB(t)
	repo := NewRepository(db)
	ctx := context.Background()
	now := time.Now()

	state.queries = []queryMock{
		{
			Pattern: "SELECT id, user_id, datasource_id, allowed_models, denied_fields, row_filters, pii_policy, created_at, updated_at FROM permissions WHERE id",
			Cols:    []string{"id", "user_id", "datasource_id", "allowed_models", "denied_fields", "row_filters", "pii_policy", "created_at", "updated_at"},
			Rows: [][]driver.Value{
				{"p-1", "role:viewer", "ds-1", `{}`, `{}`, []byte(`not valid json`), []byte(`{}`), now, now},
			},
		},
	}

	_, err := repo.GetSecurityPolicy(ctx, "p-1")
	assert.Error(t, err)
}

// scanSecurityPolicy — invalid pii_policy JSON (unmarshal error)
func TestGetSecurityPolicy_InvalidPIIPolicyJSON(t *testing.T) {
	db, state := setupMockDB(t)
	repo := NewRepository(db)
	ctx := context.Background()
	now := time.Now()

	state.queries = []queryMock{
		{
			Pattern: "SELECT id, user_id, datasource_id, allowed_models, denied_fields, row_filters, pii_policy, created_at, updated_at FROM permissions WHERE id",
			Cols:    []string{"id", "user_id", "datasource_id", "allowed_models", "denied_fields", "row_filters", "pii_policy", "created_at", "updated_at"},
			Rows: [][]driver.Value{
				{"p-1", "role:viewer", "ds-1", `{}`, `{}`, []byte(`[]`), []byte(`invalid json`), now, now},
			},
		},
	}

	_, err := repo.GetSecurityPolicy(ctx, "p-1")
	assert.Error(t, err)
}

// CountPromptTemplates — query error
func TestCountPromptTemplates_QueryError(t *testing.T) {
	db, state := setupMockDB(t)
	repo := NewRepository(db)
	ctx := context.Background()

	state.queries = []queryMock{
		{Pattern: "SELECT COUNT(*) FROM ai_prompt_templates", Err: assert.AnError},
	}

	count, err := repo.CountPromptTemplates(ctx)
	assert.Error(t, err)
	assert.Equal(t, 0, count)
}

// scanBusinessGlossaryRow — scan error (wrong type for boolean field)
func TestScanBusinessGlossaryRow_ScanError(t *testing.T) {
	db, state := setupMockDB(t)
	repo := NewRepository(db)
	ctx := context.Background()
	now := time.Now()

	// Provide a string that can't convert to bool for is_active → scan error
	state.queries = []queryMock{
		{
			Pattern: "FROM business_glossary_terms",
			Cols:    []string{"id", "datasource_id", "model_id", "term", "definition", "maps_to_type", "maps_to_name", "aliases", "ai_context", "is_active", "created_at", "updated_at"},
			Rows: [][]driver.Value{
				{"bg-1", "ds-1", "m-1", "term", "def", "table", "users", `{}`, nil, "not-a-boolean-value", now, now},
			},
		},
	}

	_, err := repo.ListBusinessGlossary(ctx, "ds-1", "m-1")
	assert.Error(t, err)
}

// ListPromptTemplates — query error (QueryContext fails)
func TestListPromptTemplates_QueryError(t *testing.T) {
	db, state := setupMockDB(t)
	repo := NewRepository(db)
	ctx := context.Background()

	state.queries = []queryMock{
		{
			Pattern: "SELECT name, locale, version, content, is_active, created_at, updated_at FROM ai_prompt_templates",
			Err:     assert.AnError,
		},
	}

	templates, err := repo.ListPromptTemplates(ctx)
	assert.Error(t, err)
	assert.Nil(t, templates)
}

// DeleteAllPromptTemplates — exec error
func TestDeleteAllPromptTemplates_ExecError(t *testing.T) {
	db, state := setupMockDB(t)
	repo := NewRepository(db)
	ctx := context.Background()

	state.execs = []execMock{
		{Pattern: "DELETE FROM ai_prompt_templates", Err: assert.AnError},
	}

	err := repo.DeleteAllPromptTemplates(ctx)
	assert.Error(t, err)
}

// GetPromptTemplateByVersion — error from QueryRowContext
func TestGetPromptTemplateByVersion_QueryError(t *testing.T) {
	db, _ := setupMockDB(t)
	repo := NewRepository(db)
	ctx := context.Background()

	// No matching mock → mock returns error
	content, err := repo.GetPromptTemplateByVersion(ctx, "any", "en", 1)
	assert.Error(t, err)
	assert.Equal(t, "", content)
}

// GetPromptTemplateVersion — error from QueryRowContext
func TestGetPromptTemplateVersion_QueryError(t *testing.T) {
	db, _ := setupMockDB(t)
	repo := NewRepository(db)
	ctx := context.Background()

	// No matching mock → mock returns error
	content, version, err := repo.GetPromptTemplateVersion(ctx, "any", "en")
	assert.Error(t, err)
	assert.Equal(t, "", content)
	assert.Equal(t, 0, version)
}
