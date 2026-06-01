package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/biqly/biqly/internal/query"
	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEvalCaseCRUD(t *testing.T) {
	h := &AIHandler{}

	// 1. Create a golden case
	reqBody := evalCaseWire{
		ID:       "test-http-case",
		Question: "how many order records exist?",
		ModelID:  "orders",
		Expected: query.LogicalQuery{
			Select: []query.SelectItem{{Type: "metric", Name: "row_count"}},
			Limit:  10,
		},
	}
	bodyBytes, err := json.Marshal(reqBody)
	require.NoError(t, err)

	w := httptest.NewRecorder()
	r := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/ai/eval/cases", bytes.NewReader(bodyBytes))
	h.EvalCreateCase(w, r)

	assert.Equal(t, http.StatusCreated, w.Code)
	var createResp map[string]any
	err = json.NewDecoder(w.Body).Decode(&createResp)
	require.NoError(t, err)
	assert.Equal(t, "created", createResp["status"])
	assert.Equal(t, "test-http-case", createResp["id"])

	// 2. List cases and verify it exists
	w = httptest.NewRecorder()
	r = httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/ai/eval/cases", nil)
	h.EvalListCases(w, r)

	assert.Equal(t, http.StatusOK, w.Code)
	var listResp []evalCaseWire
	err = json.NewDecoder(w.Body).Decode(&listResp)
	require.NoError(t, err)

	found := false
	for _, c := range listResp {
		if c.ID == "test-http-case" {
			found = true
			assert.Equal(t, "how many order records exist?", c.Question)
			assert.Equal(t, "orders", c.ModelID)
			break
		}
	}
	assert.True(t, found, "expected test-http-case to be in list")

	// 3. Delete the case
	w = httptest.NewRecorder()
	r = httptest.NewRequestWithContext(context.Background(), http.MethodDelete, "/api/ai/eval/cases/test-http-case", nil)
	
	// Setup route URL param id using chi context
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "test-http-case")
	r = r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))
	
	h.EvalDeleteCase(w, r)

	assert.Equal(t, http.StatusOK, w.Code)
	var deleteResp map[string]any
	err = json.NewDecoder(w.Body).Decode(&deleteResp)
	require.NoError(t, err)
	assert.Equal(t, "deleted", deleteResp["status"])
	assert.Equal(t, "test-http-case", deleteResp["id"])

	// 4. List cases and verify it is deleted
	w = httptest.NewRecorder()
	r = httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/ai/eval/cases", nil)
	h.EvalListCases(w, r)

	assert.Equal(t, http.StatusOK, w.Code)
	err = json.NewDecoder(w.Body).Decode(&listResp)
	require.NoError(t, err)

	for _, c := range listResp {
		if c.ID == "test-http-case" {
			t.Fatal("expected test-http-case to be deleted, but it was found in the list")
		}
	}
}

func TestEvalCaseCRUD_ValidationErrors(t *testing.T) {
	h := &AIHandler{}

	t.Run("missing ID", func(t *testing.T) {
		req := evalCaseWire{
			Question: "q",
			ModelID:  "orders",
		}
		b, _ := json.Marshal(req)
		w := httptest.NewRecorder()
		r := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/ai/eval/cases", bytes.NewReader(b))
		h.EvalCreateCase(w, r)
		assert.Equal(t, http.StatusBadRequest, w.Code)
		assert.Contains(t, w.Body.String(), "id is required")
	})

	t.Run("missing Question", func(t *testing.T) {
		req := evalCaseWire{
			ID:      "id-1",
			ModelID: "orders",
		}
		b, _ := json.Marshal(req)
		w := httptest.NewRecorder()
		r := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/ai/eval/cases", bytes.NewReader(b))
		h.EvalCreateCase(w, r)
		assert.Equal(t, http.StatusBadRequest, w.Code)
		assert.Contains(t, w.Body.String(), "question is required")
	})

	t.Run("missing ModelID", func(t *testing.T) {
		req := evalCaseWire{
			ID:       "id-1",
			Question: "q",
		}
		b, _ := json.Marshal(req)
		w := httptest.NewRecorder()
		r := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/ai/eval/cases", bytes.NewReader(b))
		h.EvalCreateCase(w, r)
		assert.Equal(t, http.StatusBadRequest, w.Code)
		assert.Contains(t, w.Body.String(), "model_id is required")
	})

	t.Run("invalid json body", func(t *testing.T) {
		w := httptest.NewRecorder()
		r := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/ai/eval/cases", strings.NewReader("{invalid"))
		h.EvalCreateCase(w, r)
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})
}
