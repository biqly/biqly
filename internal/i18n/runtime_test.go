package i18n

import (
	"context"
	"errors"
	"slices"
	"testing"
)

type stubRuntimeProvider struct {
	locales    []RuntimeLocale
	bundles    map[Locale]map[string]any
	localesErr error
	bundlesErr error
	calls      int
}

func (s *stubRuntimeProvider) Locales(context.Context) ([]RuntimeLocale, error) {
	s.calls++
	return s.locales, s.localesErr
}

func (s *stubRuntimeProvider) Bundles(context.Context) (map[Locale]map[string]any, error) {
	return s.bundles, s.bundlesErr
}

func withProvider(t *testing.T, p RuntimeProvider) {
	t.Helper()
	SetRuntimeProvider(p)
	t.Cleanup(func() { SetRuntimeProvider(nil) })
}

func deLocaleRow(enabled bool) RuntimeLocale {
	return RuntimeLocale{
		Profile: LocaleProfile{
			Locale:          Locale("de"),
			Label:           "Deutsch",
			ShortLabel:      "DE",
			QuestionLetters: "äöüß",
			QuestionSignals: []string{" wie viele ", " zeige "},
		},
		Enabled: enabled,
	}
}

// A registry row makes a brand-new locale fully supported — parsing, context
// propagation, profiles — without a release (DİL-3 acceptance).
func TestRuntimeRegistryAddsNewLocale(t *testing.T) {
	withProvider(t, &stubRuntimeProvider{locales: []RuntimeLocale{deLocaleRow(true)}})

	if !IsSupported("de") {
		t.Fatal("registry-added locale must be supported")
	}
	if got := ParseLocale("de-DE"); got != Locale("de") {
		t.Fatalf("ParseLocale(de-DE) = %q, want de", got)
	}
	if got := FromContext(WithLocale(context.Background(), "de")); got != Locale("de") {
		t.Fatalf("FromContext = %q, want de to survive context round-trip", got)
	}
	profile, ok := LocaleProfileFor("de")
	if !ok || profile.Label != "Deutsch" {
		t.Fatalf("LocaleProfileFor(de) = %+v, %v", profile, ok)
	}
	if got := ActiveLocales(); !slices.Contains(got, Locale("de")) || got[0] != LocaleEN {
		t.Fatalf("ActiveLocales() = %v, want embedded order first plus de", got)
	}
}

// T resolves: DB bundle for the locale → embedded → DefaultLocale chain → key.
func TestRuntimeBundleLookupChain(t *testing.T) {
	withProvider(t, &stubRuntimeProvider{
		locales: []RuntimeLocale{deLocaleRow(true)},
		bundles: map[Locale]map[string]any{
			"de": {"clarification": map[string]any{"ambiguity_reason": "Mehrdeutige Frage."}},
			"tr": {"clarification": map[string]any{"ambiguity_reason": "DB-TR-override"}},
		},
	})

	if got := T("de", "clarification.ambiguity_reason"); got != "Mehrdeutige Frage." {
		t.Fatalf("T(de) = %q, want DB bundle hit", got)
	}
	// DB bundle overrides the embedded TR catalog per-key…
	if got := T(LocaleTR, "clarification.ambiguity_reason"); got != "DB-TR-override" {
		t.Fatalf("T(tr) = %q, want DB override", got)
	}
	// …while keys absent from the DB bundle fall back to the embedded catalog.
	if got := T(LocaleTR, "clarification.needs_clarification_warning"); !slices.Contains([]string{got}, "Sorgu oluşturulabilmesi için bu sorunun netleştirilmesi gerekiyor.") {
		t.Fatalf("T(tr, embedded key) = %q, want embedded TR text", got)
	}
	// de has no embedded catalog: unknown keys resolve through DefaultLocale.
	if got := T("de", "clarification.needs_clarification_warning"); got != T(LocaleEN, "clarification.needs_clarification_warning") {
		t.Fatalf("T(de, missing) = %q, want EN fallback", got)
	}
	// Completely unknown keys return the key itself.
	if got := T("de", "no.such.key"); got != "no.such.key" {
		t.Fatalf("T(unknown) = %q", got)
	}
}

func TestRuntimeDisableLocale(t *testing.T) {
	withProvider(t, &stubRuntimeProvider{locales: []RuntimeLocale{
		{Profile: localeProfiles[LocaleTR], Enabled: false},
		{Profile: localeProfiles[LocaleEN], Enabled: false}, // must be ignored: EN is the terminal fallback
	}})

	if IsSupported(LocaleTR) {
		t.Fatal("disabled registry locale must not be supported")
	}
	if !IsSupported(LocaleEN) {
		t.Fatal("the default locale cannot be disabled")
	}
	// Embedded TR catalog still serves texts (catalog ≠ registry): T falls back.
	if got := FromContext(WithLocale(context.Background(), LocaleTR)); got != DefaultLocale {
		t.Fatalf("FromContext(disabled tr) = %q, want default", got)
	}
}

func TestRuntimeProviderErrorFallsBackToEmbedded(t *testing.T) {
	withProvider(t, &stubRuntimeProvider{localesErr: errors.New("db down"), bundlesErr: errors.New("db down")})

	if !IsSupported(LocaleTR) || !IsSupported(LocaleEN) {
		t.Fatal("embedded locales must survive provider errors")
	}
	if IsSupported("de") {
		t.Fatal("no registry rows should be visible on provider error")
	}
	if got := T(LocaleTR, "clarification.ambiguity_reason"); got == "clarification.ambiguity_reason" {
		t.Fatal("embedded TR catalog must keep serving on provider error")
	}
}

func TestRuntimeSnapshotCachedAndInvalidated(t *testing.T) {
	p := &stubRuntimeProvider{locales: []RuntimeLocale{deLocaleRow(true)}}
	withProvider(t, p)

	IsSupported("de")
	IsSupported("de")
	if p.calls != 1 {
		t.Fatalf("provider calls = %d, want 1 (snapshot cached)", p.calls)
	}
	InvalidateRuntime()
	IsSupported("de")
	if p.calls != 2 {
		t.Fatalf("provider calls = %d, want 2 after InvalidateRuntime", p.calls)
	}
}
