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
	values [][]any
	index  int
	err    error
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
	return nil
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
