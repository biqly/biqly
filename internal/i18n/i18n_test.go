package i18n

import (
	"context"
	"testing"
)

func TestT_FallsBackToEnglish(t *testing.T) {
	got := T(LocaleTR, "datasources.title")
	if got == "datasources.title" {
		t.Fatalf("expected localized string, got raw key")
	}
}

func TestT_UnknownKeyReturnsKey(t *testing.T) {
	got := T(LocaleEN, "does.not.exist")
	if got != "does.not.exist" {
		t.Fatalf("expected raw key for unknown lookup, got %q", got)
	}
}

func TestTf_Interpolates(t *testing.T) {
	got := Tf(LocaleEN, "errmsg.unknown_dimension", map[string]any{"Name": "foo"})
	want := "unknown dimension: foo"
	if got != want {
		t.Fatalf("Tf = %q; want %q", got, want)
	}
}

func TestTf_TurkishInterpolation(t *testing.T) {
	got := Tf(LocaleTR, "errmsg.unknown_dimension", map[string]any{"Name": "x"})
	if got == "" || got == "errmsg.unknown_dimension" {
		t.Fatalf("expected Turkish interpolation, got %q", got)
	}
}

func TestParseAcceptLanguage(t *testing.T) {
	cases := []struct {
		header string
		want   Locale
	}{
		{"", DefaultLocale},
		{"tr-TR,tr;q=0.9,en;q=0.8", LocaleTR},
		{"en-US,en;q=0.9", LocaleEN},
		{"fr-FR,fr;q=0.9", DefaultLocale},
		{"de-DE;q=0.9,tr;q=0.8", LocaleTR},
		{"*", DefaultLocale},
	}
	for _, c := range cases {
		got := ParseAcceptLanguage(c.header)
		if got != c.want {
			t.Errorf("ParseAcceptLanguage(%q) = %q; want %q", c.header, got, c.want)
		}
	}
}

func TestParseLocaleNormalizes(t *testing.T) {
	if got := ParseLocale("TR"); got != LocaleTR {
		t.Errorf("upper TR not normalized: %q", got)
	}
	if got := ParseLocale("tr_TR"); got != LocaleTR {
		t.Errorf("underscore tag not normalized: %q", got)
	}
	if got := ParseLocale(""); got != DefaultLocale {
		t.Errorf("empty did not return default: %q", got)
	}
}

func TestContextRoundTrip(t *testing.T) {
	ctx := WithLocale(context.Background(), LocaleTR)
	if got := FromContext(ctx); got != LocaleTR {
		t.Fatalf("FromContext = %q; want tr", got)
	}
	if got := FromContext(context.Background()); got != DefaultLocale {
		t.Fatalf("bare context = %q; want default", got)
	}
}
