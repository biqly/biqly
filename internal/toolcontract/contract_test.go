package toolcontract

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"testing"

	"github.com/bytedance/sonic"
)

// recordedReq captures what the fake backend received.
type recordedReq struct {
	method  string
	path    string
	query   string
	body    string
	headers http.Header
}

// fakeBackend returns an http.Handler that always replies with the given status
// and body, recording the incoming request.
func fakeBackend(t *testing.T, status int, respBody string) (http.Handler, *recordedReq) {
	t.Helper()
	rec := &recordedReq{}
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		rec.method = r.Method
		rec.path = r.URL.Path
		rec.query = r.URL.RawQuery
		rec.body = string(body)
		rec.headers = r.Header.Clone()
		w.WriteHeader(status)
		_, _ = w.Write([]byte(respBody))
	})
	return handler, rec
}

func TestAllTools_HasExactlyTenTools(t *testing.T) {
	want := map[ToolName]bool{
		ToolListDatasources:     true,
		ToolListModels:          true,
		ToolListPromptTemplates: true,
		ToolRunQuestion:         true,
		ToolRunLogicalQuery:     true,
		ToolListSkills:          true,
		ToolRunSkill:            true,
		ToolDryPlan:             true,
		ToolDryRun:              true,
		ToolMetricQuery:         true,
	}
	if len(AllTools) != 10 {
		t.Fatalf("expected 10 tools, got %d", len(AllTools))
	}
	for _, spec := range AllTools {
		if !want[spec.Name] {
			t.Errorf("unexpected tool %q in AllTools", spec.Name)
		}
		if spec.Description == "" {
			t.Errorf("tool %q has empty description", spec.Name)
		}
		if spec.Method == "" || spec.Path == "" {
			t.Errorf("tool %q has empty method or path", spec.Name)
		}
	}
}

func TestHTTPDispatcher_ForwardsCredentialsAndChannel(t *testing.T) {
	backend, rec := fakeBackend(t, http.StatusOK, `{"ok":true}`)
	disp := &HTTPDispatcher{API: backend}

	cred := Credential{Authorization: "Bearer jwt-token", APIKey: "pat-key"}
	res, err := disp.Dispatch(context.Background(), http.MethodGet, "/api/datasources", nil, cred, ChannelAgent)
	if err != nil {
		t.Fatalf("dispatch: %v", err)
	}
	if res.IsError() {
		t.Fatalf("unexpected error status %d", res.StatusCode)
	}

	if got := rec.headers.Get("Authorization"); got != "Bearer jwt-token" {
		t.Errorf("Authorization = %q, want %q", got, "Bearer jwt-token")
	}
	if got := rec.headers.Get("X-API-Key"); got != "pat-key" {
		t.Errorf("X-API-Key = %q, want %q", got, "pat-key")
	}
	if got := rec.headers.Get("X-Biqly-Channel"); got != ChannelAgent {
		t.Errorf("X-Biqly-Channel = %q, want %q", got, ChannelAgent)
	}
	if rec.method != http.MethodGet || rec.path != "/api/datasources" {
		t.Errorf("dispatch path = %s %s, want GET /api/datasources", rec.method, rec.path)
	}

	var body map[string]bool
	if err := sonic.Unmarshal(res.Body, &body); err != nil {
		t.Fatalf("unmarshal body: %v: %s", err, string(res.Body))
	}
	if !body["ok"] {
		t.Errorf("expected ok=true in body, got %s", string(res.Body))
	}
}

func TestHTTPDispatcher_EncodesJSONBody(t *testing.T) {
	backend, rec := fakeBackend(t, http.StatusOK, `{}`)
	disp := &HTTPDispatcher{API: backend}

	body := map[string]any{"question": "hello", "datasource_id": "ds1"}
	_, err := disp.Dispatch(context.Background(), http.MethodPost, "/api/ai/query/run", body, Credential{}, ChannelMCP)
	if err != nil {
		t.Fatalf("dispatch: %v", err)
	}
	if rec.method != http.MethodPost {
		t.Errorf("method = %q, want POST", rec.method)
	}
	if rec.headers.Get("Content-Type") != "application/json" {
		t.Errorf("Content-Type = %q", rec.headers.Get("Content-Type"))
	}
	if rec.body == "" {
		t.Error("expected non-empty body")
	}
}

