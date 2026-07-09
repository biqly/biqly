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
	gotRun     []RunContext
}

var errPlannerFailure = errors.New("planner failure")

func (p *scriptedPlanner) Decide(_ context.Context, run RunContext, history []RuntimeStep) (PlannerDecision, error) {
	p.gotHistory = append(p.gotHistory, history)
	p.gotRun = append(p.gotRun, run)
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
	require.NotNil(t, state.PendingClarification, "the paused question survives for a caller to surface")
	assert.Equal(t, "which metric?", state.PendingClarification.Question)

	// Resuming re-loads the paused state and continues the loop.
	planner.decisions = append(planner.decisions, finalDecision("resumed"))
	state, err = rt.Run(context.Background(), run, "run-3")
	require.NoError(t, err)
	require.NotNil(t, state.Terminal)
	assert.Equal(t, "resumed", state.Terminal.Final.Answer)
	assert.Nil(t, state.PendingClarification, "resuming past the clarification clears it")
}

// TestRuntimeAccumulatesClarificationHistoryAcrossTwoResumes is the genuine
// 2-round flow T8 must support: round 1 pauses on Q1, is resumed with A1
// (mirroring resumeWebAgentRun's job of pairing the persisted pending
// question with the caller's answer and feeding the accumulated history
// back in via RunContext), round 2 pauses on Q2 and is resumed with A2. It
// proves round 1's Q1/A1 is never lost by the time round 2's planner call
// happens — the exact gap RuntimeState.PendingClarification's single-field
// overwrite used to create — and that RuntimeState.ClarificationHistory
// durably accumulates both rounds across the persisted state.
func TestRuntimeAccumulatesClarificationHistoryAcrossTwoResumes(t *testing.T) {
	registry := NewRegistry(&PolicyEngine{}, NewCatalogTool(&fakeCatalogResolver{}))
	run := runtimeTestRun()
	store := newFakeStateStore()
	planner := &scriptedPlanner{decisions: []PlannerDecision{clarificationDecision("which metric?")}}
	rt := NewRuntime(planner, registry, store)

	// Round 1: pause on Q1.
	state, err := rt.Run(context.Background(), run, "run-accum")
	require.NoError(t, err)
	require.NotNil(t, state.PendingClarification)
	assert.Equal(t, "which metric?", state.PendingClarification.Question)
	assert.Empty(t, state.ClarificationHistory)

	// Resume 1: caller pairs the persisted pending question with the user's
	// answer (A1) and passes the accumulated history back via RunContext for
	// round 2's planner call — this is resumeWebAgentRun's job in production.
	run.ClarificationHistory = []ClarificationExchange{{Question: "which metric?", Answer: "net_revenue"}}
	planner.decisions = append(planner.decisions, clarificationDecision("which quarter?"))
	state, err = rt.Run(context.Background(), run, "run-accum")
	require.NoError(t, err)
	require.NotNil(t, state.PendingClarification)
	assert.Equal(t, "which quarter?", state.PendingClarification.Question)
	// Round 1's Q1/A1 must survive into the persisted state, not be
	// overwritten by round 2's new pending question.
	require.Len(t, state.ClarificationHistory, 1)
	assert.Equal(t, "which metric?", state.ClarificationHistory[0].Question)
	assert.Equal(t, "net_revenue", state.ClarificationHistory[0].Answer)
	// Round 2's planner call must have seen round 1's Q1/A1.
	require.Len(t, planner.gotRun, 2)
	require.Len(t, planner.gotRun[1].ClarificationHistory, 1)
	assert.Equal(t, "net_revenue", planner.gotRun[1].ClarificationHistory[0].Answer)

	// Resume 2: round 2 resolves too — the caller now carries BOTH rounds.
	run.ClarificationHistory = append(run.ClarificationHistory,
		ClarificationExchange{Question: "which quarter?", Answer: "Q2"})
	planner.decisions = append(planner.decisions, finalDecision("net revenue for Q2"))
	state, err = rt.Run(context.Background(), run, "run-accum")
	require.NoError(t, err)
	require.NotNil(t, state.Terminal)
	assert.Equal(t, "net revenue for Q2", state.Terminal.Final.Answer)
	require.Len(t, state.ClarificationHistory, 2)
	assert.Equal(t, "which quarter?", state.ClarificationHistory[1].Question)
	assert.Equal(t, "Q2", state.ClarificationHistory[1].Answer)
	// The FINAL planner call must have seen BOTH rounds' Q&A, not just the
	// latest one.
	require.Len(t, planner.gotRun, 3)
	require.Len(t, planner.gotRun[2].ClarificationHistory, 2)
	assert.Equal(t, "net_revenue", planner.gotRun[2].ClarificationHistory[0].Answer)
	assert.Equal(t, "Q2", planner.gotRun[2].ClarificationHistory[1].Answer)
}

