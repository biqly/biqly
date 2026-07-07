package agent

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeStateStore is an in-memory StateStore. Each Save call is recorded so
// tests can assert persistence happens before/after external calls.
type fakeStateStore struct {
	mu      sync.Mutex
	states  map[string]RuntimeState
	saves   int
	saveErr error
	loadErr error
}

func newFakeStateStore() *fakeStateStore {
	return &fakeStateStore{states: make(map[string]RuntimeState)}
}

func (f *fakeStateStore) Save(_ context.Context, runID string, state RuntimeState) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.saves++
	if f.saveErr != nil {
		return f.saveErr
	}
	f.states[runID] = state
	return nil
}

func (f *fakeStateStore) Load(_ context.Context, runID string) (RuntimeState, bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.loadErr != nil {
		return RuntimeState{}, false, f.loadErr
	}
	state, ok := f.states[runID]
	return state, ok, nil
}

// scriptedPlanner returns the decisions in order, one per Decide call. A nil
// entry means "return errPlannerFailure" instead of a decision.
type scriptedPlanner struct {
	decisions  []PlannerDecision
	errs       []error
	calls      int
	gotHistory [][]RuntimeStep
}

var errPlannerFailure = errors.New("planner failure")

func (p *scriptedPlanner) Decide(_ context.Context, _ RunContext, history []RuntimeStep) (PlannerDecision, error) {
	p.gotHistory = append(p.gotHistory, history)
	i := p.calls
	p.calls++
	if i < len(p.errs) && p.errs[i] != nil {
		return PlannerDecision{}, p.errs[i]
	}
	if i >= len(p.decisions) {
		return PlannerDecision{}, errPlannerFailure
	}
	return p.decisions[i], nil
}

func toolDecision(tool ToolName, args string) PlannerDecision {
	return PlannerDecision{Kind: DecisionTool, Proposal: &Proposal{Tool: tool, Arguments: []byte(args)}}
}

func finalDecision(answer string) PlannerDecision {
	return PlannerDecision{Kind: DecisionFinal, Final: &FinalResponse{Answer: answer, Confidence: 1}}
}

func clarificationDecision(question string) PlannerDecision {
	return PlannerDecision{Kind: DecisionClarification, Clarification: &Clarification{Question: question}}
}

// runtimeTestRun builds a RunContext with sane bounds for runtime tests.
func runtimeTestRun() RunContext {
	run := baseRunContext()
	run.MaxSteps = 6
	run.MaxClarificationRounds = 2
	run.Timeout = time.Minute
	return run
}

func identityJSON(run RunContext) string {
	return `{"tenant_id":"` + run.TenantID + `","user_id":"` + run.UserID + `","datasource_id":"` + run.DatasourceID + `"}`
}

func TestRuntimeSuccessfulPlanReachesFinal(t *testing.T) {
	fake := &fakeCatalogResolver{result: []CatalogEntity{{Table: "orders"}}}
	registry := NewRegistry(&PolicyEngine{}, NewCatalogTool(fake))
	run := runtimeTestRun()
	planner := &scriptedPlanner{decisions: []PlannerDecision{
		toolDecision(ToolCatalog, identityJSON(run)),
		finalDecision("42"),
	}}
	store := newFakeStateStore()
	rt := NewRuntime(planner, registry, store)

	state, err := rt.Run(context.Background(), run, "run-1")
	require.NoError(t, err)
	require.NotNil(t, state.Terminal)
	assert.Equal(t, DecisionFinal, state.Terminal.Kind)
	assert.Equal(t, "42", state.Terminal.Final.Answer)
	require.Len(t, state.Steps, 1)
	assert.NotNil(t, state.Steps[0].Observation)
	assert.Equal(t, 1, fake.calls)
}