func TestHTTPDispatcher_NilBodyForGET(t *testing.T) {
	backend, rec := fakeBackend(t, http.StatusOK, `[]`)
	disp := &HTTPDispatcher{API: backend}

	_, err := disp.Dispatch(context.Background(), http.MethodGet, "/api/datasources", nil, Credential{}, ChannelMCP)
	if err != nil {
		t.Fatalf("dispatch: %v", err)
	}
	if rec.body != "" {
		t.Errorf("expected empty body for nil GET, got %q", rec.body)
	}
}

func TestHTTPDispatcher_Non2xxIsError(t *testing.T) {
	backend, _ := fakeBackend(t, http.StatusForbidden, `{"error":"denied"}`)
	disp := &HTTPDispatcher{API: backend}

	res, err := disp.Dispatch(context.Background(), http.MethodGet, "/api/datasources", nil, Credential{}, ChannelMCP)
	if err != nil {
		t.Fatalf("dispatch: %v", err)
	}
	if !res.IsError() {
		t.Errorf("expected IsError for 403, status=%d", res.StatusCode)
	}
	if res.StatusCode != http.StatusForbidden {
		t.Errorf("status = %d, want %d", res.StatusCode, http.StatusForbidden)
	}
	text := res.ErrorText()
	if text == "" {
		t.Error("expected non-empty ErrorText")
	}
}

func TestDispatchListModels_DatasourceFilterQuery(t *testing.T) {
	backend, rec := fakeBackend(t, http.StatusOK, "[]")
	disp := &HTTPDispatcher{API: backend}

	_, err := DispatchListModels(context.Background(), disp, ListModelsInput{DatasourceID: "ds 1"}, Credential{}, ChannelAgent)
	if err != nil {
		t.Fatalf("dispatch: %v", err)
	}
	if rec.path != "/api/semantic/models" {
		t.Errorf("path = %q", rec.path)
	}
	if rec.query != "datasource_id=ds+1&include=full" && rec.query != "include=full&datasource_id=ds+1" {
		t.Errorf("query = %q, want datasource_id + include=full", rec.query)
	}
	if got := rec.headers.Get("X-Biqly-Channel"); got != ChannelAgent {
		t.Errorf("channel = %q, want %q", got, ChannelAgent)
	}
}

func TestDispatchListModels_NoFilter(t *testing.T) {
	backend, rec := fakeBackend(t, http.StatusOK, "[]")
	disp := &HTTPDispatcher{API: backend}

	_, err := DispatchListModels(context.Background(), disp, ListModelsInput{}, Credential{}, ChannelMCP)
	if err != nil {
		t.Fatalf("dispatch: %v", err)
	}
	if rec.query != "include=full" {
		t.Errorf("query = %q, want include=full", rec.query)
	}
}

func TestDispatchListPromptTemplates_LocaleQuery(t *testing.T) {
	backend, rec := fakeBackend(t, http.StatusOK, "[]")
	disp := &HTTPDispatcher{API: backend}

	_, err := DispatchListPromptTemplates(context.Background(), disp, ListPromptTemplatesInput{Locale: "tr"}, Credential{}, ChannelMCP)
	if err != nil {
		t.Fatalf("dispatch: %v", err)
	}
	if rec.path != "/api/ai/prompt-templates/active" {
		t.Errorf("path = %q", rec.path)
	}
	if rec.query != "locale=tr" {
		t.Errorf("query = %q, want locale=tr", rec.query)
	}
}

