package handlers

import (
	"context"
	"strings"
	"testing"

	"github.com/biqly/biqly/internal/app"
	"github.com/biqly/biqly/internal/metadata"
	"github.com/biqly/biqly/internal/semantic"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newSemanticHandlerWithRepo(repo *metadata.Repository) *SemanticHandler {
	return &SemanticHandler{deps: (&app.Dependencies{MetaRepo: repo}).CatalogDeps()}
}

func findCall(calls []mockCall, pattern string) *mockCall {
	for i := range calls {
		if strings.Contains(calls[i].Op, pattern) {
			return &calls[i]
		}
	}
	return nil
}

// On a valid publish, confirmed queries stored under a different semantic model
// hash must be deactivated using the freshly published version's hash.
func TestDeactivateStaleConfirmedQueriesUsesPublishedHash(t *testing.T) {
	db, state := setupMockDB(t)
	repo := metadata.NewRepository(db)
	state.execs = []execMock{
		{Pattern: "UPDATE ai_saved_queries", RowsAffected: 3},
	}

	h := newSemanticHandlerWithRepo(repo)
	h.deactivateStaleConfirmedQueries(context.Background(), &semantic.PublishResult{
		Model:   &semantic.SemanticModel{ID: "m-1"},
		Version: 3,
	})

	call := findCall(state.calls, "UPDATE ai_saved_queries")
	require.NotNil(t, call, "expected deactivation UPDATE to run")
	require.Len(t, call.Args, 2)
	assert.Equal(t, "m-1", call.Args[0])
	assert.Equal(t, metadata.SemanticModelHash("m-1", 3), call.Args[1])
}

// The hook is a no-op when there is no published model, so it never issues a
// destructive UPDATE on a missing/invalid publish result.
func TestDeactivateStaleConfirmedQueriesNoModelIsNoop(t *testing.T) {
	db, state := setupMockDB(t)
	repo := metadata.NewRepository(db)
	h := newSemanticHandlerWithRepo(repo)

	h.deactivateStaleConfirmedQueries(context.Background(), nil)
	h.deactivateStaleConfirmedQueries(context.Background(), &semantic.PublishResult{Version: 2})

	assert.Nil(t, findCall(state.calls, "UPDATE ai_saved_queries"))
}
