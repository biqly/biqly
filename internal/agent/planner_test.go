package agent

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDecodePlannerDecisionTool(t *testing.T) {
	decision, err := DecodePlannerDecision([]byte(`{"tool":{"name":"catalog.resolve","arguments":{"a":1}}}`))
	require.NoError(t, err)
	assert.Equal(t, DecisionTool, decision.Kind)
	require.NotNil(t, decision.Proposal)
	assert.Equal(t, ToolCatalog, decision.Proposal.Tool)
	assert.JSONEq(t, `{"a":1}`, string(decision.Proposal.Arguments))
}

func TestDecodePlannerDecisionClarification(t *testing.T) {
	decision, err := DecodePlannerDecision([]byte(`{"clarification":{"question":"which metric?","options":["revenue","cost"]}}`))
	require.NoError(t, err)
	assert.Equal(t, DecisionClarification, decision.Kind)
	require.NotNil(t, decision.Clarification)
	assert.Equal(t, "which metric?", decision.Clarification.Question)
	assert.Equal(t, []string{"revenue", "cost"}, decision.Clarification.Options)
}

func TestDecodePlannerDecisionFinal(t *testing.T) {
	decision, err := DecodePlannerDecision([]byte(`{"final":{"answer":"42","confidence":0.9}}`))
	require.NoError(t, err)
	assert.Equal(t, DecisionFinal, decision.Kind)
	require.NotNil(t, decision.Final)
	assert.Equal(t, "42", decision.Final.Answer)
	assert.Equal(t, 0.9, decision.Final.Confidence)
}

func TestDecodePlannerDecisionFail(t *testing.T) {
	decision, err := DecodePlannerDecision([]byte(`{"fail":{"reason_code":"unresolvable","message":"no data"}}`))
	require.NoError(t, err)
	assert.Equal(t, DecisionFail, decision.Kind)
	require.NotNil(t, decision.Failure)
	assert.Equal(t, "unresolvable", decision.Failure.ReasonCode)
}

func TestDecodePlannerDecisionRejectsNoVariant(t *testing.T) {
	_, err := DecodePlannerDecision([]byte(`{}`))
	assert.Error(t, err)
}

func TestDecodePlannerDecisionRejectsMixedVariants(t *testing.T) {
	_, err := DecodePlannerDecision([]byte(
		`{"final":{"answer":"42","confidence":0.9},"clarification":{"question":"which?"}}`))
	assert.Error(t, err)
}

func TestDecodePlannerDecisionRejectsUnknownField(t *testing.T) {
	_, err := DecodePlannerDecision([]byte(`{"final":{"answer":"42","confidence":0.9},"debug_notes":"scratch"}`))
	assert.Error(t, err)
}

func TestDecodePlannerDecisionRejectsMalformedJSON(t *testing.T) {
	_, err := DecodePlannerDecision([]byte(`not json`))
	assert.Error(t, err)
}