func TestDispatchRunQuestion_BuildsCorrectBody(t *testing.T) {
	backend, rec := fakeBackend(t, http.StatusOK, `{}`)
	disp := &HTTPDispatcher{API: backend}

	in := RunQuestionInput{DatasourceID: "ds-1", Question: "how many orders", ModelID: "model-5"}
	_, err := DispatchRunQuestion(context.Background(), disp, in, Credential{Authorization: "Bearer tok"}, ChannelAgent)
	if err != nil {
		t.Fatalf("dispatch: %v", err)
	}
	if rec.method != http.MethodPost || rec.path != "/api/ai/query/run" {
		t.Errorf("dispatch = %s %s, want POST /api/ai/query/run", rec.method, rec.path)
	}
	var body map[string]any
	if err := sonic.Unmarshal([]byte(rec.body), &body); err != nil {
		t.Fatalf("unmarshal body: %v", err)
	}
	if body["datasource_id"] != "ds-1" {
		t.Errorf("datasource_id = %v", body["datasource_id"])
	}
	if body["question"] != "how many orders" {
		t.Errorf("question = %v", body["question"])
	}
	if body["model_id"] != "model-5" {
		t.Errorf("model_id = %v", body["model_id"])
	}
	if got := rec.headers.Get("Authorization"); got != "Bearer tok" {
		t.Errorf("Authorization = %q", got)
	}
}

func TestDispatchRunQuestion_OmitsEmptyModelID(t *testing.T) {
	backend, rec := fakeBackend(t, http.StatusOK, `{}`)
	disp := &HTTPDispatcher{API: backend}

	_, err := DispatchRunQuestion(context.Background(), disp, RunQuestionInput{DatasourceID: "ds", Question: "q"}, Credential{}, ChannelAgent)
	if err != nil {
		t.Fatalf("dispatch: %v", err)
	}
	var body map[string]any
	_ = sonic.Unmarshal([]byte(rec.body), &body)
	if _, exists := body["model_id"]; exists {
		t.Error("expected model_id to be omitted when empty")
	}
}

func TestDispatchRunLogicalQuery(t *testing.T) {
	backend, rec := fakeBackend(t, http.StatusOK, `{}`)
	disp := &HTTPDispatcher{API: backend}

	lq := map[string]any{"datasource_id": "ds", "select": []any{}}
	_, err := DispatchRunLogicalQuery(context.Background(), disp, RunLogicalQueryInput{LogicalQuery: lq}, Credential{}, ChannelAgent)
	if err != nil {
		t.Fatalf("dispatch: %v", err)
	}
	if rec.method != http.MethodPost || rec.path != "/api/query/run" {
		t.Errorf("dispatch = %s %s", rec.method, rec.path)
	}
	var body map[string]any
	_ = sonic.Unmarshal([]byte(rec.body), &body)
	if _, ok := body["logical_query"]; !ok {
		t.Error("expected logical_query in body")
	}
}

func TestDispatchRunLogicalQuery_InjectsDatasourceAndModelID(t *testing.T) {
	backend, rec := fakeBackend(t, http.StatusOK, `{}`)
	disp := &HTTPDispatcher{API: backend}

	_, err := DispatchRunLogicalQuery(context.Background(), disp, RunLogicalQueryInput{
		DatasourceID: "ds-from-input",
		ModelID:      "model-from-input",
		LogicalQuery: map[string]any{"select": []any{}},
	}, Credential{}, ChannelAgent)
	if err != nil {
		t.Fatalf("dispatch: %v", err)
	}
	var body map[string]any
	_ = sonic.Unmarshal([]byte(rec.body), &body)
	lq, ok := body["logical_query"].(map[string]any)
	if !ok {
		t.Fatalf("logical_query type = %T", body["logical_query"])
	}
	if lq["datasource_id"] != "ds-from-input" {
		t.Errorf("datasource_id = %v, want ds-from-input", lq["datasource_id"])
	}
	if lq["model_id"] != "model-from-input" {
		t.Errorf("model_id = %v, want model-from-input", lq["model_id"])
	}
}

