package abtest

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestRepositoryUpdateExperimentToRunningValidatesVariantAllocation(t *testing.T) {
	ctx := context.Background()
	experiment := &Experiment{
		ID:           "exp-1",
		Name:         "clarification prompt",
		TemplateName: "clarification",
		Locale:       "en",
		Status:       ExperimentStatusRunning,
	}
	runner := &fakeDBRunner{
		queryContext: func(_ context.Context, query string, args ...any) (rowsScanner, error) {
			if !strings.Contains(query, "FROM ab_variants") {
				t.Errorf("UpdateExperiment validation query = %q, want ab_variants lookup", query)
			}
			if got, want := args[0], experiment.ID; got != want {
				t.Errorf("UpdateExperiment validation experiment id = %v, want %v", got, want)
			}
			return newFakeRows([][]any{
				{"control", "exp-1", "control", 1, 50, true},
				{"treatment", "exp-1", "treatment", 2, 40, false},
			}), nil
		},
		execContext: func(context.Context, string, ...any) (sql.Result, error) {
			t.Fatal("UpdateExperiment should not update when variants are invalid")
			return fakeResult{}, nil
		},
	}
	repo := newRepositoryWithRunner(runner)

	err := repo.UpdateExperiment(ctx, experiment)
	if err == nil {
		t.Fatal("UpdateExperiment(ctx, running experiment) error = nil, want invalid traffic error")
	}
	if !strings.Contains(err.Error(), "traffic_pct") {
		t.Errorf("UpdateExperiment(ctx, running experiment) error = %v, want traffic_pct validation", err)
	}
}

func TestRepositoryAddVariantValidatesPromptTemplateVersion(t *testing.T) {
	ctx := context.Background()
	variant := &Variant{
		ID:              "variant-1",
		ExperimentID:    "exp-1",
		Name:            "treatment",
		TemplateVersion: 3,
		TrafficPct:      50,
	}
	runner := &fakeDBRunner{
		queryRowContext: func(_ context.Context, query string, args ...any) rowScanner {
			if !strings.Contains(query, "FROM ab_experiments") {
				t.Errorf("AddVariant experiment query = %q, want ab_experiments lookup", query)
			}
			if got, want := args[0], variant.ExperimentID; got != want {
				t.Errorf("AddVariant experiment id = %v, want %v", got, want)
			}
			return fakeRow{values: []any{
				"exp-1", "Experiment", "", "clarification", "en", string(ExperimentStatusDraft),
				sql.NullTime{}, sql.NullTime{}, sql.NullString{}, time.Now(), time.Now(),
			}}
		},
		queryScalarContext: func(_ context.Context, query string, args ...any) rowScanner {
			if !strings.Contains(query, "FROM ai_prompt_templates") {
				t.Errorf("AddVariant template query = %q, want ai_prompt_templates lookup", query)
			}
			if got, want := args[0], "clarification"; got != want {
				t.Errorf("AddVariant template name = %v, want %v", got, want)
			}
			if got, want := args[1], "en"; got != want {
				t.Errorf("AddVariant locale = %v, want %v", got, want)
			}
			if got, want := args[2], variant.TemplateVersion; got != want {
				t.Errorf("AddVariant template version = %v, want %v", got, want)
			}
			return fakeRow{values: []any{0}}
		},
		execContext: func(context.Context, string, ...any) (sql.Result, error) {
			t.Fatal("AddVariant should not insert when template version does not exist")
			return fakeResult{}, nil
		},
	}
	repo := newRepositoryWithRunner(runner)

	err := repo.AddVariant(ctx, variant)
	if err == nil {
		t.Fatal("AddVariant(ctx, variant) error = nil, want missing template version error")
	}
	if !strings.Contains(err.Error(), "template version") {
		t.Errorf("AddVariant(ctx, variant) error = %v, want template version validation", err)
	}
}

func TestRepositoryCreateExperimentDraft(t *testing.T) {
	ctx := context.Background()
	now := time.Now()
	experiment := &Experiment{
		Name:         "test-experiment",
		Description:  "test description",
		TemplateName: "system_rules",
		Locale:       "en",
		Status:       ExperimentStatusDraft,
		StartedAt:    &now,
		EndedAt:      &now,
		CreatedBy:    "admin-1",
	}
	runner := &fakeDBRunner{
		queryRowContext: func(_ context.Context, query string, _ ...any) rowScanner {
			if !strings.Contains(query, "INSERT INTO ab_experiments") {
				t.Errorf("CreateExperiment query = %q, want INSERT INTO ab_experiments", query)
			}
			return fakeRow{values: []any{"exp-1", now, now}}
		},
		queryContext: func(context.Context, string, ...any) (rowsScanner, error) {
			return newFakeRows(nil), nil // no variants for draft
		},
	}
	repo := newRepositoryWithRunner(runner)

	err := repo.CreateExperiment(ctx, experiment)
	if err != nil {
		t.Fatalf("CreateExperiment(ctx, draft) error = %v, want nil", err)
	}
	if experiment.ID != "exp-1" {
		t.Errorf("CreateExperiment ID = %q, want exp-1", experiment.ID)
	}
}

