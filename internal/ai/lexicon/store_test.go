package lexicon

import (
	"context"
	"encoding/json"
	"errors"
	"slices"
	"testing"

	"github.com/biqly/biqly/internal/metadata"
)

func TestStaticStoreServesUnionOfLocales(t *testing.T) {
	store := NewStaticStore()

	// Union across en+tr must equal the previously hardcoded mixed lists.
	month := store.Terms(DomainGrainSynonym, "month")
	for _, want := range []string{"month", "monthly", "ay", "aylık", "per month", "ay bazında"} {
		if !slices.Contains(month, want) {
			t.Fatalf("month grain synonyms missing %q: %v", want, month)
		}
	}
	deleted := store.Terms(DomainSoftDelete, "ts_deleted")
	for _, want := range []string{"deleted", "soft-delete", "silinen", "kaldirilan"} {
		if !slices.Contains(deleted, want) {
			t.Fatalf("ts_deleted terms missing %q: %v", want, deleted)
		}
	}
}

func TestStaticStoreTemporalPhrasesSortedWithKeys(t *testing.T) {
	phrases := NewStaticStore().TemporalPhrases()
	if len(phrases) == 0 {
		t.Fatal("expected default temporal phrases")
	}
	for i := 1; i < len(phrases); i++ {
		if phrases[i-1].Phrase >= phrases[i].Phrase {
			t.Fatalf("phrases not sorted: %q before %q", phrases[i-1].Phrase, phrases[i].Phrase)
		}
	}
	var gecenAy *TemporalPhrase
	for i := range phrases {
		if phrases[i].Phrase == "geçen ay" {
			gecenAy = &phrases[i]
		}
	}
	if gecenAy == nil || !slices.Equal(gecenAy.InterpretationKeys, []string{"prev_calendar_month", "rolling_30d"}) {
		t.Fatalf("'geçen ay' interpretation keys wrong: %+v", gecenAy)
	}
}

func TestTermsReturnsCopy(t *testing.T) {
	store := NewStaticStore()
	first := store.Terms(DomainRowCount, "row_count")
	first[0] = "mutated"
	second := store.Terms(DomainRowCount, "row_count")
	if second[0] == "mutated" {
		t.Fatal("Terms must return a copy, snapshot was mutated")
	}
}

type fakeLexiconRepo struct {
	calls int
	rows  []metadata.NLLexiconEntry
	err   error
}

func (f *fakeLexiconRepo) ListActiveNLLexicon(context.Context) ([]metadata.NLLexiconEntry, error) {
	f.calls++
	return f.rows, f.err
}

func mustRow(t *testing.T, locale, domain, key string, value any) metadata.NLLexiconEntry {
	t.Helper()
	raw, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return metadata.NLLexiconEntry{Locale: locale, Domain: domain, Key: key, Value: raw, IsActive: true}
}

func TestDBStoreFallsBackToDefaultsOnError(t *testing.T) {
	store := NewDBStore(&fakeLexiconRepo{err: errors.New("db down")})
	if got := store.Terms(DomainGrainSynonym, "month"); !slices.Contains(got, "ay") {
		t.Fatalf("expected embedded defaults on DB error, got %v", got)
	}
	if got := store.TemporalPhrases(); len(got) == 0 {
		t.Fatal("expected embedded temporal phrases on DB error")
	}
}

func TestDBStorePerDomainFallback(t *testing.T) {
	// DB has only a grain_synonym row: that domain comes from the DB, every
	// other domain falls back to the embedded defaults (ADR-0001 K5).
	repo := &fakeLexiconRepo{rows: []metadata.NLLexiconEntry{
		mustRow(t, "de", DomainGrainSynonym, "month", map[string]any{"terms": []string{"monat", "monatlich"}}),
	}}
	store := NewDBStore(repo)

	month := store.Terms(DomainGrainSynonym, "month")
	if !slices.Equal(month, []string{"monat", "monatlich"}) {
		t.Fatalf("expected DB-defined month terms, got %v", month)
	}
	if got := store.Terms(DomainSoftDelete, "ts_deleted"); !slices.Contains(got, "silinen") {
		t.Fatalf("expected embedded soft_delete fallback, got %v", got)
	}
	if got := store.TemporalPhrases(); len(got) == 0 {
		t.Fatal("expected embedded temporal phrase fallback")
	}
}

func TestDBStoreUnionAcrossLocalesAndInvalidate(t *testing.T) {
	repo := &fakeLexiconRepo{rows: []metadata.NLLexiconEntry{
		mustRow(t, "en", DomainIntentToken, "count", map[string]any{"terms": []string{"count"}}),
		mustRow(t, "de", DomainIntentToken, "count", map[string]any{"terms": []string{"wie viele", "anzahl"}}),
	}}
	store := NewDBStore(repo)

	got := store.Terms(DomainIntentToken, "count")
	if !slices.Equal(got, []string{"count", "wie viele", "anzahl"}) {
		t.Fatalf("expected locale union, got %v", got)
	}
	if repo.calls != 1 {
		t.Fatalf("repo calls = %d, want 1 (cached)", repo.calls)
	}

	store.Terms(DomainIntentToken, "count")
	if repo.calls != 1 {
		t.Fatalf("repo calls = %d, want 1 after cached lookup", repo.calls)
	}

	store.Invalidate()
	store.Terms(DomainIntentToken, "count")
	if repo.calls != 2 {
		t.Fatalf("repo calls = %d, want 2 after Invalidate", repo.calls)
	}
}

func TestDBStoreSkipsMalformedRows(t *testing.T) {
	bad := metadata.NLLexiconEntry{Locale: "en", Domain: DomainIntentToken, Key: "count", Value: []byte("{not json"), IsActive: true}
	repo := &fakeLexiconRepo{rows: []metadata.NLLexiconEntry{
		bad,
		mustRow(t, "tr", DomainIntentToken, "count", map[string]any{"terms": []string{"kaç"}}),
	}}
	store := NewDBStore(repo)
	if got := store.Terms(DomainIntentToken, "count"); !slices.Equal(got, []string{"kaç"}) {
		t.Fatalf("expected malformed row skipped, got %v", got)
	}
}

func TestSetActiveRestoresPrevious(t *testing.T) {
	custom := NewStaticStore()
	prev := SetActive(custom)
	defer SetActive(prev)
	if Active() != custom {
		t.Fatal("SetActive did not swap the active store")
	}
}