func TestDispatchRunLogicalQuery_PreservesExistingModelID(t *testing.T) {
	backend, rec := fakeBackend(t, http.StatusOK, `{}`)
	disp := &HTTPDispatcher{API: backend}

	_, err := DispatchRunLogicalQuery(context.Background(), disp, RunLogicalQueryInput{
		ModelID: "model-from-input",
		LogicalQuery: map[string]any{
			"datasource_id": "ds",
			"model_id":      "model-from-lq",
			"select":        []any{},
		},
	}, Credential{}, ChannelAgent)
	if err != nil {
		t.Fatalf("dispatch: %v", err)
	}
	var body map[string]any
	_ = sonic.Unmarshal([]byte(rec.body), &body)
	lq, ok := body["logical_query"].(map[string]any)
	if !ok {
		t.Fatalf("logical_query type = %T", body["logical_query"])
	}
	if lq["model_id"] != "model-from-lq" {
		t.Errorf("model_id = %v, want model-from-lq (planner value wins)", lq["model_id"])
	}
}

func TestDispatchDryPlan_DispatchesToCompile(t *testing.T) {
	backend, rec := fakeBackend(t, http.StatusOK, `{}`)
	disp := &HTTPDispatcher{API: backend}

	lq := map[string]any{"datasource_id": "ds", "select": []any{}}
	_, err := DispatchDryPlan(context.Background(), disp, RunLogicalQueryInput{LogicalQuery: lq}, Credential{}, ChannelAgent)
	if err != nil {
		t.Fatalf("dispatch: %v", err)
	}
	if rec.method != http.MethodPost || rec.path != "/api/query/compile" {
		t.Errorf("dispatch = %s %s, want POST /api/query/compile", rec.method, rec.path)
	}
	var body map[string]any
	_ = sonic.Unmarshal([]byte(rec.body), &body)
	if _, ok := body["logical_query"]; !ok {
		t.Error("expected logical_query in body")
	}
}

func TestDispatchDryRun_DispatchesToDryRun(t *testing.T) {
	backend, rec := fakeBackend(t, http.StatusOK, `{}`)
	disp := &HTTPDispatcher{API: backend}

	lq := map[string]any{"datasource_id": "ds", "select": []any{}}
	_, err := DispatchDryRun(context.Background(), disp, RunLogicalQueryInput{LogicalQuery: lq}, Credential{}, ChannelAgent)
	if err != nil {
		t.Fatalf("dispatch: %v", err)
	}
	if rec.method != http.MethodPost || rec.path != "/api/query/dry-run" {
		t.Errorf("dispatch = %s %s, want POST /api/query/dry-run", rec.method, rec.path)
	}
	var body map[string]any
	_ = sonic.Unmarshal([]byte(rec.body), &body)
	if _, ok := body["logical_query"]; !ok {
		t.Error("expected logical_query in body")
	}
}

func TestDispatchMetricQuery_DispatchesToMetric(t *testing.T) {
	backend, rec := fakeBackend(t, http.StatusOK, `{}`)
	disp := &HTTPDispatcher{API: backend}

	_, err := DispatchMetricQuery(context.Background(), disp, MetricQueryInput{
		DatasourceID: "ds",
		ModelID:      "m",
		Measures:     []string{"revenue"},
		Dimensions:   []string{"region"},
		Limit:        25,
	}, Credential{}, ChannelAgent)
	if err != nil {
		t.Fatalf("dispatch: %v", err)
	}
	if rec.method != http.MethodPost || rec.path != "/api/query/metric" {
		t.Errorf("dispatch = %s %s, want POST /api/query/metric", rec.method, rec.path)
	}
	var body map[string]any
	_ = sonic.Unmarshal([]byte(rec.body), &body)
	if body["datasource_id"] != "ds" || body["model_id"] != "m" {
		t.Errorf("body = %+v", body)
	}
}

func TestDispatchListSkills_DatasourceFilter(t *testing.T) {
	backend, rec := fakeBackend(t, http.StatusOK, "[]")
	disp := &HTTPDispatcher{API: backend}

	_, err := DispatchListSkills(context.Background(), disp, ListSkillsInput{DatasourceID: "ds/safe"}, Credential{}, ChannelAgent)
	if err != nil {
		t.Fatalf("dispatch: %v", err)
	}
	if rec.path != "/api/ai/skills" {
		t.Errorf("path = %q", rec.path)
	}
	if rec.query != "datasource_id=ds%2Fsafe" {
		t.Errorf("query = %q, want datasource_id=ds%%2Fsafe (url-encoded)", rec.query)
	}
}