func TestRepositoryCreateExperimentRunningFailsValidation(t *testing.T) {
	ctx := context.Background()
	experiment := &Experiment{
		ID:           "exp-1",
		Name:         "running-test",
		TemplateName: "system_rules",
		Locale:       "en",
		Status:       ExperimentStatusRunning,
	}
	runner := &fakeDBRunner{
		queryContext: func(_ context.Context, query string, _ ...any) (rowsScanner, error) {
			if !strings.Contains(query, "FROM ab_variants") {
				t.Errorf("CreateExperiment running query = %q, want ab_variants lookup", query)
			}
			// Return variants that fail validation (traffic sums to 0)
			return newFakeRows([][]any{
				{"control", "exp-1", "control", 1, 0, true},
			}), nil
		},
		queryRowContext: func(context.Context, string, ...any) rowScanner {
			t.Fatal("CreateExperiment should not insert when validation fails")
			return fakeRow{}
		},
	}
	repo := newRepositoryWithRunner(runner)

	err := repo.CreateExperiment(ctx, experiment)
	if err == nil {
		t.Fatal("CreateExperiment(ctx, running) error = nil, want validation error")
	}
}

func TestRepositoryListExperimentsAll(t *testing.T) {
	ctx := context.Background()
	now := time.Now()
	runner := &fakeDBRunner{
		queryContext: func(_ context.Context, query string, _ ...any) (rowsScanner, error) {
			if strings.Contains(query, "WHERE status = $1") {
				t.Error("ListExperiments('') should not add status filter")
			}
			return newFakeRows([][]any{
				{
					"exp-1", "Test 1", "", "system_rules", "en", string(ExperimentStatusDraft),
					sql.NullTime{}, sql.NullTime{}, sql.NullString{}, now, now,
				},
				{
					"exp-2", "Test 2", "desc", "clarification", "tr", string(ExperimentStatusRunning),
					sql.NullTime{Time: now, Valid: true}, sql.NullTime{}, sql.NullString{}, now, now,
				},
			}), nil
		},
	}
	repo := newRepositoryWithRunner(runner)

	experiments, err := repo.ListExperiments(ctx, "")
	if err != nil {
		t.Fatalf("ListExperiments(ctx, '') error = %v, want nil", err)
	}
	if len(experiments) != 2 {
		t.Fatalf("ListExperiments(ctx, '') len = %d, want 2", len(experiments))
	}
}

func TestRepositoryListExperimentsFiltered(t *testing.T) {
	ctx := context.Background()
	now := time.Now()
	runner := &fakeDBRunner{
		queryContext: func(_ context.Context, query string, args ...any) (rowsScanner, error) {
			if !strings.Contains(query, "WHERE status = $1") {
				t.Error("ListExperiments('running') should add status filter")
			}
			if got, want := args[0], "running"; got != want {
				t.Errorf("ListExperiments status arg = %v, want %v", got, want)
			}
			return newFakeRows([][]any{
				{
					"exp-1", "Test", "desc", "clarification", "en", string(ExperimentStatusRunning),
					sql.NullTime{Time: now, Valid: true}, sql.NullTime{}, sql.NullString{}, now, now,
				},
			}), nil
		},
	}
	repo := newRepositoryWithRunner(runner)

	experiments, err := repo.ListExperiments(ctx, "running")
	if err != nil {
		t.Fatalf("ListExperiments(ctx, 'running') error = %v, want nil", err)
	}
	if len(experiments) != 1 {
		t.Fatalf("ListExperiments(ctx, 'running') len = %d, want 1", len(experiments))
	}
}

