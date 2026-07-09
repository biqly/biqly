package agent

import (
	"context"
	"encoding/json"
	"fmt"
)

// DecisionKind is the variant a PlannerDecision carries. Exactly one is set.
type DecisionKind string

const (
	DecisionTool          DecisionKind = "tool"
	DecisionClarification DecisionKind = "clarification"
	DecisionFinal         DecisionKind = "final"
	DecisionFail          DecisionKind = "fail"
)

// Clarification asks the user to disambiguate before the run continues.
type Clarification struct {
	Question string   `json:"question"`
	Options  []string `json:"options,omitempty"`
}

// ClarificationExchange carries one clarification round's question and the
// user's answer into the planner's next Decide call, right after a resumed
// run continues past a clarification_required pause. RunContext.
// ClarificationHistory carries the full accumulated list of these — every
// round resolved so far, oldest first — across a multi-round resume chain,
// not just the most recent one; PolicyEngine does not use it — only the
// planner prompt (provider_planner.go), so the model doesn't re-ask a
// question it already has an answer for.
type ClarificationExchange struct {
	Question string
	Answer   string
}

// FinalResponse is a run's successful terminal outcome.
type FinalResponse struct {
	Answer     string  `json:"answer"`
	Confidence float64 `json:"confidence"`
}

// Failure is a run's unsuccessful terminal outcome.
type Failure struct {
	ReasonCode string `json:"reason_code"`
	Message    string `json:"message"`
}

// PlannerDecision is the planner's next move. Exactly one of Proposal /
// Clarification / Final / Failure is set, matching Kind.
type PlannerDecision struct {
	Kind          DecisionKind
	Proposal      *Proposal
	Clarification *Clarification
	Final         *FinalResponse
	Failure       *Failure
}

// Planner produces the next decision given a run and its step history so
// far — including steps policy denied or that errored, not just successful
// observations, so the planner can see a denial and propose something else
// instead of blindly repeating it.
type Planner interface {
	Decide(ctx context.Context, run RunContext, history []RuntimeStep) (PlannerDecision, error)
}

// toolProposalWire is the wire shape of a tool decision inside the strict
// planner envelope.
type toolProposalWire struct {
	Name      ToolName        `json:"name"`
	Arguments json.RawMessage `json:"arguments"`
}

// plannerDecisionEnvelope is the strict wire shape a planner provider must
// emit: exactly one of tool/clarification/final/fail set, nothing else.
type plannerDecisionEnvelope struct {
	Tool          *toolProposalWire `json:"tool,omitempty"`
	Clarification *Clarification    `json:"clarification,omitempty"`
	Final         *FinalResponse    `json:"final,omitempty"`
	Fail          *Failure          `json:"fail,omitempty"`
}

// DecodePlannerDecision strictly decodes a planner provider's raw JSON
// output into exactly one PlannerDecision variant. Unknown fields, zero
// variants set, and more than one variant set are all rejected.
func DecodePlannerDecision(raw json.RawMessage) (PlannerDecision, error) {
	var env plannerDecisionEnvelope
	if err := strictDecode(raw, &env); err != nil {
		return PlannerDecision{}, fmt.Errorf("decode planner decision: %w", err)
	}

	var decision PlannerDecision
	set := 0
	if env.Tool != nil {
		set++
		decision = PlannerDecision{
			Kind:     DecisionTool,
			Proposal: &Proposal{Tool: env.Tool.Name, Arguments: env.Tool.Arguments},
		}
	}
	if env.Clarification != nil {
		set++
		decision = PlannerDecision{Kind: DecisionClarification, Clarification: env.Clarification}
	}
	if env.Final != nil {
		set++
		decision = PlannerDecision{Kind: DecisionFinal, Final: env.Final}
	}
	if env.Fail != nil {
		set++
		decision = PlannerDecision{Kind: DecisionFail, Failure: env.Fail}
	}
	if set != 1 {
		return PlannerDecision{}, fmt.Errorf(
			"planner decision must set exactly one of tool/clarification/final/fail, got %d", set)
	}
	return decision, nil
}
