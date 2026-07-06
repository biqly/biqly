package agent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/biqly/biqly/internal/ai/jsonextract"
	providerpkg "github.com/biqly/biqly/internal/ai/provider"
	"github.com/biqly/biqly/internal/platform/observability"
)

// ErrPlannerResponseHasNoJSON is returned when the provider's completion
// contains no extractable JSON object for DecodePlannerDecision to decode.
var ErrPlannerResponseHasNoJSON = errors.New("planner response contained no JSON object")

// ProviderPlanner is a Planner backed by a generic LLM provider. It builds a
// deterministic prompt from the run and its step history, asks the
// provider for the strict planner-decision envelope (see planner.go), and
// decodes it via DecodePlannerDecision — so every rule that envelope
// enforces (exactly one variant, no unknown fields) applies to real
// provider output the same way it applies in tests.
type ProviderPlanner struct {
	provider providerpkg.Provider
	metrics  *observability.Metrics
}

// NewProviderPlanner builds a ProviderPlanner backed by provider.
func NewProviderPlanner(provider providerpkg.Provider) *ProviderPlanner {
	return &ProviderPlanner{provider: provider}
}

// SetMetrics wires m as the destination for this ProviderPlanner's token
// usage metrics. Optional — a nil (or never-set) recorder is a no-op.
func (p *ProviderPlanner) SetMetrics(m *observability.Metrics) {
	p.metrics = m
}

// Decide implements Planner.
func (p *ProviderPlanner) Decide(ctx context.Context, run RunContext, history []RuntimeStep) (PlannerDecision, error) {
	result, err := p.provider.Generate(ctx, buildPlannerPrompt(run, history))
	if err != nil {
		return PlannerDecision{}, fmt.Errorf("planner generate: %w", err)
	}
	if result.Usage != nil {
		p.metrics.RecordAgentPlannerTokens(result.Usage.Prompt, result.Usage.Completion)
	}
	obj, ok := jsonextract.ExtractJSONObject(result.Content)
	if !ok {
		return PlannerDecision{}, ErrPlannerResponseHasNoJSON
	}
	return DecodePlannerDecision(json.RawMessage(obj))
}

// buildPlannerPrompt renders the run's question, its allowed tools, and its
// step history so far into the fixed instructions the provider must follow
// to emit a valid planner-decision envelope.
func buildPlannerPrompt(run RunContext, history []RuntimeStep) string {
	var b strings.Builder
	b.WriteString("You are the planner for a governed BI query agent. ")
	b.WriteString("Respond with exactly one JSON object with exactly one of these top-level keys: ")
	b.WriteString(`"tool", "clarification", "final", "fail". `)
	b.WriteString(`"tool" is {"name": "<tool>", "arguments": {...}}. `)
	b.WriteString(`"clarification" is {"question": "...", "options": [...]}. `)
	b.WriteString(`"final" is {"answer": "...", "confidence": 0-1}. `)
	b.WriteString(`"fail" is {"reason_code": "...", "message": "..."}. `)
	b.WriteString("No other keys, no prose outside the JSON object.\n\n")

	fmt.Fprintf(&b, "Question: %s\n", run.Question)
	b.WriteString("Available tools: ")
	for i, tool := range run.AllowedTools {
		if i > 0 {
			b.WriteString(", ")
		}
		b.WriteString(string(tool))
	}
	b.WriteString("\n")

	if len(history) == 0 {
		b.WriteString("No steps taken yet.\n")
		return b.String()
	}
	b.WriteString("Steps so far:\n")
	for _, step := range history {
		b.WriteString(describeStep(step))
	}
	return b.String()
}

func describeStep(step RuntimeStep) string {
	var b strings.Builder
	fmt.Fprintf(&b, "%d. tool=%s", step.Seq, step.Proposal.Tool)
	switch {
	case step.DeniedReason != "":
		fmt.Fprintf(&b, " DENIED reason=%s (propose something else)\n", step.DeniedReason)
	case step.Error != "":
		fmt.Fprintf(&b, " ERROR=%s\n", step.Error)
	case step.Observation != nil:
		fmt.Fprintf(&b, " observation=%s\n", truncate(string(step.Observation.Payload), 500))
	default:
		b.WriteString(" (pending)\n")
	}
	return b.String()
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "...(" + strconv.Itoa(len(s)-maxLen) + " more bytes truncated)"
}
