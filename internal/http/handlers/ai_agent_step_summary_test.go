package handlers

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/biqly/biqly/internal/agent"
)

// TestWebAgentStepSummary is the table test for the tool-aware step
// summarizer: completed steps get counts/names/durations ONLY (never raw
// rows), denied steps keep the bare reason code (so RunTrace.tsx's
// reason-code i18n mapping still fires), errored steps keep the error text.
func TestWebAgentStepSummary(t *testing.T) {
	obs := func(tool agent.ToolName, payload string) *agent.Observation {
		return &agent.Observation{Tool: tool, Payload: []byte(payload)}
	}
	cases := []struct {
		name string
		step agent.RuntimeStep
		want string
	}{
		{
			name: "denied step keeps the bare reason code",
			step: agent.RuntimeStep{
				Proposal:     agent.Proposal{Tool: agent.ToolWebRunLogicalQuery},
				DeniedReason: "hidden_column_denied",
			},
			want: "hidden_column_denied",
		},
		{
			name: "errored step keeps the error text",
			step: agent.RuntimeStep{
				Proposal: agent.Proposal{Tool: agent.ToolWebRunQuestion},
				Error:    "upstream timeout",
			},
			want: "upstream timeout",
		},
		{
			name: "in-flight step (no observation) has no summary",
			step: agent.RuntimeStep{Proposal: agent.Proposal{Tool: agent.ToolWebListModels}},
			want: "",
		},
		{
			name: "list_models counts and names from a bare array",
			step: agent.RuntimeStep{
				Proposal:    agent.Proposal{Tool: agent.ToolWebListModels},
				Observation: obs(agent.ToolWebListModels, `[{"name":"Revenue"},{"name":"Orders"}]`),
			},
			want: "2 models: Revenue, Orders",
		},
		{
			name: "list_models tolerates a wrapping object keyed by the noun",
			step: agent.RuntimeStep{
				Proposal:    agent.Proposal{Tool: agent.ToolWebListModels},
				Observation: obs(agent.ToolWebListModels, `{"models":[{"name":"Revenue"}]}`),
			},
			want: "1 models: Revenue",
		},
		{
			name: "list_datasources shows at most three names then an ellipsis",
			step: agent.RuntimeStep{
				Proposal: agent.Proposal{Tool: agent.ToolWebListDatasources},
				Observation: obs(agent.ToolWebListDatasources,
					`[{"name":"a"},{"name":"b"},{"name":"c"},{"name":"d"}]`),
			},
			want: "4 datasources: a, b, c, …",
		},
		{
			name: "list_skills unwraps the skills endpoint's {skills:[...]} shape",
			step: agent.RuntimeStep{
				Proposal:    agent.Proposal{Tool: agent.ToolWebListSkills},
				Observation: obs(agent.ToolWebListSkills, `{"skills":[{"name":"Monthly Revenue"}]}`),
			},
			want: "1 skills: Monthly Revenue",
		},
		{
			name: "list_models falls back to label when name is empty",
			step: agent.RuntimeStep{
				Proposal:    agent.Proposal{Tool: agent.ToolWebListModels},
				Observation: obs(agent.ToolWebListModels, `[{"name":"","label":"Gelir"}]`),
			},
			want: "1 models: Gelir",
		},
		{
			name: "run_question summarizes row count and duration, never rows",
			step: agent.RuntimeStep{
				Proposal: agent.Proposal{Tool: agent.ToolWebRunQuestion},
				Observation: obs(agent.ToolWebRunQuestion,
					`{"result":{"result":{"columns":[{"name":"c"}],"rows":[["secret"]],"stats":{"row_count":43,"duration_ms":1887}}}}`),
			},
			want: "43 rows in 1887ms",
		},
		{
			name: "run_logical_query summarizes the bare query result",
			step: agent.RuntimeStep{
				Proposal: agent.Proposal{Tool: agent.ToolWebRunLogicalQuery},
				Observation: obs(agent.ToolWebRunLogicalQuery,
					`{"columns":[{"name":"c"}],"rows":[],"stats":{"row_count":7,"duration_ms":12}}`),
			},
			want: "7 rows in 12ms",
		},
		{
			name: "run_skill names the skill and counts rows",
			step: agent.RuntimeStep{
				Proposal: agent.Proposal{Tool: agent.ToolWebRunSkill},
				Observation: obs(agent.ToolWebRunSkill,
					`{"skill_id":"s1","name":"Monthly Revenue","result":{"columns":[],"rows":[],"stats":{"row_count":12,"duration_ms":90}}}`),
			},
			want: `skill "Monthly Revenue": 12 rows`,
		},
		{
			name: "malformed payload yields an empty summary rather than an error",
			step: agent.RuntimeStep{
				Proposal:    agent.Proposal{Tool: agent.ToolWebRunQuestion},
				Observation: obs(agent.ToolWebRunQuestion, `not json`),
			},
			want: "",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, webAgentStepSummary(tc.step))
		})
	}
}

func TestWebAgentStepSummaryTruncatesLongErrors(t *testing.T) {
	long := strings.Repeat("x", 300)
	got := webAgentStepSummary(agent.RuntimeStep{
		Proposal: agent.Proposal{Tool: agent.ToolWebRunQuestion},
		Error:    long,
	})
	assert.Equal(t, webAgentSummaryMaxRunes+1, len([]rune(got)), "200 runes + ellipsis")
	assert.True(t, strings.HasSuffix(got, "…"))
}

func TestWebAgentStepArgs(t *testing.T) {
	assert.Empty(t, webAgentStepArgs(agent.RuntimeStep{}))
	assert.Empty(t, webAgentStepArgs(agent.RuntimeStep{
		Proposal: agent.Proposal{Arguments: []byte(`{}`)}}))
	assert.Empty(t, webAgentStepArgs(agent.RuntimeStep{
		Proposal: agent.Proposal{Arguments: []byte(`null`)}}))
	assert.Equal(t, `{"datasource_id":"ds-1"}`, webAgentStepArgs(agent.RuntimeStep{
		Proposal: agent.Proposal{Arguments: []byte(`{"datasource_id":"ds-1"}`)}}))

	long := `{"question":"` + strings.Repeat("a", 400) + `"}`
	got := webAgentStepArgs(agent.RuntimeStep{Proposal: agent.Proposal{Arguments: []byte(long)}})
	assert.Equal(t, webAgentSummaryMaxRunes+1, len([]rune(got)))
}

func TestWebAgentClarificationDetail(t *testing.T) {
	got := webAgentClarificationDetail(agent.ClarificationExchange{
		Question: "Which datasource?",
		Answer:   "zlitter",
	})
	assert.Equal(t, "asked: Which datasource? — answered: zlitter", got)
}
