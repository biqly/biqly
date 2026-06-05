package prompt

import (
	"context"
	"strings"
	"testing"

	"github.com/biqly/biqly/internal/i18n"
	"github.com/biqly/biqly/internal/semantic"
)

type testPromptStore struct {
	templates map[string]PromptTemplateSnapshot
}

func (s testPromptStore) Template(ctx context.Context, loc i18n.Locale, name string) string {
	return s.Snapshot(ctx, loc, name).Content
}

func (s testPromptStore) Snapshot(_ context.Context, loc i18n.Locale, name string) PromptTemplateSnapshot {
	if v, ok := s.templates[name+"\x00"+string(loc)]; ok {
		return v
	}
	return PromptTemplateSnapshot{Name: name, Locale: loc, Content: "", Version: 0}
}

func (s testPromptStore) SnapshotForUser(ctx context.Context, _ string, loc i18n.Locale, name string) PromptTemplateSnapshot {
	return s.Snapshot(ctx, loc, name)
}



func withPromptStore(t *testing.T, store PromptTemplateStore) {
	t.Helper()
	prev := getActivePromptStore()
	SetPromptTemplateStore(store)
	t.Cleanup(func() { SetPromptTemplateStore(prev) })
}

func TestKnownPromptTemplateNamesIncludesRetryAndClarification(t *testing.T) {
	got := strings.Join(KnownPromptTemplateNames(), ",")
	for _, name := range []string{"system_rules", "output_format", "retry", "clarification", "prompt_layout"} {
		if !strings.Contains(got, name) {
			t.Fatalf("expected %s in editable prompt template names, got %q", name, got)
		}
	}
}

func TestTurkishEditablePromptDefaultsAreLocalized(t *testing.T) {
	tests := []struct {
		name      string
		mustHave  string
		mustAvoid string
	}{
		{name: "system_rules", mustHave: "Bir Business Intelligence sorgu motorusun", mustAvoid: "Convert the user's natural language question"},
		{name: "clarification", mustHave: "Kullanıcı Sorusu", mustAvoid: "User Question"},
		{name: "retry", mustHave: "Önceki Deneme", mustAvoid: "Previous Attempt"},
		{name: "output_format", mustHave: "Çıktı Formatı", mustAvoid: "Output Format"},
		{name: "prompt_layout", mustHave: "Semantik Model", mustAvoid: "Semantic Model"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tr := promptTemplateFromEmbed(i18n.LocaleTR, tc.name)
			en := promptTemplateFromEmbed(i18n.LocaleEN, tc.name)
			if tr == "" {
				t.Fatalf("missing Turkish template for %s", tc.name)
			}
			if tr == en {
				t.Fatalf("Turkish template %s must not be identical to English default", tc.name)
			}
			if !strings.Contains(tr, tc.mustHave) {
				t.Fatalf("Turkish template %s does not contain %q: %q", tc.name, tc.mustHave, tr)
			}
			if strings.Contains(tr, tc.mustAvoid) {
				t.Fatalf("Turkish template %s still contains English marker %q: %q", tc.name, tc.mustAvoid, tr)
			}
		})
	}
}

func TestBuildRetryUsesEditableTemplate(t *testing.T) {
	withPromptStore(t, testPromptStore{templates: map[string]PromptTemplateSnapshot{
		"retry\x00tr": {
			Name:    "retry",
			Locale:  i18n.LocaleTR,
			Content: "CUSTOM RETRY {{.OriginalPrompt}} {{.LastResponse}} {{.ValidationError}}",
			Version: 7,
		},
	}})

	pb := &PromptBuilder{}
	got := pb.BuildRetry(context.Background(), i18n.LocaleTR, "ORIGINAL", "BAD", "BROKEN")
	if !strings.Contains(got, "CUSTOM RETRY ORIGINAL BAD BROKEN") {
		t.Fatalf("expected retry template to render from store, got %q", got)
	}
}