func TestRuntimePolicyDenialLetsPlannerCorrect(t *testing.T) {
	fake := &fakeCatalogResolver{result: []CatalogEntity{{Table: "orders"}}}
	registry := NewRegistry(&PolicyEngine{}, NewCatalogTool(fake))
	run := runtimeTestRun()
	run.AllowedTools = []ToolName{ToolSemantic, ToolCatalog} // semantic tool not registered -> denial path via unregistered? use policy denial instead
	run.HiddenColumns = nil

	// First proposal denied by policy (tool not allowlisted); second corrects
	// by proposing the allowlisted tool instead.
	planner := &scriptedPlanner{decisions: []PlannerDecision{
		toolDecision(ToolQueryExecute, identityJSON(run)),
		toolDecision(ToolCatalog, identityJSON(run)),
		finalDecision("corrected"),
	}}
	store := newFakeStateStore()
	rt := NewRuntime(planner, registry, store)

	state, err := rt.Run(context.Background(), run, "run-2")
	require.NoError(t, err)
	require.NotNil(t, state.Terminal)
	assert.Equal(t, "corrected", state.Terminal.Final.Answer)
	require.Len(t, state.Steps, 2)
	assert.Equal(t, ReasonToolNotAllowlisted, state.Steps[0].DeniedReason)
	assert.Nil(t, state.Steps[0].Observation)
	assert.NotNil(t, state.Steps[1].Observation)

	// The planner's second call must have seen the first step's denial.
	require.Len(t, planner.gotHistory, 3)
	require.Len(t, planner.gotHistory[1], 1)
	assert.Equal(t, ReasonToolNotAllowlisted, planner.gotHistory[1][0].DeniedReason)
}

func TestRuntimeClarificationPauses(t *testing.T) {
	registry := NewRegistry(&PolicyEngine{}, NewCatalogTool(&fakeCatalogResolver{}))
	run := runtimeTestRun()
	planner := &scriptedPlanner{decisions: []PlannerDecision{clarificationDecision("which metric?")}}
	store := newFakeStateStore()
	rt := NewRuntime(planner, registry, store)

	state, err := rt.Run(context.Background(), run, "run-3")
	require.NoError(t, err)
	assert.Nil(t, state.Terminal, "clarification pauses without a terminal result")
	assert.Equal(t, 1, state.ClarificationRounds)

	// Resuming re-loads the paused state and continues the loop.
	planner.decisions = append(planner.decisions, finalDecision("resumed"))
	state, err = rt.Run(context.Background(), run, "run-3")
	require.NoError(t, err)
	require.NotNil(t, state.Terminal)
	assert.Equal(t, "resumed", state.Terminal.Final.Answer)
}

func TestRuntimeMaxTwoClarificationRoundsExceeded(t *testing.T) {
	registry := NewRegistry(&PolicyEngine{}, NewCatalogTool(&fakeCatalogResolver{}))
	run := runtimeTestRun()
	run.MaxClarificationRounds = 2
	store := newFakeStateStore()

	planner := &scriptedPlanner{decisions: []PlannerDecision{clarificationDecision("q1")}}
	rt := NewRuntime(planner, registry, store)
	state, err := rt.Run(context.Background(), run, "run-4")
	require.NoError(t, err)
	assert.Equal(t, 1, state.ClarificationRounds)

	planner.decisions = append(planner.decisions, clarificationDecision("q2"))
	state, err = rt.Run(context.Background(), run, "run-4")
	require.NoError(t, err)
	assert.Equal(t, 2, state.ClarificationRounds)

	// A third round exceeds the bound and the run fails terminally.
	planner.decisions = append(planner.decisions, clarificationDecision("q3"))
	state, err = rt.Run(context.Background(), run, "run-4")
	require.NoError(t, err)
	require.NotNil(t, state.Terminal)
	assert.Equal(t, DecisionFail, state.Terminal.Kind)
	assert.Equal(t, "max_clarification_rounds_exceeded", state.Terminal.Failure.ReasonCode)
}

