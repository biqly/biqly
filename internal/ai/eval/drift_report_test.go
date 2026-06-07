package eval_test

import (
	"testing"

	evalpkg "github.com/biqly/biqly/internal/ai/eval"
)

func TestCompareSnapshotsDetectsRegression(t *testing.T) {
	baseline := evalpkg.RunSnapshot{
		Suite: evalpkg.NightlySuiteName,
		Cases: map[string]evalpkg.CaseSnapshot{
			"a": {Question: "q1", Match: true},
			"b": {Question: "q2", Match: true},
		},
	}
	current := evalpkg.RunSnapshot{
		Suite: evalpkg.NightlySuiteName,
		Cases: map[string]evalpkg.CaseSnapshot{
			"a": {Question: "q1", Match: true},
			"b": {Question: "q2", Match: false, Reason: "wrong metric"},
		},
	}
	report := evalpkg.CompareSnapshots(baseline, current)
	if len(report.NewFailures) != 1 {
		t.Fatalf("expected 1 new failure, got %d", len(report.NewFailures))
	}
	if report.NewFailures[0].CaseID != "b" {
		t.Fatalf("expected case b, got %q", report.NewFailures[0].CaseID)
	}
}

func TestBaselineSnapshotFromCases(t *testing.T) {
	cases := []evalpkg.GoldenCase{{ID: "x", Question: "q"}}
	snap := evalpkg.BaselineSnapshotFromCases("suite", cases)
	if snap.Passed != 1 || !snap.Cases["x"].Match {
		t.Fatalf("unexpected baseline snapshot: %+v", snap)
	}
}

func TestNightlyBaselineMatchesSuite(t *testing.T) {
	cases := evalpkg.NightlyCases()
	snap, err := evalpkg.LoadRunSnapshot("../../../testdata/eval/nightly_baseline.json")
	if err != nil {
		t.Fatalf("load baseline: %v", err)
	}
	if snap.Suite != evalpkg.NightlySuiteName {
		t.Fatalf("suite = %q, want %q", snap.Suite, evalpkg.NightlySuiteName)
	}
	for _, c := range cases {
		got, ok := snap.Cases[c.ID]
		if !ok {
			t.Errorf("baseline missing case %q", c.ID)
			continue
		}
		if !got.Match {
			t.Errorf("baseline case %q should expect match=true", c.ID)
		}
	}
}
