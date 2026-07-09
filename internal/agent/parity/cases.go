package parity

import (
	"fmt"

	"github.com/bytedance/sonic"

	"github.com/biqly/biqly/internal/query"
	"github.com/biqly/biqly/internal/toolcontract"
)

// Fixed fixture IDs shared between Cases and NewFakeBackend.
const (
	DatasourceOrders = "ds-1"
	ModelOrders      = "orders"
	SkillTopCountry  = "skill-top-country"
)

// caseCount is the number of cases Cases returns — kept in sync with the
// literal case list below so the result slice can be preallocated.
const caseCount = 8

// Case is one fixed question/tool-call run through both governed paths.
//
// MCPTool/MCPArgs is what an external MCP client — which has no planner and
// decides tool calls itself — would naturally send: either a single direct
// call (run_logical_query, run_skill) or a bare question with model
// selection left to the shared backend (run_question omits model_id).
//
// AgentScript is the scripted (non-LLM) planner-decision sequence the web
// agent's Runtime replays via a stub provider — see the package doc comment
// for why this is a scripted script rather than a real LLM call. Each entry
// is one strict planner-decision envelope (internal/agent.DecodePlannerDecision
// shape); the last entry is always a "final" decision.
type Case struct {
	ID           string
	Question     string
	DatasourceID string
	MCPTool      toolcontract.ToolName
	MCPArgs      map[string]any
	AgentScript  []string
	// CompareTool names the tool whose observation the agent path should
	// extract for comparison against the MCP call above (the web agent may
	// call other tools first, e.g. list_models, before the comparable one).
	CompareTool toolcontract.ToolName
}

// toolStep and finalStep build one planner-decision JSON envelope each
// (internal/agent.DecodePlannerDecision's wire shape). They return an error
// rather than panicking so a hypothetical future non-JSON-safe argument value
// fails Cases() loudly instead of crashing the process — in practice every
// caller below passes static, JSON-safe literals.
func toolStep(name toolcontract.ToolName, args map[string]any) (string, error) {
	raw, err := sonic.Marshal(map[string]any{
		"tool": map[string]any{"name": string(name), "arguments": args},
	})
	if err != nil {
		return "", fmt.Errorf("build tool step for %s: %w", name, err)
	}
	return string(raw), nil
}

func finalStep(answer string) (string, error) {
	raw, err := sonic.Marshal(map[string]any{
		"final": map[string]any{"answer": answer, "confidence": 0.9},
	})
	if err != nil {
		return "", fmt.Errorf("build final step: %w", err)
	}
	return string(raw), nil
}

// lqArgs converts a LogicalQuery into the map[string]any wire shape the
// governed tool contract expects for run_logical_query's "logical_query"
// argument (toolcontract.RunLogicalQueryInput.LogicalQuery).
func lqArgs(lq query.LogicalQuery) (map[string]any, error) {
	raw, err := sonic.Marshal(lq)
	if err != nil {
		return nil, fmt.Errorf("marshal logical query: %w", err)
	}
	var m map[string]any
	if err := sonic.Unmarshal(raw, &m); err != nil {
		return nil, fmt.Errorf("unmarshal logical query to map: %w", err)
	}
	return m, nil
}

// goldenLogicalQueries maps each run_question case's exact question text to
// the LogicalQuery the fake backend (NewFakeBackend) deterministically
// returns for it — standing in for the real ai.Service NL routing this
// harness does not exercise (see package doc comment). Two of the four
// questions and their expected shapes are borrowed verbatim from
// internal/ai/eval.DefaultGoldenCases (count-orders, orders-by-country) to
// stay consistent with this repo's existing golden question set rather than
// inventing unrelated fixtures; the other two are direct English/Turkish
// analogues of that same set's shipped-orders and status-breakdown cases.
var goldenLogicalQueries = map[string]query.LogicalQuery{
	"kaç sipariş var": {
		Select: []query.SelectItem{{Type: "metric", Name: "row_count"}},
		Limit:  100,
	},
	"how many orders are there": {
		Select: []query.SelectItem{{Type: "metric", Name: "row_count"}},
		Limit:  100,
	},
	"ülkeye göre sipariş sayısı": {
		Select:  []query.SelectItem{{Type: "dimension", Name: "country"}, {Type: "metric", Name: "row_count"}},
		GroupBy: []query.GroupBy{{Field: "country"}},
		Limit:   100,
	},
	"order count by status": {
		Select:  []query.SelectItem{{Type: "dimension", Name: "status"}, {Type: "metric", Name: "row_count"}},
		GroupBy: []query.GroupBy{{Field: "status"}},
		Limit:   100,
	},
}

// cancelledOrdersTotal is the run_logical_query case's fixed query — both
// paths send this exact document, so it exercises the shared dispatch
// contract directly rather than any auto-selection logic. Borrowed from
// internal/ai/eval.BenchmarkCases' "bench-cancelled-total".
func cancelledOrdersTotal() query.LogicalQuery {
	return query.LogicalQuery{
		DatasourceID: DatasourceOrders,
		ModelID:      ModelOrders,
		Select:       []query.SelectItem{{Type: "metric", Name: "total_amount"}},
		Filters:      []query.Filter{{Field: "status", Operator: "eq", Value: "cancelled"}},
		Limit:        100,
	}
}