func TestRuntimeMaxSixStepsExceeded(t *testing.T) {
	fake := &fakeCatalogResolver{result: []CatalogEntity{{Table: "orders"}}}
	registry := NewRegistry(&PolicyEngine{}, NewCatalogTool(fake))
	run := runtimeTestRun()
	run.MaxSteps = 6

	decisions := make([]PlannerDecision, 0, 7)
	for range 7 {
		decisions = append(decisions, toolDecision(ToolCatalog, identityJSON(run)))
	}
	planner := &scriptedPlanner{decisions: decisions}
	store := newFakeStateStore()
	rt := NewRuntime(planner, registry, store)

	state, err := rt.Run(context.Background(), run, "run-5")
	require.NoError(t, err)
	require.NotNil(t, state.Terminal)
	assert.Equal(t, "max_steps_exceeded", state.Terminal.Failure.ReasonCode)
	assert.Len(t, state.Steps, 6, "the 7th proposal must never be dispatched")
	assert.Equal(t, 6, fake.calls)
}

func TestRuntimeTimeoutFinalizesTerminalFailure(t *testing.T) {
	registry := NewRegistry(&PolicyEngine{}, NewCatalogTool(&fakeCatalogResolver{}))
	run := runtimeTestRun()
	run.Timeout = time.Nanosecond // already expired by the time Run checks it
	planner := &scriptedPlanner{decisions: []PlannerDecision{finalDecision("too late")}}
	store := newFakeStateStore()
	rt := NewRuntime(planner, registry, store)

	time.Sleep(time.Millisecond)
	state, err := rt.Run(context.Background(), run, "run-6")
	require.NoError(t, err)
	require.NotNil(t, state.Terminal)
	assert.Equal(t, "timeout", state.Terminal.Failure.ReasonCode)
	assert.Zero(t, planner.calls, "the planner must never be invoked once the deadline has passed")
}

func TestRuntimeCancellationBeforeQueryExecuteReturnsNonTerminal(t *testing.T) {
	registry := NewRegistry(&PolicyEngine{}, NewCatalogTool(&fakeCatalogResolver{}))
	run := runtimeTestRun()
	planner := &scriptedPlanner{decisions: []PlannerDecision{finalDecision("unused")}}
	store := newFakeStateStore()
	rt := NewRuntime(planner, registry, store)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	state, err := rt.Run(ctx, run, "run-7")
	require.Error(t, err)
	assert.True(t, errors.Is(err, context.Canceled))
	assert.Nil(t, state.Terminal, "safe to resume/retry — Query Execute never started")
}

func TestRuntimeCancellationAfterQueryExecuteForcesTerminalFailure(t *testing.T) {
	compiler := &fakeQueryCompiler{result: CompileResult{Fingerprint: "fp-1"}}
	executor := &fakeQueryExecutor{result: QueryResult{RowCount: 1}}
	registry := NewRegistry(&PolicyEngine{}, NewQueryExecuteTool(compiler, executor))
	run := runtimeTestRun()

	ctx, cancel := context.WithCancel(context.Background())
	execArgs := executeArgs(t, run, `{}`, "fp-1", 10, 10)
	planner := &scriptedPlanner{decisions: []PlannerDecision{
		{Kind: DecisionTool, Proposal: &Proposal{Tool: ToolQueryExecute, Arguments: execArgs}},
		finalDecision("unreachable"),
	}}
	// Cancel between the first (execute) step and the second planner call.
	planner.errs = []error{nil, nil}
	store := newFakeStateStore()

	// A store whose Save triggers cancellation the moment QueryExecuteStarted
	// is persisted, simulating cancellation arriving right after execute begins.
	cancelingStore := &cancelAfterExecuteStore{inner: store, cancel: cancel}
	rt := NewRuntime(planner, registry, cancelingStore)

	state, err := rt.Run(ctx, run, "run-8")
	require.NoError(t, err, "once execute has started, cancellation must be absorbed into a terminal result, not surfaced as a bare error")
	require.NotNil(t, state.Terminal)
	assert.Equal(t, DecisionFail, state.Terminal.Kind)
	assert.Equal(t, "context_canceled", state.Terminal.Failure.ReasonCode)
	assert.True(t, state.QueryExecuteStarted)
}

