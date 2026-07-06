package agent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/bytedance/sonic"
)

// SemanticPlan is the LogicalQuery-shaped plan the AI service's NL→query
// generation returns for a question, opaque to the agent beyond its
// confidence score.
type SemanticPlan struct {
	LogicalQuery json.RawMessage `json:"logical_query"`
	Confidence   float64         `json:"confidence"`
}

// SemanticGenerator is the subset of pkg/aiclient.Client the semantic tool
// needs. A local interface, satisfied by the real client's Query method,
// so tests use a fake instead of a real HTTP client.
type SemanticGenerator interface {
	GeneratePlan(ctx context.Context, datasourceID, modelID, question string) (SemanticPlan, error)
}

// semanticResolveArgs is the strict shape of semantic.resolve arguments.
type semanticResolveArgs struct {
	identityArgs
	ModelID  string `json:"model_id,omitempty"`
	Question string `json:"question"`
}

// SemanticTool implements the semantic.resolve tool.
type SemanticTool struct {
	generator SemanticGenerator
}

// NewSemanticTool builds a SemanticTool backed by generator.
func NewSemanticTool(generator SemanticGenerator) *SemanticTool {
	return &SemanticTool{generator: generator}
}

// Name implements Tool.
func (*SemanticTool) Name() ToolName { return ToolSemantic }

// Execute implements Tool.
func (t *SemanticTool) Execute(ctx context.Context, run RunContext, arguments json.RawMessage) (Observation, error) {
	var args semanticResolveArgs
	if err := strictDecode(arguments, &args); err != nil {
		return Observation{}, fmt.Errorf("semantic.resolve: %w", err)
	}
	if args.Question == "" {
		return Observation{}, errors.New("semantic.resolve: question is required")
	}

	plan, err := callWithSingleRetry(ctx, func(ctx context.Context) (SemanticPlan, error) {
		return t.generator.GeneratePlan(ctx, run.DatasourceID, args.ModelID, args.Question)
	})
	if err != nil {
		return Observation{}, fmt.Errorf("semantic.resolve: %w", err)
	}

	payload, err := sonic.Marshal(plan)
	if err != nil {
		return Observation{}, fmt.Errorf("semantic.resolve: encode observation: %w", err)
	}
	return Observation{Tool: ToolSemantic, Payload: payload}, nil
}