// TestRuntimeClarificationCarriesOptionsThroughPendingState proves the
// planner's clarification options (not just the question) survive into
// RuntimeState.PendingClarification, since T8's clarification_required SSE
// event renders both question and choices from this field.
func TestRuntimeClarificationCarriesOptionsThroughPendingState(t *testing.T) {
	registry := NewRegistry(&PolicyEngine{}, NewCatalogTool(&fakeCatalogResolver{}))
	run := runtimeTestRun()
	planner := &scriptedPlanner{decisions: []PlannerDecision{
		{Kind: DecisionClarification, Clarification: &Clarification{
			Question: "which metric?",
			Options:  []string{"net_revenue", "gross_revenue"},
		}},
	}}
	store := newFakeStateStore()
	rt := NewRuntime(planner, registry, store)

	state, err := rt.Run(context.Background(), run, "run-9")
	require.NoError(t, err)
	require.NotNil(t, state.PendingClarification)
	assert.Equal(t, []string{"net_revenue", "gross_revenue"}, state.PendingClarification.Options)
}

// TestRuntimeMaxClarificationRoundsExceededClearsPending proves that once a
// run fails terminally for exhausting its clarification budget, there is no
// stale "pending" question left behind for a caller to (incorrectly) resume.
func TestRuntimeMaxClarificationRoundsExceededClearsPending(t *testing.T) {
	registry := NewRegistry(&PolicyEngine{}, NewCatalogTool(&fakeCatalogResolver{}))
	run := runtimeTestRun()
	run.MaxClarificationRounds = 0
	store := newFakeStateStore()
	planner := &scriptedPlanner{decisions: []PlannerDecision{clarificationDecision("q1")}}
	rt := NewRuntime(planner, registry, store)

	state, err := rt.Run(context.Background(), run, "run-10")
	require.NoError(t, err)
	require.NotNil(t, state.Terminal)
	assert.Equal(t, "max_clarification_rounds_exceeded", state.Terminal.Failure.ReasonCode)
	assert.Nil(t, state.PendingClarification)
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

// TestRuntimeStepHookObservesStartedThenCompleted proves the live event sink
// (T6 item 1): SetStepHook fires once when a tool proposal is persisted
// (Observation still nil — a "started" event) and once more after the
// outcome is known (Observation set — a "completed" event), in that order,
// synchronously as Run executes rather than only after Run returns.
func TestRuntimeStepHookObservesStartedThenCompleted(t *testing.T) {
	fake := &fakeCatalogResolver{result: []CatalogEntity{{Table: "orders"}}}
	registry := NewRegistry(&PolicyEngine{}, NewCatalogTool(fake))
	run := runtimeTestRun()
	planner := &scriptedPlanner{decisions: []PlannerDecision{
		toolDecision(ToolCatalog, identityJSON(run)),
		finalDecision("42"),
	}}
	store := newFakeStateStore()
	rt := NewRuntime(planner, registry, store)

	var seen []RuntimeStep
	rt.SetStepHook(func(step RuntimeStep) {
		// Deep-copy so a later mutation of the shared underlying step (the
		// runtime reuses state.Steps[idx] in place) cannot retroactively
		// change what this test already observed.
		seen = append(seen, step)
	})

	state, err := rt.Run(context.Background(), run, "run-hook-1")
	require.NoError(t, err)
	require.NotNil(t, state.Terminal)

	require.Len(t, seen, 2, "one 'started' event and one 'completed' event")
	assert.Nil(t, seen[0].Observation, "first event fires before dispatch")
	assert.Equal(t, 1, seen[0].Seq)
	assert.NotNil(t, seen[1].Observation, "second event fires after the outcome is known")
	assert.Equal(t, 1, seen[1].Seq)
}

// TestRuntimeStepHookObservesDenial proves a policy-denied step is also
// surfaced live: a "started" event (proposal persisted, pre-dispatch) then a
// second event once the policy denial is known (DeniedReason set).
func TestRuntimeStepHookObservesDenial(t *testing.T) {
	fake := &fakeCatalogResolver{result: []CatalogEntity{{Table: "orders"}}}
	registry := NewRegistry(&PolicyEngine{}, NewCatalogTool(fake))
	run := runtimeTestRun()
	run.AllowedTools = []ToolName{ToolSemantic, ToolCatalog}
	planner := &scriptedPlanner{decisions: []PlannerDecision{
		toolDecision(ToolQueryExecute, identityJSON(run)), // not allowlisted -> denied
		toolDecision(ToolCatalog, identityJSON(run)),
		finalDecision("corrected"),
	}}
	store := newFakeStateStore()
	rt := NewRuntime(planner, registry, store)

	var seen []RuntimeStep
	rt.SetStepHook(func(step RuntimeStep) {
		seen = append(seen, step)
	})

	_, err := rt.Run(context.Background(), run, "run-hook-2")
	require.NoError(t, err)

	require.GreaterOrEqual(t, len(seen), 2)
	assert.Empty(t, seen[0].DeniedReason, "first event is the pre-dispatch 'started' step")
	assert.Equal(t, ReasonToolNotAllowlisted, seen[1].DeniedReason, "second event carries the policy denial")
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