// cancelAfterExecuteStore cancels its associated context the moment a saved
// state shows QueryExecuteStarted, simulating cancellation arriving exactly
// after query.execute begins.
type cancelAfterExecuteStore struct {
	inner  StateStore
	cancel context.CancelFunc
	fired  bool
}

func (s *cancelAfterExecuteStore) Save(ctx context.Context, runID string, state RuntimeState) error {
	err := s.inner.Save(ctx, runID, state)
	if state.QueryExecuteStarted && !s.fired {
		s.fired = true
		s.cancel()
	}
	return err
}

func (s *cancelAfterExecuteStore) Load(ctx context.Context, runID string) (RuntimeState, bool, error) {
	return s.inner.Load(ctx, runID)
}

func TestRuntimeRetriesPlannerErrorOnce(t *testing.T) {
	registry := NewRegistry(&PolicyEngine{}, NewCatalogTool(&fakeCatalogResolver{}))
	run := runtimeTestRun()
	planner := &scriptedPlanner{
		errs:      []error{errors.New("transient decode error")},
		decisions: []PlannerDecision{{}, finalDecision("recovered")},
	}
	store := newFakeStateStore()
	rt := NewRuntime(planner, registry, store)

	state, err := rt.Run(context.Background(), run, "run-9")
	require.NoError(t, err)
	require.NotNil(t, state.Terminal)
	assert.Equal(t, "recovered", state.Terminal.Final.Answer)
	assert.Equal(t, 2, planner.calls, "exactly one retry after the first failure")
}

// A second consecutive planner error before Query Execute has ever started
// surfaces as a plain resumable error — like cancellation at the same
// stage, nothing has run yet, so retrying the whole run later (e.g. via a
// NATS redelivery) is safe and the run is not forced into a terminal state.
func TestRuntimeFailsAfterSecondConsecutivePlannerError(t *testing.T) {
	registry := NewRegistry(&PolicyEngine{}, NewCatalogTool(&fakeCatalogResolver{}))
	run := runtimeTestRun()
	planner := &scriptedPlanner{errs: []error{errors.New("first"), errors.New("second")}}
	store := newFakeStateStore()
	rt := NewRuntime(planner, registry, store)

	state, err := rt.Run(context.Background(), run, "run-10")
	require.Error(t, err)
	assert.Nil(t, state.Terminal)
	assert.Equal(t, 2, planner.calls, "must not retry a second time")
}

// The same second-consecutive-planner-error case, but after Query Execute
// has already started: now the run must resolve to a terminal failure
// instead of surfacing a bare resumable error — nothing may retry a run
// whose execute step has already happened.
func TestRuntimePlannerErrorAfterQueryExecuteForcesTerminalFailure(t *testing.T) {
	compiler := &fakeQueryCompiler{result: CompileResult{Fingerprint: "fp-1"}}
	executor := &fakeQueryExecutor{result: QueryResult{RowCount: 1}}
	registry := NewRegistry(&PolicyEngine{}, NewQueryExecuteTool(compiler, executor))
	run := runtimeTestRun()
	execArgs := executeArgs(t, run, `{}`, "fp-1", 10, 10)
	planner := &scriptedPlanner{
		decisions: []PlannerDecision{{Kind: DecisionTool, Proposal: &Proposal{Tool: ToolQueryExecute, Arguments: execArgs}}},
		errs:      []error{nil, errors.New("first"), errors.New("second")},
	}
	store := newFakeStateStore()
	rt := NewRuntime(planner, registry, store)

	state, err := rt.Run(context.Background(), run, "run-10b")
	require.NoError(t, err)
	require.NotNil(t, state.Terminal)
	assert.Equal(t, "planner_error", state.Terminal.Failure.ReasonCode)
	assert.True(t, state.QueryExecuteStarted)
}