func TestRepositoryUpdateVariant(t *testing.T) {
	ctx := context.Background()
	now := time.Now()
	variant := &Variant{
		ID:              "var-1",
		ExperimentID:    "exp-1",
		Name:            "updated-treatment",
		TemplateVersion: 2,
		TrafficPct:      50,
		IsControl:       false,
	}
	runner := &fakeDBRunner{
		queryRowContext: func(_ context.Context, _ string, _ ...any) rowScanner {
			return fakeRow{values: []any{
				"exp-1", "Test", "desc", "clarification", "en", string(ExperimentStatusDraft),
				sql.NullTime{}, sql.NullTime{}, sql.NullString{}, now, now,
			}}
		},
		queryScalarContext: func(_ context.Context, _ string, _ ...any) rowScanner {
			return fakeRow{values: []any{1}} // template version exists
		},
		execContext: func(_ context.Context, query string, _ ...any) (sql.Result, error) {
			if !strings.Contains(query, "UPDATE ab_variants") {
				t.Errorf("UpdateVariant query = %q, want UPDATE ab_variants", query)
			}
			return fakeResult{}, nil
		},
	}
	repo := newRepositoryWithRunner(runner)

	err := repo.UpdateVariant(ctx, variant)
	if err != nil {
		t.Fatalf("UpdateVariant(ctx, variant) error = %v, want nil", err)
	}
}

func TestRepositoryDeleteVariant(t *testing.T) {
	ctx := context.Background()
	runner := &fakeDBRunner{
		execContext: func(_ context.Context, query string, _ ...any) (sql.Result, error) {
			if !strings.Contains(query, "DELETE FROM ab_variants") {
				t.Errorf("DeleteVariant query = %q, want DELETE FROM ab_variants", query)
			}
			return fakeResult{}, nil
		},
	}
	repo := newRepositoryWithRunner(runner)

	err := repo.DeleteVariant(ctx, "var-1")
	if err != nil {
		t.Fatalf("DeleteVariant(ctx, 'var-1') error = %v, want nil", err)
	}
}

func TestNullStringEmpty(t *testing.T) {
	ns := nullString("")
	if ns.Valid {
		t.Fatal("nullString('') should return NullString with Valid=false")
	}
}

func TestNullStringNonEmpty(t *testing.T) {
	ns := nullString("admin-1")
	if !ns.Valid {
		t.Fatal("nullString('admin-1') should return NullString with Valid=true")
	}
	if ns.String != "admin-1" {
		t.Fatalf("nullString('admin-1').String = %q, want 'admin-1'", ns.String)
	}
}

func TestCloseRowsError(t *testing.T) {
	rows := &fakeRows{
		closeErr: errors.New("close failed"),
	}
	var err error
	closeRows(&err, rows, "test rows")
	if err == nil {
		t.Fatal("closeRows should propagate close error")
	}
	if !strings.Contains(err.Error(), "close failed") {
		t.Errorf("closeRows error = %v, want 'close failed'", err)
	}
}

func TestRepositoryUpdateExperimentNonRunning(t *testing.T) {
	ctx := context.Background()
	experiment := &Experiment{
		ID:           "exp-1",
		Name:         "updated-name",
		Description:  "updated desc",
		TemplateName: "system_rules",
		Locale:       "fr",
		Status:       ExperimentStatusDraft,
	}
	runner := &fakeDBRunner{
		execContext: func(_ context.Context, query string, _ ...any) (sql.Result, error) {
			if !strings.Contains(query, "UPDATE ab_experiments") {
				t.Errorf("UpdateExperiment query = %q, want UPDATE ab_experiments", query)
			}
			return fakeResult{}, nil
		},
	}
	repo := newRepositoryWithRunner(runner)

	err := repo.UpdateExperiment(ctx, experiment)
	if err != nil {
		t.Fatalf("UpdateExperiment(ctx, draft) error = %v, want nil", err)
	}
}