func TestBuildClarificationUsesEditableTemplate(t *testing.T) {
	withPromptStore(t, testPromptStore{templates: map[string]PromptTemplateSnapshot{
		"clarification\x00tr": {
			Name:    "clarification",
			Locale:  i18n.LocaleTR,
			Content: "CUSTOM CLARIFY {{.Question}} {{.ModelName}} {{.FailureReason}}",
			Version: 3,
		},
	}})

	pb := &PromptBuilder{}
	model := &semantic.SemanticModel{Name: "tweets"}
	got := pb.BuildClarification(context.Background(), i18n.LocaleTR, "silinen tweetler", model, "ambiguous")
	if !strings.Contains(got, "CUSTOM CLARIFY silinen tweetler tweets ambiguous") {
		t.Fatalf("expected clarification template to render from store, got %q", got)
	}
}

func TestPromptTemplateBundleVersionsReturnsActiveVersions(t *testing.T) {
	withPromptStore(t, testPromptStore{templates: map[string]PromptTemplateSnapshot{
		"system_rules\x00tr":  {Name: "system_rules", Locale: i18n.LocaleTR, Version: 2},
		"output_format\x00tr": {Name: "output_format", Locale: i18n.LocaleTR, Version: 5},
		"retry\x00tr":         {Name: "retry", Locale: i18n.LocaleTR, Version: 8},
		"clarification\x00tr": {Name: "clarification", Locale: i18n.LocaleTR, Version: 13},
		"prompt_layout\x00tr": {Name: "prompt_layout", Locale: i18n.LocaleTR, Version: 14},
	}})

	got := PromptTemplateBundleVersions(context.Background(), i18n.LocaleTR)
	if got["system_rules"] != 2 || got["output_format"] != 5 || got["retry"] != 8 || got["clarification"] != 13 || got["prompt_layout"] != 14 {
		t.Fatalf("unexpected version bundle: %#v", got)
	}
}

type mockPromptTemplateRepo struct {
	count   int
	data    map[string]string
	upserts []struct {
		name    string
		locale  i18n.Locale
		content string
	}
}

func (m *mockPromptTemplateRepo) CountPromptTemplates(_ context.Context) (int, error) {
	return m.count, nil
}

func (m *mockPromptTemplateRepo) GetPromptTemplate(_ context.Context, name string, loc i18n.Locale) (string, error) {
	return m.data[name+"\x00"+string(loc)], nil
}

func (m *mockPromptTemplateRepo) UpsertPromptTemplate(_ context.Context, name string, loc i18n.Locale, content string) error {
	m.upserts = append(m.upserts, struct {
		name    string
		locale  i18n.Locale
		content string
	}{name, loc, content})
	return nil
}

func TestSeedPromptTemplatesUpdatesEnglishFallback(t *testing.T) {
	enRules := promptTemplateFromEmbed(i18n.LocaleEN, "system_rules")
	trRules := promptTemplateFromEmbed(i18n.LocaleTR, "system_rules")
	if enRules == "" || trRules == "" || enRules == trRules {
		t.Fatalf("precondition failed: embed templates must exist and differ")
	}

	repo := &mockPromptTemplateRepo{
		count: 5,
		data: map[string]string{
			"system_rules\x00tr": enRules,
		},
	}

	err := SeedPromptTemplatesFromEmbed(context.Background(), repo)
	if err != nil {
		t.Fatalf("unexpected seed error: %v", err)
	}

	var foundTRSystemRules bool
	for _, u := range repo.upserts {
		if u.name == "system_rules" && u.locale == i18n.LocaleTR {
			foundTRSystemRules = true
			if u.content != trRules {
				t.Fatalf("expected Turkish system rules, got: %s", u.content)
			}
		}
	}
	if !foundTRSystemRules {
		t.Fatalf("expected system_rules/tr to be updated/upserted")
	}
}
