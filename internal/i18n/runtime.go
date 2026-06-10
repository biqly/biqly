package i18n

import (
	"context"
	"log/slog"
	"maps"
	"slices"
	"sync"
	"time"
)

// RuntimeLocale is one i18n_locales registry row: a locale profile plus its
// enabled flag.
type RuntimeLocale struct {
	Profile LocaleProfile
	Enabled bool
}

// RuntimeProvider feeds the DB-managed locale registry and message-bundle
// overlay into the i18n layer (ADR-0001 K5/K8). Implementations live outside
// this package so it stays dependency-free; a nil provider means embedded-only
// behavior (EN/TR).
type RuntimeProvider interface {
	// Locales returns every registry row (enabled and disabled).
	Locales(ctx context.Context) ([]RuntimeLocale, error)
	// Bundles returns the DB-managed message bundles keyed by locale.
	Bundles(ctx context.Context) (map[Locale]map[string]any, error)
}

// runtimeTTL bounds cross-replica staleness after an admin write; the writing
// replica invalidates immediately, others converge within the TTL (ADR-0001 K6).
const runtimeTTL = 30 * time.Second

// runtimeLoadTimeout bounds the snapshot refresh. T/Tf run without a request
// context, so the load uses its own background deadline.
const runtimeLoadTimeout = 3 * time.Second

// runtimeState is an immutable snapshot served to lookups.
type runtimeState struct {
	supported []Locale
	profiles  map[Locale]LocaleProfile
	bundles   map[Locale]bundle // DB overlay only; embedded bundles stay separate
}

var (
	runtimeMu       sync.Mutex
	runtimeProvider RuntimeProvider
	runtimeSnap     *runtimeState
	runtimeExpires  time.Time

	embeddedStateOnce sync.Once
	embeddedState     *runtimeState
)

// SetRuntimeProvider wires the DB-backed locale registry and bundle overlay.
// Pass nil to revert to embedded-only behavior (tests).
func SetRuntimeProvider(p RuntimeProvider) {
	runtimeMu.Lock()
	runtimeProvider = p
	runtimeSnap = nil
	runtimeExpires = time.Time{}
	runtimeMu.Unlock()
}

// InvalidateRuntime drops the cached registry/bundle snapshot so the next
// lookup reloads (admin writes call this on the writing replica).
func InvalidateRuntime() {
	runtimeMu.Lock()
	runtimeSnap = nil
	runtimeExpires = time.Time{}
	runtimeMu.Unlock()
}

func embeddedRuntimeState() *runtimeState {
	embeddedStateOnce.Do(func() {
		embeddedState = &runtimeState{
			supported: SupportedLocales,
			profiles:  localeProfiles,
		}
	})
	return embeddedState
}

func currentRuntime() *runtimeState {
	runtimeMu.Lock()
	defer runtimeMu.Unlock()
	if runtimeProvider == nil {
		return embeddedRuntimeState()
	}
	if runtimeSnap != nil && time.Now().Before(runtimeExpires) {
		return runtimeSnap
	}
	runtimeSnap = loadRuntimeState(runtimeProvider)
	runtimeExpires = time.Now().Add(runtimeTTL)
	return runtimeSnap
}

// loadRuntimeState refreshes the snapshot with its own background deadline:
// T/Tf/IsSupported run on hot paths without a request context by design
// (ADR-0001 K6), so no caller context exists to propagate. The directive below
// suppresses contextcheck's whole-chain reports at every caller; nolintlint is
// included because the chain diagnostics attach to those callers, not here.
//
//nolint:contextcheck,nolintlint // cache refresh is deliberately request-scope-free
func loadRuntimeState(p RuntimeProvider) *runtimeState {
	ctx, cancel := context.WithTimeout(context.Background(), runtimeLoadTimeout)
	defer cancel()

	st := &runtimeState{
		supported: append([]Locale(nil), SupportedLocales...),
		profiles:  maps.Clone(localeProfiles),
		bundles:   map[Locale]bundle{},
	}
	rows, err := p.Locales(ctx)
	switch {
	case err != nil:
		slog.WarnContext(ctx, "i18n locale registry load failed, serving embedded locales", "error", err)
	case len(rows) > 0:
		st.supported, st.profiles = mergeRuntimeLocales(rows)
	}
	dbBundles, err := p.Bundles(ctx)
	if err != nil {
		slog.WarnContext(ctx, "i18n bundle overlay load failed, serving embedded bundles", "error", err)
		return st
	}
	for loc, b := range dbBundles {
		if len(b) > 0 {
			st.bundles[loc] = bundle(b)
		}
	}
	return st
}

// mergeRuntimeLocales overlays registry rows onto the embedded locales:
// embedded order first, registry-added locales appended sorted. DefaultLocale
// is the terminal fallback and cannot be disabled (ADR-0001 K8).
func mergeRuntimeLocales(rows []RuntimeLocale) ([]Locale, map[Locale]LocaleProfile) {
	profiles := maps.Clone(localeProfiles)
	enabled := make(map[Locale]bool, len(rows))
	inRegistry := make(map[Locale]bool, len(rows))
	for _, row := range rows {
		loc := row.Profile.Locale
		if loc == "" {
			continue
		}
		inRegistry[loc] = true
		enabled[loc] = row.Enabled
		if row.Enabled {
			profiles[loc] = row.Profile
		}
	}

	supported := make([]Locale, 0, len(profiles))
	for _, loc := range SupportedLocales {
		if loc == DefaultLocale || !inRegistry[loc] || enabled[loc] {
			supported = append(supported, loc)
		}
	}
	var extra []Locale
	for loc := range profiles {
		if !slices.Contains(SupportedLocales, loc) {
			extra = append(extra, loc)
		}
	}
	slices.Sort(extra)
	return append(supported, extra...), profiles
}

// runtimeBundle returns the DB-managed bundle for a locale, nil when absent.
func runtimeBundle(loc Locale) bundle {
	return currentRuntime().bundles[loc]
}
