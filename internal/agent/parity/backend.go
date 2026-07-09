package parity

import (
	"log/slog"
	"net/http"

	"github.com/bytedance/sonic"

	"github.com/biqly/biqly/internal/query"
)

// topCountrySkillQuery is the LogicalQuery template saved skill
// SkillTopCountry runs — borrowed from internal/ai/eval.BenchmarkCases'
// "bench-top-country-by-revenue" shape.
func topCountrySkillQuery() query.LogicalQuery {
	return query.LogicalQuery{
		DatasourceID: DatasourceOrders,
		ModelID:      ModelOrders,
		Select: []query.SelectItem{
			{Type: "dimension", Name: "country"},
			{Type: "metric", Name: "total_amount"},
		},
		GroupBy: []query.GroupBy{{Field: "country"}},
		Filters: []query.Filter{
			{Field: "country", Operator: "is_not_null", Value: nil},
			{Field: "country", Operator: "neq", Value: ""},
		},
		OrderBy: []query.OrderBy{{Field: "total_amount", Direction: "desc"}},
		Limit:   1,
	}
}

// defaultLogicalQuery is what the fake backend returns for a run_question
// call whose question isn't in goldenLogicalQueries — keeps the backend
// total (never 500s on an unexpected question) instead of silently
// dropping test coverage of a typo'd case question.
func defaultLogicalQuery() query.LogicalQuery {
	return query.LogicalQuery{
		Select: []query.SelectItem{{Type: "metric", Name: "row_count"}},
		Limit:  100,
	}
}

// NewFakeBackend builds the deterministic governed-backend double both the
// MCP and web-agent paths dispatch against in this harness (see the package
// doc comment: this in-process harness stands in for a real /api/* router +
// DB + ai.Service so it can run without a live cluster, exactly like
// internal/http/mcp_server_test.go's mcpTestBackend). Responses are a
// function of the request only, so calling it twice with identical input
// (once per channel) always yields identical output — any observed
// difference between the MCP and web-agent paths for the same tool+args is
// therefore a real divergence in how each path calls the shared contract,
// never backend nondeterminism.
func NewFakeBackend() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/datasources", handleListDatasources)
	mux.HandleFunc("GET /api/semantic/models", handleListModels)
	mux.HandleFunc("POST /api/ai/query/run", handleRunQuestion)
	mux.HandleFunc("POST /api/query/run", handleRunLogicalQuery)
	mux.HandleFunc("GET /api/ai/skills", handleListSkills)
	mux.HandleFunc("POST /api/ai/skills/{id}/run", handleRunSkill)
	return mux
}

// writeJSON encodes v as the response body. Every call site passes a
// static, package-controlled value (map[string]string/any literals or
// query.LogicalQuery), so an encode failure here would indicate a
// programmer error in one of those literals, not a runtime condition — it
// is logged rather than retried since the status/headers are already sent.
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := sonic.ConfigStd.NewEncoder(w).Encode(v); err != nil {
		slog.Error("parity fake backend: encode response", "error", err)
	}
}

func handleListDatasources(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, []map[string]string{
		{"id": DatasourceOrders, "name": "Orders"},
	})
}

func handleListModels(w http.ResponseWriter, r *http.Request) {
	ds := r.URL.Query().Get("datasource_id")
	if ds != "" && ds != DatasourceOrders {
		writeJSON(w, http.StatusOK, []map[string]string{})
		return
	}
	writeJSON(w, http.StatusOK, []map[string]string{
		{"id": ModelOrders, "name": "Orders", "datasource_id": DatasourceOrders},
	})
}

func handleRunQuestion(w http.ResponseWriter, r *http.Request) {
	var req struct {
		DatasourceID string `json:"datasource_id"`
		Question     string `json:"question"`
		ModelID      string `json:"model_id"`
	}
	if err := sonic.ConfigStd.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	modelID := req.ModelID
	if modelID == "" {
		// Auto-selection stand-in: ds-1 has exactly one model in this fixture
		// set, so "automatic routing" (toolcontract.RunQuestionInput's
		// documented model_id-omitted behavior) always resolves to it.
		modelID = ModelOrders
	}
	lq, ok := goldenLogicalQueries[req.Question]
	if !ok {
		lq = defaultLogicalQuery()
	}
	lq.DatasourceID = req.DatasourceID
	lq.ModelID = modelID
	writeJSON(w, http.StatusOK, map[string]any{
		"datasource_id": req.DatasourceID,
		"model_id":      modelID,
		"logical_query": lq,
		"rows":          []map[string]any{},
	})
}

func handleRunLogicalQuery(w http.ResponseWriter, r *http.Request) {
	var req struct {
		LogicalQuery query.LogicalQuery `json:"logical_query"`
	}
	if err := sonic.ConfigStd.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"datasource_id": req.LogicalQuery.DatasourceID,
		"model_id":      req.LogicalQuery.ModelID,
		"logical_query": req.LogicalQuery,
		"rows":          []map[string]any{},
	})
}

func handleListSkills(w http.ResponseWriter, r *http.Request) {
	ds := r.URL.Query().Get("datasource_id")
	if ds != "" && ds != DatasourceOrders {
		writeJSON(w, http.StatusOK, []map[string]string{})
		return
	}
	writeJSON(w, http.StatusOK, []map[string]string{
		{"id": SkillTopCountry, "name": "Top country by revenue", "datasource_id": DatasourceOrders},
	})
}

func handleRunSkill(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id != SkillTopCountry {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "unknown skill " + id})
		return
	}
	lq := topCountrySkillQuery()
	writeJSON(w, http.StatusOK, map[string]any{
		"datasource_id": lq.DatasourceID,
		"model_id":      lq.ModelID,
		"logical_query": lq,
		"rows":          []map[string]any{},
	})
}
