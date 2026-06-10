package lexicon

import (
	"context"
	"testing"

	"github.com/biqly/biqly/internal/metadata"
)

type fakeSeedRepo struct {
	countVal    int
	countCalls  int
	upserted    []metadata.NLLexiconEntry
	upsertCalls int
}

func (f *fakeSeedRepo) CountNLLexicon(context.Context) (int, error) {
	f.countCalls++
	return f.countVal, nil
}

func (f *fakeSeedRepo) UpsertNLLexiconEntries(_ context.Context, entries []metadata.NLLexiconEntry) error {
	f.upsertCalls++
	f.upserted = append(f.upserted, entries...)
	return nil
}

func TestSeedSkipsNonEmptyTable(t *testing.T) {
	repo := &fakeSeedRepo{countVal: 12}
	if err := Seed(context.Background(), repo); err != nil {
		t.Fatal(err)
	}
	if repo.upsertCalls != 0 {
		t.Fatalf("upsertCalls = %d, want 0 when table already seeded", repo.upsertCalls)
	}
}

func TestSeedPopulatesEmptyTable(t *testing.T) {
	repo := &fakeSeedRepo{}
	if err := Seed(context.Background(), repo); err != nil {
		t.Fatal(err)
	}
	if len(repo.upserted) != len(DefaultEntries()) {
		t.Fatalf("seeded %d rows, want %d", len(repo.upserted), len(DefaultEntries()))
	}
	for _, row := range repo.upserted {
		if !row.IsActive || row.Locale == "" || row.Domain == "" || row.Key == "" || len(row.Value) == 0 {
			t.Fatalf("malformed seeded row: %+v", row)
		}
	}
}