func TestRuntimeResumesFromPersistedStateOnNATSRedelivery(t *testing.T) {
	fake := &fakeCatalogResolver{result: []CatalogEntity{{Table: "orders"}}}
	registry := NewRegistry(&PolicyEngine{}, NewCatalogTool(fake))
	run := runtimeTestRun()
	store := newFakeStateStore()

	// Simulate a run that already took one step (as if the previous
	// delivery attempt persisted it and then the process crashed before the
	// planner could act on it).
	preexisting := RuntimeState{Steps: []RuntimeStep{{
		Seq:         1,
		Proposal:    Proposal{Tool: ToolCatalog, Arguments: []byte(identityJSON(run))},
		Observation: &Observation{Tool: ToolCatalog, Payload: []byte(`[{"table":"orders"}]`)},
	}}}
	require.NoError(t, store.Save(context.Background(), "run-11", preexisting))

	planner := &scriptedPlanner{decisions: []PlannerDecision{finalDecision("resumed-final")}}
	rt := NewRuntime(planner, registry, store)

	state, err := rt.Run(context.Background(), run, "run-11")
	require.NoError(t, err)
	require.NotNil(t, state.Terminal)
	assert.Equal(t, "resumed-final", state.Terminal.Final.Answer)
	// The pre-existing step must still be there, untouched, plus exactly one
	// new step for the planner's next decision (Final has no tool step, so
	// the count stays 1) — the run resumed, it did not restart from zero.
	require.Len(t, state.Steps, 1)
	assert.Zero(t, fake.calls, "resuming from a persisted final decision must not redo the already-observed catalog call")
}

func TestRuntimeImmutableTerminalResultIsNotReplanned(t *testing.T) {
	registry := NewRegistry(&PolicyEngine{}, NewCatalogTool(&fakeCatalogResolver{}))
	run := runtimeTestRun()
	store := newFakeStateStore()
	terminal := RuntimeState{Terminal: &TerminalResult{Kind: DecisionFinal, Final: &FinalResponse{Answer: "done"}}}
	require.NoError(t, store.Save(context.Background(), "run-12", terminal))

	planner := &scriptedPlanner{decisions: []PlannerDecision{finalDecision("should not run")}}
	rt := NewRuntime(planner, registry, store)

	state, err := rt.Run(context.Background(), run, "run-12")
	assert.True(t, errors.Is(err, ErrRunAlreadyTerminal))
	assert.Equal(t, "done", state.Terminal.Final.Answer)
	assert.Zero(t, planner.calls, "a terminal run must never be replanned")
}

func TestMarshalUnmarshalStateRoundTrips(t *testing.T) {
	state := RuntimeState{
		Steps: []RuntimeStep{{
			Seq:         1,
			Proposal:    Proposal{Tool: ToolCatalog, Arguments: []byte(`{"a":1}`)},
			Observation: &Observation{Tool: ToolCatalog, Payload: []byte(`[{"table":"orders"}]`)},
		}},
		ClarificationRounds: 1,
		QueryExecuteStarted: true,
		Terminal:            &TerminalResult{Kind: DecisionFinal, Final: &FinalResponse{Answer: "42", Confidence: 0.9}},
	}

	raw, err := MarshalState(state)
	require.NoError(t, err)

	got, err := UnmarshalState(raw)
	require.NoError(t, err)
	assert.Equal(t, state, got)
}

func TestUnmarshalStateEmptyIsZeroValue(t *testing.T) {
	got, err := UnmarshalState(nil)
	require.NoError(t, err)
	assert.Equal(t, RuntimeState{}, got)
}

func TestRuntimePersistsBeforeAndAfterEveryExternalCall(t *testing.T) {
	fake := &fakeCatalogResolver{result: []CatalogEntity{{Table: "orders"}}}
	registry := NewRegistry(&PolicyEngine{}, NewCatalogTool(fake))
	run := runtimeTestRun()
	planner := &scriptedPlanner{decisions: []PlannerDecision{
		toolDecision(ToolCatalog, identityJSON(run)),
		finalDecision("done"),
	}}
	store := newFakeStateStore()
	rt := NewRuntime(planner, registry, store)

	_, err := rt.Run(context.Background(), run, "run-13")
	require.NoError(t, err)
	// One save before dispatch, one after, one for the final terminal write.
	assert.Equal(t, 3, store.saves)
}
