package prompt

import (
	"context"
	"strings"
	"testing"

	"github.com/biqly/biqly/internal/ai/lexicon"
	"github.com/biqly/biqly/internal/i18n"
)

type bridgeFakeRepo struct {
	rows map[string]string // name + "\x00" + locale → content
}

func (r *bridgeFakeRepo) CountPromptTemplates(context.Context) (int, error) {
	return len(r.rows), nil
}

func (r *bridgeFakeRepo) GetPromptTemplate(_ context.Context, name string, loc i18n.Locale) (string, error) {
	return r.rows[name+"\x00"+string(loc)], nil
}

// Rows report version 2 ("admin-edited") so the seed's version-1 refresh rule
// stays out of these bridge-behavior tests.
func (r *bridgeFakeRepo) GetPromptTemplateVersion(_ context.Context, name string, loc i18n.Locale) (string, int, error) {
	return r.rows[name+"\x00"+string(loc)], 2, nil
}

func (r *bridgeFakeRepo) UpsertPromptTemplate(_ context.Context, name string, loc i18n.Locale, content string) error {
	r.rows[name+"\x00"+string(loc)] = content
	return nil
}

// A locale without its own template is served the EN content plus the language
// bridge note; an admin-authored row for that locale drops the note (DİL-4).
func TestDBPromptStoreUnknownLocaleGetsLanguageBridge(t *testing.T) {
	repo := &bridgeFakeRepo{rows: map[string]string{
		"system_rules\x00en":  "EN RULES",
		"output_format\x00en": "EN OUTPUT",
	}}
	store := NewDBPromptTemplateStore(repo)

	rules := store.Snapshot(context.Background(), "de", "system_rules")
	if !strings.HasPrefix(rules.Content, "EN RULES") || !strings.Contains(rules.Content, "## User Language") || !strings.Contains(rules.Content, "(de)") {
		t.Fatalf("Snapshot(de, system_rules) = %q, want EN content + bridge note", rules.Content)
	}
	// Structural sections carry no user-facing wording — no note.
	if out := store.Snapshot(context.Background(), "de", "output_format"); strings.Contains(out.Content, "## User Language") {
		t.Fatalf("output_format must not get a bridge note, got %q", out.Content)
	}

	// Admin authors a German template: it takes precedence, note disappears.
	repo.rows["system_rules\x00de"] = "DE REGELN"
	fresh := NewDBPromptTemplateStore(repo)
	if got := fresh.Snapshot(context.Background(), "de", "system_rules"); got.Content != "DE REGELN" {
		t.Fatalf("Snapshot(de) after authoring = %q, want DE REGELN without note", got.Content)
	}
}

func TestDBPromptStoreEmbeddedLocaleHasNoBridge(t *testing.T) {
	store := NewDBPromptTemplateStore(&bridgeFakeRepo{rows: map[string]string{}})
	if got := store.Snapshot(context.Background(), i18n.LocaleTR, "system_rules"); strings.Contains(got.Content, "## User Language") {
		t.Fatal("embedded TR template must not get a bridge note")
	}
}

func TestEmbedPromptStoreUnknownLocaleGetsLanguageBridge(t *testing.T) {
	store := embedPromptStore{}
	clar := store.Snapshot(context.Background(), "de", "clarification")
	if !strings.Contains(clar.Content, "## User Language") || !strings.Contains(clar.Content, "(de)") {
		t.Fatalf("embed Snapshot(de, clarification) = %q, want bridge note", clar.Content)
	}
	if tr := store.Snapshot(context.Background(), i18n.LocaleTR, "clarification"); strings.Contains(tr.Content, "## User Language") {
		t.Fatal("embedded TR clarification must not get a bridge note")
	}
	if out := store.Snapshot(context.Background(), "de", "output_format"); strings.Contains(out.Content, "## User Language") {
		t.Fatal("output_format must not get a bridge note")
	}
}

// Prompt hints draw their vocabulary from the NL lexicon union, so every
// active language contributes examples without code changes.
func TestLexiconHintSamplesSpansLocales(t *testing.T) {
	got := lexiconHintSamples(1,
		[2]string{lexicon.DomainIntentToken, "count"},
		[2]string{lexicon.DomainSoftDelete, "ts_deleted"},
		[2]string{lexicon.DomainGrainSynonym, "month"},
	)
	for _, want := range []string{"“count”", "“adet”", "“deleted”", "“kaldirilan”", "“month”", "“ay bazında”"} {
		if !strings.Contains(got, want) {
			t.Fatalf("lexiconHintSamples() = %q, missing %s", got, want)
		}
	}
}
