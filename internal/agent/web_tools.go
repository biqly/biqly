package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/bytedance/sonic"

	"github.com/biqly/biqly/internal/toolcontract"
)

// WebTools bundles the six MCP-parity web agent tools and the dispatcher they
// share. The web handler (T6) builds one per request from the caller's
// credentials and registers them in a Registry.
type WebTools struct {
	disp toolcontract.Dispatcher
	cred toolcontract.Credential
}

// NewWebTools creates a WebTools set backed by disp, forwarding cred on every
// loopback call with channel=agent.
func NewWebTools(disp toolcontract.Dispatcher, cred toolcontract.Credential) *WebTools {
	return &WebTools{disp: disp, cred: cred}
}

// All returns the six web tool adapters, ready to register in a Registry.
func (w *WebTools) All() []Tool {
	return []Tool{
		webTool{w, ToolWebListDatasources, webListDatasources},
		webTool{w, ToolWebListModels, webListModels},
		webTool{w, ToolWebRunQuestion, webRunQuestion},
		webTool{w, ToolWebRunLogicalQuery, webRunLogicalQuery},
		webTool{w, ToolWebListSkills, webListSkills},
		webTool{w, ToolWebRunSkill, webRunSkill},
	}
}

// webTool is a generic adapter that implements Tool for any web dispatch
// function, eliminating per-tool duplication. The dispatch fn receives the
// caller's credential, channel, and run context, performs the loopback call,
// and returns the raw response.
type webTool struct {
	w        *WebTools
	name     ToolName
	dispatch func(ctx context.Context, disp toolcontract.Dispatcher, cred toolcontract.Credential, run RunContext, args json.RawMessage) (toolcontract.DispatchResult, error)
}

func (t webTool) Name() ToolName { return t.name }

func (t webTool) Execute(ctx context.Context, run RunContext, args json.RawMessage) (Observation, error) {
	res, err := t.dispatch(ctx, t.w.disp, t.w.cred, run, args)
	if err != nil {
		return Observation{}, err
	}
	if res.IsError() {
		return Observation{}, fmt.Errorf("%s: %s", t.name, res.ErrorText())
	}
	return Observation{Tool: t.name, Payload: truncateForPlanner(res.Body)}, nil
}

// maxPlannerRows caps how many result rows are visible to the planner in the
// observation payload. The full governed result (row-limited by the query path)
// goes to the client via the finalizer; the planner only needs enough to decide
// the next step.
const maxPlannerRows = 100

// truncateForPlanner trims a JSON tool response to keep the planner's context
// window bounded: if the response is a JSON object with a "rows" array, it caps
// the array at maxPlannerRows; otherwise it truncates the raw string at a
// generous rune limit. The original governed result is preserved in the SSE
// final payload — this only affects planner visibility.
func truncateForPlanner(raw json.RawMessage) json.RawMessage {
	if len(raw) == 0 {
		return raw
	}
	var obj map[string]json.RawMessage
	if err := sonic.Unmarshal(raw, &obj); err != nil {
		return truncateRunes(raw, 5000)
	}
	rowsRaw, ok := obj["rows"]
	if !ok {
		return raw
	}
	var rows []json.RawMessage
	if err := sonic.Unmarshal(rowsRaw, &rows); err != nil {
		return raw
	}
	if len(rows) <= maxPlannerRows {
		return raw
	}
	truncated, err := sonic.Marshal(rows[:maxPlannerRows])
	if err != nil {
		return raw
	}
	obj["rows"] = truncated
	obj["rows_truncated"] = json.RawMessage(strconv.Itoa(len(rows) - maxPlannerRows))
	out, err := sonic.Marshal(obj)
	if err != nil {
		return raw
	}
	return out
}

func truncateRunes(raw json.RawMessage, maxRunes int) json.RawMessage {
	s := string(raw)
	if len([]rune(s)) <= maxRunes {
		return raw
	}
	runes := []rune(s)
	truncated := string(runes[:maxRunes])
	return json.RawMessage(truncated + `,"_truncated":true}`)
}

// --- Per-tool dispatch functions ---

func webListDatasources(ctx context.Context, disp toolcontract.Dispatcher, cred toolcontract.Credential, _ RunContext, _ json.RawMessage) (toolcontract.DispatchResult, error) {
	return toolcontract.DispatchListDatasources(ctx, disp, cred, toolcontract.ChannelAgent)
}

func webListModels(ctx context.Context, disp toolcontract.Dispatcher, cred toolcontract.Credential, _ RunContext, args json.RawMessage) (toolcontract.DispatchResult, error) {
	in, err := decodeArgs[toolcontract.ListModelsInput](args)
	if err != nil {
		return toolcontract.DispatchResult{}, err
	}
	return toolcontract.DispatchListModels(ctx, disp, in, cred, toolcontract.ChannelAgent)
}

func webRunQuestion(ctx context.Context, disp toolcontract.Dispatcher, cred toolcontract.Credential, _ RunContext, args json.RawMessage) (toolcontract.DispatchResult, error) {
	in, err := decodeArgs[toolcontract.RunQuestionInput](args)
	if err != nil {
		return toolcontract.DispatchResult{}, err
	}
	return toolcontract.DispatchRunQuestion(ctx, disp, in, cred, toolcontract.ChannelAgent)
}

func webRunLogicalQuery(ctx context.Context, disp toolcontract.Dispatcher, cred toolcontract.Credential, run RunContext, args json.RawMessage) (toolcontract.DispatchResult, error) {
	in, err := decodeArgs[toolcontract.RunLogicalQueryInput](args)
	if err != nil {
		return toolcontract.DispatchResult{}, err
	}
	// Inject the datasource_id from RunContext when the planner omitted it
	// from the tool arguments (web tools skip identity validation — the
	// datasource context is held by RunContext instead).
	if in.DatasourceID == "" {
		in.DatasourceID = run.DatasourceID
	}
	return toolcontract.DispatchRunLogicalQuery(ctx, disp, in, cred, toolcontract.ChannelAgent)
}

func webListSkills(ctx context.Context, disp toolcontract.Dispatcher, cred toolcontract.Credential, _ RunContext, args json.RawMessage) (toolcontract.DispatchResult, error) {
	in, err := decodeArgs[toolcontract.ListSkillsInput](args)
	if err != nil {
		return toolcontract.DispatchResult{}, err
	}
	return toolcontract.DispatchListSkills(ctx, disp, in, cred, toolcontract.ChannelAgent)
}

func webRunSkill(ctx context.Context, disp toolcontract.Dispatcher, cred toolcontract.Credential, _ RunContext, args json.RawMessage) (toolcontract.DispatchResult, error) {
	in, err := decodeArgs[toolcontract.RunSkillInput](args)
	if err != nil {
		return toolcontract.DispatchResult{}, err
	}
	return toolcontract.DispatchRunSkill(ctx, disp, in, cred, toolcontract.ChannelAgent)
}

// decodeArgs unmarshals JSON arguments into a typed input struct, tolerating
// empty input (the planner may send "{}" or null for no-arg tools).
func decodeArgs[T any](raw json.RawMessage) (T, error) {
	var zero T
	if len(raw) == 0 || string(raw) == "null" {
		return zero, nil
	}
	var in T
	if err := sonic.Unmarshal(raw, &in); err != nil {
		return zero, fmt.Errorf("decode %T: %w", in, err)
	}
	return in, nil
}
