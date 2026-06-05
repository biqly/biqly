// Package i18n provides a lightweight, dependency-free localization layer
// for backend-rendered messages. Locale bundles are embedded JSON files
// containing nested keys (e.g. "datasources.title").
//
// Usage:
//
//	loc := i18n.FromContext(ctx)        // Locale("tr") | Locale("en")
//	msg := i18n.T(loc, "datasources.title")
//	msg := i18n.Tf(loc, "errmsg.unknown_dimension", map[string]any{"Name": "foo"})
//
// Bundles fall back: requested locale → DefaultLocale → key itself.
package i18n

import (
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"sync"
)

// Locale is a BCP-47 language tag subset (e.g. "tr", "en").
type Locale string

const (
	LocaleTR      Locale = "tr"
	LocaleEN      Locale = "en"
	DefaultLocale        = LocaleEN
)

// SupportedLocales lists all locales that have an embedded bundle.
var SupportedLocales = []Locale{LocaleEN, LocaleTR}

// LocaleProfile contains UI labels and lightweight NL detection hints for one locale.
type LocaleProfile struct {
	Locale                   Locale
	Label                    string
	ShortLabel               string
	QuestionLetters          string
	QuestionSignals          []string
	UsesMetadataTranslations bool
}

var localeProfiles = map[Locale]LocaleProfile{
	LocaleEN: {
		Locale:     LocaleEN,
		Label:      "English",
		ShortLabel: "EN",
		QuestionSignals: []string{
			" show ", " list ", " total ", " average ", " count ", " by ", " per ",
			" today ", " yesterday ", " last ", " between ", " customer ", " order ",
			" sales ", " product ", " tweet ", " user ", " amount ",
		},
	},
	LocaleTR: {
		Locale:                   LocaleTR,
		Label:                    "Türkçe",
		ShortLabel:               "TR",
		QuestionLetters:          "ıİşŞğĞüÜöÖçÇ",
		UsesMetadataTranslations: true,
		QuestionSignals: []string{
			" kaç ", " kaç? ", " adet ", " göster ", " listele ", " toplam ",
			" ortalama ", " günlük ", " aylık ", " yıllık ", " dün ", " bugün ", " geçen ",
			" son ", " filtre ", " göre ", " arasında ", " tarih ", " müşteri ", " sipariş ",
			" satis ", " satış ", " ürün ", " tweet ", " kullanıcı ", " sayısı ", " miktar ",
			" silinen ", " silinmis ", " silinmiş ", " silindi ", " kaldırılan ", " kaldirilan ",
		},
	},
}

// SupportedLocaleProfiles returns profile data in SupportedLocales order.
func SupportedLocaleProfiles() []LocaleProfile {
	out := make([]LocaleProfile, 0, len(SupportedLocales))
	for _, loc := range SupportedLocales {
		if profile, ok := LocaleProfileFor(loc); ok {
			out = append(out, profile)
		}
	}
	return out
}

// LocaleProfileFor returns the profile for a supported locale.
func LocaleProfileFor(loc Locale) (LocaleProfile, bool) {
	profile, ok := localeProfiles[loc]
	return profile, ok
}

// SupportedLocaleCodes returns supported locale codes in priority order.
func SupportedLocaleCodes() []string {
	out := make([]string, 0, len(SupportedLocales))
	for _, loc := range SupportedLocales {
		out = append(out, string(loc))
	}
	return out
}

// IsSupported reports whether loc is configured in SupportedLocales.
func IsSupported(loc Locale) bool {
	return isSupported(loc)
}

//go:embed locales/*.json
var localeFS embed.FS

type bundle map[string]any

var (
	bundlesOnce sync.Once
	bundles     map[Locale]bundle
	errBundles  error
)

func loadBundles() {
	bundles = make(map[Locale]bundle, len(SupportedLocales))
	for _, loc := range SupportedLocales {
		raw, err := localeFS.ReadFile("locales/" + string(loc) + ".json")
		if err != nil {
			errBundles = fmt.Errorf("load locale %q: %w", loc, err)
			return
		}
		var b bundle
		if err := json.Unmarshal(raw, &b); err != nil {
			errBundles = fmt.Errorf("parse locale %q: %w", loc, err)
			return
		}
		bundles[loc] = b
	}
}

func getBundle(loc Locale) bundle {
	bundlesOnce.Do(loadBundles)
	if errBundles != nil {
		return nil
	}
	if b, ok := bundles[loc]; ok {
		return b
	}
	return bundles[DefaultLocale]
}

