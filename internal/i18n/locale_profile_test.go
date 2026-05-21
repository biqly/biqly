package i18n

import "testing"

func TestSupportedLocaleProfilesCoverSupportedLocales(t *testing.T) {
	profiles := SupportedLocaleProfiles()
	if len(profiles) != len(SupportedLocales) {
		t.Fatalf("profiles = %d, supported locales = %d", len(profiles), len(SupportedLocales))
	}
	for _, loc := range SupportedLocales {
		profile, ok := LocaleProfileFor(loc)
		if !ok {
			t.Fatalf("missing locale profile for %q", loc)
		}
		if profile.Label == "" || profile.ShortLabel == "" {
			t.Fatalf("locale %q has incomplete labels: %#v", loc, profile)
		}
	}
}

func TestSupportedLocaleCodesAreDerivedFromSupportedLocales(t *testing.T) {
	got := SupportedLocaleCodes()
	if len(got) != len(SupportedLocales) {
		t.Fatalf("codes = %d, supported locales = %d", len(got), len(SupportedLocales))
	}
	for i, loc := range SupportedLocales {
		if got[i] != string(loc) {
			t.Fatalf("codes[%d] = %q, want %q", i, got[i], loc)
		}
	}
}