func TestRepositoryGetRunningExperimentsForTemplate(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 6, 5, 12, 0, 0, 0, time.UTC)
	runner := &fakeDBRunner{
		queryContext: func(_ context.Context, query string, args ...any) (rowsScanner, error) {
			if !strings.Contains(query, "status = $3") {
				t.Errorf("GetRunningExperimentsForTemplate query = %q, want status filter", query)
			}
			wantArgs := []any{"clarification", "tr", string(ExperimentStatusRunning)}
			for i, want := range wantArgs {
				if got := args[i]; got != want {
					t.Errorf("GetRunningExperimentsForTemplate arg[%d] = %v, want %v", i, got, want)
				}
			}
			return newFakeRows([][]any{
				{
					"exp-1", "Experiment", "description", "clarification", "tr", string(ExperimentStatusRunning),
					sql.NullTime{Time: now, Valid: true}, sql.NullTime{}, sql.NullString{String: "admin-1", Valid: true}, now, now,
				},
			}), nil
		},
	}
	repo := newRepositoryWithRunner(runner)

	experiments, err := repo.GetRunningExperimentsForTemplate(ctx, "clarification", "tr")
	if err != nil {
		t.Fatalf("GetRunningExperimentsForTemplate(ctx, clarification, tr) error = %v, want nil", err)
	}
	if len(experiments) != 1 {
		t.Fatalf("GetRunningExperimentsForTemplate(ctx, clarification, tr) len = %d, want 1", len(experiments))
	}
	got := experiments[0]
	if got.ID != "exp-1" || got.TemplateName != "clarification" || got.Locale != "tr" || got.Status != ExperimentStatusRunning {
		t.Errorf("GetRunningExperimentsForTemplate(ctx, clarification, tr)[0] = %+v, want running clarification/tr experiment", got)
	}
	if got.StartedAt == nil || !got.StartedAt.Equal(now) {
		t.Errorf("GetRunningExperimentsForTemplate(ctx, clarification, tr)[0].StartedAt = %v, want %v", got.StartedAt, now)
	}
}

type fakeDBRunner struct {
	queryRowContext    func(context.Context, string, ...any) rowScanner
	queryScalarContext func(context.Context, string, ...any) rowScanner
	queryContext       func(context.Context, string, ...any) (rowsScanner, error)
	execContext        func(context.Context, string, ...any) (sql.Result, error)
}

func (f *fakeDBRunner) QueryRowContext(ctx context.Context, query string, args ...any) rowScanner {
	if strings.Contains(query, "ai_prompt_templates") && f.queryScalarContext != nil {
		return f.queryScalarContext(ctx, query, args...)
	}
	if f.queryRowContext != nil {
		return f.queryRowContext(ctx, query, args...)
	}
	return fakeRow{err: errors.New("unexpected QueryRowContext")}
}

func (f *fakeDBRunner) QueryContext(ctx context.Context, query string, args ...any) (rowsScanner, error) {
	if f.queryContext != nil {
		return f.queryContext(ctx, query, args...)
	}
	return nil, errors.New("unexpected QueryContext")
}

func (f *fakeDBRunner) ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error) {
	if f.execContext != nil {
		return f.execContext(ctx, query, args...)
	}
	return fakeResult{}, errors.New("unexpected ExecContext")
}

type fakeRow struct {
	values []any
	err    error
}

func (r fakeRow) Scan(dest ...any) error {
	if r.err != nil {
		return r.err
	}
	return assignScanValues(dest, r.values)
}

type fakeRows struct {
	values   [][]any
	index    int
	err      error
	closeErr error
}

func newFakeRows(values [][]any) *fakeRows {
	return &fakeRows{values: values, index: -1}
}

func (r *fakeRows) Next() bool {
	r.index++
	return r.index < len(r.values)
}

func (r *fakeRows) Scan(dest ...any) error {
	if r.index < 0 || r.index >= len(r.values) {
		return errors.New("fake rows scan without current row")
	}
	return assignScanValues(dest, r.values[r.index])
}

func (r *fakeRows) Err() error {
	return r.err
}

func (r *fakeRows) Close() error {
	return r.closeErr
}

type fakeResult struct{}

func (fakeResult) LastInsertId() (int64, error) {
	return 0, nil
}

func (fakeResult) RowsAffected() (int64, error) {
	return 0, nil
}

func assignScanValues(dest []any, values []any) error {
	if len(dest) != len(values) {
		return errors.New("scan destination count does not match values")
	}
	for i := range dest {
		switch d := dest[i].(type) {
		case *string:
			switch v := values[i].(type) {
			case string:
				*d = v
			case ExperimentStatus:
				*d = string(v)
			default:
				return errors.New("scan value is not string")
			}
		case *int:
			v, ok := values[i].(int)
			if !ok {
				return errors.New("scan value is not int")
			}
			*d = v
		case *bool:
			v, ok := values[i].(bool)
			if !ok {
				return errors.New("scan value is not bool")
			}
			*d = v
		case *time.Time:
			v, ok := values[i].(time.Time)
			if !ok {
				return errors.New("scan value is not time.Time")
			}
			*d = v
		case *sql.NullString:
			v, ok := values[i].(sql.NullString)
			if !ok {
				return errors.New("scan value is not sql.NullString")
			}
			*d = v
		case *sql.NullTime:
			v, ok := values[i].(sql.NullTime)
			if !ok {
				return errors.New("scan value is not sql.NullTime")
			}
			*d = v
		default:
			return errors.New("unsupported scan destination")
		}
	}
	return nil
}