func lookup(b bundle, key string) (string, bool) {
	if b == nil {
		return "", false
	}
	parts := strings.Split(key, ".")
	var current any = map[string]any(b)
	for _, p := range parts {
		m, ok := current.(map[string]any)
		if !ok {
			return "", false
		}
		v, ok := m[p]
		if !ok {
			return "", false
		}
		current = v
	}
	s, ok := current.(string)
	return s, ok
}

// T returns the localized message for the given key, falling back through
// requested locale → DefaultLocale → the key itself.
func T(loc Locale, key string) string {
	if s, ok := lookup(getBundle(loc), key); ok {
		return s
	}
	if loc != DefaultLocale {
		if s, ok := lookup(getBundle(DefaultLocale), key); ok {
			return s
		}
	}
	return key
}

// Tf returns the localized message and interpolates {{.Name}}-style
// placeholders with values from args. Missing args are left as their
// raw placeholder.
func Tf(loc Locale, key string, args map[string]any) string {
	tmpl := T(loc, key)
	return interpolate(tmpl, args)
}

func interpolate(tmpl string, args map[string]any) string {
	if args == nil || !strings.Contains(tmpl, "{{") {
		return tmpl
	}
	out := tmpl
	for k, v := range args {
		needle := "{{." + k + "}}"
		out = strings.ReplaceAll(out, needle, fmt.Sprint(v))
	}
	return out
}

// ParseLocale normalizes raw locale strings ("tr-TR", "TR", "en_US") into a
// supported Locale value, returning DefaultLocale when unrecognized.
func ParseLocale(raw string) Locale {
	if loc, ok := ParseSupportedLocale(raw); ok {
		return loc
	}
	return DefaultLocale
}

// ParseSupportedLocale normalizes raw locale strings and reports whether they are supported.
func ParseSupportedLocale(raw string) (Locale, bool) {
	return matchSupported(raw)
}

// ParseAcceptLanguage returns the highest-quality supported locale from an
// HTTP Accept-Language header. Falls back to DefaultLocale when no supported
// locale is present.
func ParseAcceptLanguage(header string) Locale {
	if header == "" {
		return DefaultLocale
	}
	type entry struct {
		loc Locale
		q   float64
	}
	parts := strings.Split(header, ",")
	best := entry{loc: DefaultLocale, q: -1}
	for _, p := range parts {
		seg := strings.TrimSpace(p)
		if seg == "" {
			continue
		}
		q := 1.0
		tag := seg
		if idx := strings.Index(seg, ";"); idx >= 0 {
			tag = strings.TrimSpace(seg[:idx])
			rest := strings.TrimSpace(seg[idx+1:])
			if after, ok := strings.CutPrefix(rest, "q="); ok {
				if v, err := parseFloatStrict(after); err == nil {
					q = v
				}
			}
		}
		loc, ok := matchSupported(tag)
		if !ok {
			continue
		}
		if q > best.q {
			best = entry{loc: loc, q: q}
		}
	}
	if best.q < 0 {
		return DefaultLocale
	}
	return best.loc
}

// matchSupported returns the supported locale matching the language tag's
// primary subtag (case-insensitive), or false if none match.
func matchSupported(tag string) (Locale, bool) {
	if tag == "" {
		return "", false
	}
	lower := strings.ToLower(tag)
	if idx := strings.IndexAny(lower, "-_"); idx > 0 {
		lower = lower[:idx]
	}
	for _, loc := range SupportedLocales {
		if string(loc) == lower {
			return loc, true
		}
	}
	return "", false
}

func parseFloatStrict(s string) (float64, error) {
	var f float64
	_, err := fmt.Sscanf(strings.TrimSpace(s), "%f", &f)
	return f, err
}

func isSupported(loc Locale) bool {
	return slices.Contains(SupportedLocales, loc)
}

type ctxKey struct{}

// WithLocale stores the chosen locale on the context.
func WithLocale(ctx context.Context, loc Locale) context.Context {
	return context.WithValue(ctx, ctxKey{}, loc)
}

// FromContext returns the locale stored on the context or DefaultLocale.
func FromContext(ctx context.Context) Locale {
	if ctx == nil {
		return DefaultLocale
	}
	if v, ok := ctx.Value(ctxKey{}).(Locale); ok && isSupported(v) {
		return v
	}
	return DefaultLocale
}