// Cases returns the fixed parity question set: 4 run_question cases
// (bilingual, exercising model auto-selection), 1 run_logical_query case
// (direct dispatch-contract equivalence), 1 list_datasources case,
// 1 list_models case, and 1 list_skills+run_skill case — covering all six
// governed tools. An error can only occur if one of this function's own
// static literals is not JSON-safe, which every caller here already is.
func Cases() ([]Case, error) {
	cases := make([]Case, 0, caseCount)

	listDatasources, err := newListDatasourcesCase()
	if err != nil {
		return nil, err
	}
	listModels, err := newListModelsCase()
	if err != nil {
		return nil, err
	}
	cases = append(cases, listDatasources, listModels)

	for _, q := range []string{
		"kaç sipariş var",
		"how many orders are there",
		"ülkeye göre sipariş sayısı",
		"order count by status",
	} {
		c, err := runQuestionCase(q)
		if err != nil {
			return nil, err
		}
		cases = append(cases, c)
	}

	runLogicalQuery, err := newRunLogicalQueryCase()
	if err != nil {
		return nil, err
	}
	runSkill, err := newRunSkillCase()
	if err != nil {
		return nil, err
	}
	cases = append(cases, runLogicalQuery, runSkill)

	return cases, nil
}

func newListDatasourcesCase() (Case, error) {
	list, err := toolStep(toolcontract.ToolListDatasources, map[string]any{})
	if err != nil {
		return Case{}, err
	}
	final, err := finalStep("Listed accessible datasources.")
	if err != nil {
		return Case{}, err
	}
	return Case{
		ID:          "list-datasources",
		Question:    "hangi veri kaynaklarına erişimim var",
		MCPTool:     toolcontract.ToolListDatasources,
		MCPArgs:     map[string]any{},
		AgentScript: []string{list, final},
		CompareTool: toolcontract.ToolListDatasources,
	}, nil
}

func newListModelsCase() (Case, error) {
	list, err := toolStep(toolcontract.ToolListModels, map[string]any{"datasource_id": DatasourceOrders})
	if err != nil {
		return Case{}, err
	}
	final, err := finalStep("Listed models for ds-1.")
	if err != nil {
		return Case{}, err
	}
	return Case{
		ID:           "list-models-orders",
		Question:     "ds-1 için hangi modeller var",
		DatasourceID: DatasourceOrders,
		MCPTool:      toolcontract.ToolListModels,
		MCPArgs:      map[string]any{"datasource_id": DatasourceOrders},
		AgentScript:  []string{list, final},
		CompareTool:  toolcontract.ToolListModels,
	}, nil
}

func newRunLogicalQueryCase() (Case, error) {
	args, err := lqArgs(cancelledOrdersTotal())
	if err != nil {
		return Case{}, err
	}
	step, err := toolStep(toolcontract.ToolRunLogicalQuery, map[string]any{"logical_query": args})
	if err != nil {
		return Case{}, err
	}
	final, err := finalStep("Cancelled order total is ready.")
	if err != nil {
		return Case{}, err
	}
	return Case{
		ID:           "run-logical-query-cancelled-total",
		Question:     "iptal edilen siparişlerin tutarı (direct LogicalQuery)",
		DatasourceID: DatasourceOrders,
		MCPTool:      toolcontract.ToolRunLogicalQuery,
		MCPArgs:      map[string]any{"logical_query": args},
		AgentScript:  []string{step, final},
		CompareTool:  toolcontract.ToolRunLogicalQuery,
	}, nil
}

func newRunSkillCase() (Case, error) {
	listSkills, err := toolStep(toolcontract.ToolListSkills, map[string]any{"datasource_id": DatasourceOrders})
	if err != nil {
		return Case{}, err
	}
	runSkill, err := toolStep(toolcontract.ToolRunSkill, map[string]any{"skill_id": SkillTopCountry})
	if err != nil {
		return Case{}, err
	}
	final, err := finalStep("Top country by revenue is ready.")
	if err != nil {
		return Case{}, err
	}
	return Case{
		ID:           "list-and-run-skill-top-country",
		Question:     "en yüksek tutarlı ülke hangisi (saved skill)",
		DatasourceID: DatasourceOrders,
		MCPTool:      toolcontract.ToolRunSkill,
		MCPArgs:      map[string]any{"skill_id": SkillTopCountry},
		AgentScript:  []string{listSkills, runSkill, final},
		CompareTool:  toolcontract.ToolRunSkill,
	}, nil
}

// runQuestionCase builds a run_question case for question: the MCP side
// calls run_question directly with no model_id (letting the shared backend
// auto-select — see NewFakeBackend), and the agent side scripts
// list_models -> run_question with the model_id the (stubbed) planner reads
// off that list, so a genuine "did both paths land on the same model and the
// same LogicalQuery" comparison happens even though the two paths took
// different routes to get there.
func runQuestionCase(question string) (Case, error) {
	listModels, err := toolStep(toolcontract.ToolListModels, map[string]any{"datasource_id": DatasourceOrders})
	if err != nil {
		return Case{}, err
	}
	runQuestion, err := toolStep(toolcontract.ToolRunQuestion, map[string]any{
		"datasource_id": DatasourceOrders, "question": question, "model_id": ModelOrders,
	})
	if err != nil {
		return Case{}, err
	}
	final, err := finalStep("Here is the answer.")
	if err != nil {
		return Case{}, err
	}
	return Case{
		ID:           "run-question-" + slug(question),
		Question:     question,
		DatasourceID: DatasourceOrders,
		MCPTool:      toolcontract.ToolRunQuestion,
		MCPArgs:      map[string]any{"datasource_id": DatasourceOrders, "question": question},
		AgentScript:  []string{listModels, runQuestion, final},
		CompareTool:  toolcontract.ToolRunQuestion,
	}, nil
}

// slug builds a short, stable, human-readable case ID suffix from a
// question — good enough for a fixed, small, ASCII-and-Turkish case set;
// not a general-purpose slugifier.
func slug(s string) string {
	out := make([]rune, 0, len(s))
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			out = append(out, r)
		case r >= 'A' && r <= 'Z':
			out = append(out, r+('a'-'A'))
		case r == ' ':
			out = append(out, '-')
		}
	}
	return string(out)
}