func TestDispatchRunSkill_PathEscapeAndParameters(t *testing.T) {
	backend, rec := fakeBackend(t, http.StatusOK, `{}`)
	disp := &HTTPDispatcher{API: backend}

	params := map[string]any{"region": "EU"}
	_, err := DispatchRunSkill(context.Background(), disp, RunSkillInput{SkillID: "skill/1", Parameters: params}, Credential{}, ChannelAgent)
	if err != nil {
		t.Fatalf("dispatch: %v", err)
	}
	if rec.method != http.MethodPost {
		t.Errorf("method = %q", rec.method)
	}
	// r.URL.Path decodes %2F back to / — the escape still protects the
	// request wire format; the router matches on the decoded segment.
	if rec.path != "/api/ai/skills/skill/1/run" {
		t.Errorf("path = %q", rec.path)
	}
	var body map[string]any
	_ = sonic.Unmarshal([]byte(rec.body), &body)
	if _, ok := body["parameters"]; !ok {
		t.Error("expected parameters in body")
	}
}

func TestDispatchRunSkill_NoParametersOmitsField(t *testing.T) {
	backend, rec := fakeBackend(t, http.StatusOK, `{}`)
	disp := &HTTPDispatcher{API: backend}

	_, err := DispatchRunSkill(context.Background(), disp, RunSkillInput{SkillID: "s1"}, Credential{}, ChannelAgent)
	if err != nil {
		t.Fatalf("dispatch: %v", err)
	}
	var body map[string]any
	_ = sonic.Unmarshal([]byte(rec.body), &body)
	if _, exists := body["parameters"]; exists {
		t.Error("expected parameters to be omitted when empty")
	}
}

func TestDispatchResult_IsError(t *testing.T) {
	tests := []struct {
		status int
		isErr  bool
	}{
		{http.StatusOK, false},
		{http.StatusCreated, false},
		{http.StatusBadRequest, true},
		{http.StatusForbidden, true},
		{http.StatusNotFound, true},
		{http.StatusInternalServerError, true},
	}
	for _, tt := range tests {
		r := DispatchResult{StatusCode: tt.status}
		if r.IsError() != tt.isErr {
			t.Errorf("status %d: IsError() = %v, want %v", tt.status, r.IsError(), tt.isErr)
		}
	}
}

// fakeDispatcher is a test double for Dispatcher that records calls without
// real HTTP, useful for higher-level tests that verify argument routing.
type fakeDispatcher struct {
	lastMethod  string
	lastPath    string
	lastBody    any
	lastCred    Credential
	lastChannel string
	result      DispatchResult
}

func (f *fakeDispatcher) Dispatch(_ context.Context, method, path string, body any, cred Credential, channel string) (DispatchResult, error) {
	f.lastMethod = method
	f.lastPath = path
	f.lastBody = body
	f.lastCred = cred
	f.lastChannel = channel
	return f.result, nil
}

func TestFakeDispatcher_TracksCalls(t *testing.T) {
	fd := &fakeDispatcher{result: DispatchResult{StatusCode: 200, Body: json.RawMessage(`{}`)}}
	cred := Credential{Authorization: "Bearer x"}

	_, err := DispatchListDatasources(context.Background(), fd, cred, ChannelAgent)
	if err != nil {
		t.Fatalf("dispatch: %v", err)
	}
	if fd.lastMethod != http.MethodGet {
		t.Errorf("method = %q, want GET", fd.lastMethod)
	}
	if fd.lastPath != "/api/datasources" {
		t.Errorf("path = %q", fd.lastPath)
	}
	if fd.lastCred != cred {
		t.Errorf("cred = %+v", fd.lastCred)
	}
	if fd.lastChannel != ChannelAgent {
		t.Errorf("channel = %q", fd.lastChannel)
	}
}
