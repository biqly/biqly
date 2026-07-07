package agent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/bytedance/sonic"
)

// CompileResult is the Query service's validated compile output: the SQL it
// generated from a LogicalQuery, plus a fingerprint over that output.
type CompileResult struct {
	Fingerprint string `json:"fingerprint"`
	SQL         string `json:"sql"`
}

// QueryResult is one query.execute outcome.
type QueryResult struct {
	Rows     json.RawMessage `json:"rows"`
	RowCount int             `json:"row_count"`
}

// QueryCompiler is the subset of pkg/queryclient.Client the compile tool
// needs — satisfied by the real client's Compile/CompileWithModel.
type QueryCompiler interface {
	Compile(ctx context.Context, datasourceID string, logicalQuery json.RawMessage) (CompileResult, error)
}

// QueryExecutor is the subset of pkg/queryclient.Client the execute tool
// needs — satisfied by the real client's Run/RunWithModel.
type QueryExecutor interface {
	Execute(ctx context.Context, datasourceID string, logicalQuery json.RawMessage, rowLimit, timeoutSeconds int) (QueryResult, error)
}

// ErrCompileFingerprintMismatch means query.execute's logical_query does not
// reproduce the fingerprint it claims — the Query service must re-validate
// it before the caller returns to query.execute with the result.
var ErrCompileFingerprintMismatch = errors.New("logical_query does not match the claimed compile fingerprint")

// QueryCompileTool implements query.compile: it validates a LogicalQuery
// through the real Query service and returns the resulting SQL and
// fingerprint. It never executes anything.
type QueryCompileTool struct {
	compiler QueryCompiler
}

// NewQueryCompileTool builds a QueryCompileTool backed by compiler.
func NewQueryCompileTool(compiler QueryCompiler) *QueryCompileTool {
	return &QueryCompileTool{compiler: compiler}
}

// Name implements Tool.
func (*QueryCompileTool) Name() ToolName { return ToolQueryCompile }

// Execute implements Tool.
func (t *QueryCompileTool) Execute(ctx context.Context, run RunContext, arguments json.RawMessage) (Observation, error) {
	var args queryProposalArgs
	if err := strictDecode(arguments, &args); err != nil {
		return Observation{}, fmt.Errorf("query.compile: %w", err)
	}
	if len(args.LogicalQuery) == 0 {
		return Observation{}, errors.New("query.compile: logical_query is required")
	}

	result, err := callWithSingleRetry(ctx, func(ctx context.Context) (CompileResult, error) {
		return t.compiler.Compile(ctx, run.DatasourceID, args.LogicalQuery)
	})
	if err != nil {
		return Observation{}, fmt.Errorf("query.compile: %w", err)
	}

	payload, err := sonic.Marshal(result)
	if err != nil {
		return Observation{}, fmt.Errorf("query.compile: encode observation: %w", err)
	}
	return Observation{Tool: ToolQueryCompile, Payload: payload}, nil
}

// QueryExecuteTool implements query.execute. It accepts only a logical_query
// that reproduces a fingerprint the caller claims came from query.compile —
// verified by recompiling it through the same trusted Query service path
// before running anything. This guarantees execution can never bypass
// compile-time validation, even when the runtime already ran query.compile
// moments earlier: there is no raw-SQL or "already validated, trust me"
// argument that skips the Query service's own checks.
type QueryExecuteTool struct {
	compiler QueryCompiler
	executor QueryExecutor
}

// NewQueryExecuteTool builds a QueryExecuteTool backed by compiler and executor.
func NewQueryExecuteTool(compiler QueryCompiler, executor QueryExecutor) *QueryExecuteTool {
	return &QueryExecuteTool{compiler: compiler, executor: executor}
}

// Name implements Tool.
func (*QueryExecuteTool) Name() ToolName { return ToolQueryExecute }

// Execute implements Tool.
func (t *QueryExecuteTool) Execute(ctx context.Context, run RunContext, arguments json.RawMessage) (Observation, error) {
	var args queryProposalArgs
	if err := strictDecode(arguments, &args); err != nil {
		return Observation{}, fmt.Errorf("query.execute: %w", err)
	}
	if len(args.LogicalQuery) == 0 || args.Fingerprint == "" {
		return Observation{}, errors.New("query.execute: logical_query and fingerprint are required")
	}

	compiled, err := t.compiler.Compile(ctx, run.DatasourceID, args.LogicalQuery)
	if err != nil {
		return Observation{}, fmt.Errorf("query.execute: revalidate compile: %w", err)
	}
	if compiled.Fingerprint != args.Fingerprint {
		return Observation{}, fmt.Errorf("query.execute: %w", ErrCompileFingerprintMismatch)
	}

	result, err := callWithSingleRetry(ctx, func(ctx context.Context) (QueryResult, error) {
		return t.executor.Execute(ctx, run.DatasourceID, args.LogicalQuery, args.RowLimit, args.TimeoutSeconds)
	})
	if err != nil {
		return Observation{}, fmt.Errorf("query.execute: %w", err)
	}

	payload, err := sonic.Marshal(result)
	if err != nil {
		return Observation{}, fmt.Errorf("query.execute: encode observation: %w", err)
	}
	return Observation{Tool: ToolQueryExecute, Payload: payload}, nil
}
