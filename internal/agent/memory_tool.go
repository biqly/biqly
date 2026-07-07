package agent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/bytedance/sonic"
)

// RecalledExample is one confirmed-query few-shot example surfaced for a
// question.
type RecalledExample struct {
	Question     string          `json:"question"`
	LogicalQuery json.RawMessage `json:"logical_query,omitempty"`
}

// MemoryRecaller is the subset of confirmed-query few-shot recall the
// memory tool needs — satisfied in production by internal/metadata.Repository
// (via a thin adapter in the agent service, Task 11), not called directly
// here to avoid an internal/agent -> internal/metadata import.
type MemoryRecaller interface {
	Recall(ctx context.Context, datasourceID, modelID, question string, limit int) ([]RecalledExample, error)
}

// memoryRecallArgs is the strict shape of memory.recall arguments.
type memoryRecallArgs struct {
	identityArgs
	ModelID  string `json:"model_id,omitempty"`
	Question string `json:"question"`
	Limit    int    `json:"limit,omitempty"`
}

// MemoryTool implements the memory.recall tool.
type MemoryTool struct {
	recaller MemoryRecaller
	// maxLimit caps the admin-tunable memory recall limit (config.AIMemoryConfig)
	// regardless of what the proposal requests — a defense-in-depth ceiling
	// independent of PolicyEngine, which does not model this tool's arguments.
	maxLimit int
}

// NewMemoryTool builds a MemoryTool backed by recaller. maxLimit caps the
// number of examples any single call may request; values <= 0 disable the cap.
func NewMemoryTool(recaller MemoryRecaller, maxLimit int) *MemoryTool {
	return &MemoryTool{recaller: recaller, maxLimit: maxLimit}
}

// Name implements Tool.
func (*MemoryTool) Name() ToolName { return ToolMemoryRecall }

// Execute implements Tool.
func (t *MemoryTool) Execute(ctx context.Context, run RunContext, arguments json.RawMessage) (Observation, error) {
	var args memoryRecallArgs
	if err := strictDecode(arguments, &args); err != nil {
		return Observation{}, fmt.Errorf("memory.recall: %w", err)
	}
	if args.Question == "" {
		return Observation{}, errors.New("memory.recall: question is required")
	}
	limit := args.Limit
	if limit <= 0 || (t.maxLimit > 0 && limit > t.maxLimit) {
		limit = t.maxLimit
	}

	examples, err := callWithSingleRetry(ctx, func(ctx context.Context) ([]RecalledExample, error) {
		return t.recaller.Recall(ctx, run.DatasourceID, args.ModelID, args.Question, limit)
	})
	if err != nil {
		return Observation{}, fmt.Errorf("memory.recall: %w", err)
	}

	payload, err := sonic.Marshal(examples)
	if err != nil {
		return Observation{}, fmt.Errorf("memory.recall: encode observation: %w", err)
	}
	return Observation{Tool: ToolMemoryRecall, Payload: payload}, nil
}
