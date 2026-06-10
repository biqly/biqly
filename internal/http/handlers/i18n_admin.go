package handlers

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"sort"
	"strings"

	"github.com/biqly/biqly/internal/i18n"
	"github.com/biqly/biqly/internal/metadata"
	"github.com/bytedance/sonic"
	"github.com/go-chi/chi/v5"
)

// i18nLocalesResponse lists the locale registry rows (DB-authoritative after seed).
type i18nLocalesResponse struct {
	Locales []metadata.I18nLocaleRow `json:"locales"`
}

// AdminListI18nLocales returns the i18n_locales registry (enabled and disabled).
func (h *AIHandler) AdminListI18nLocales(w http.ResponseWriter, r *http.Request) {
	rows, err := h.deps.MetaRepo.ListI18nLocales(r.Context())
	if err != nil {
		writeInternalError(r.Context(), w, http.StatusInternalServerError, "failed to list i18n locales", err)
		return
	}
	writeJSON(w, http.StatusOK, i18nLocalesResponse{Locales: rows})
}

type i18nLocalesUpsertRequest struct {
	Locales []metadata.I18nLocaleRow `json:"locales"`
}

// AdminUpsertI18nLocales validates and upserts locale registry rows, then
// invalidates the i18n runtime cache on this replica.
func (h *AIHandler) AdminUpsertI18nLocales(w http.ResponseWriter, r *http.Request) {
	input, ok := decodeJSON[i18nLocalesUpsertRequest](w, r)
	if !ok {
		return
	}
	if len(input.Locales) == 0 {
		writeError(w, http.StatusBadRequest, "locales must not be empty")
		return
	}
	for _, row := range input.Locales {
		if msg := validateI18nLocaleRow(row); msg != "" {
			writeError(w, http.StatusBadRequest, msg)
			return
		}
	}
	if err := h.deps.MetaRepo.UpsertI18nLocales(r.Context(), input.Locales); err != nil {
		writeInternalError(r.Context(), w, http.StatusInternalServerError, "failed to save i18n locales", err)
		return
	}
	i18n.InvalidateRuntime()
	writeJSON(w, http.StatusOK, i18nLocalesResponse{Locales: input.Locales})
}

func validateI18nLocaleRow(row metadata.I18nLocaleRow) string {
	if !localePattern.MatchString(row.Locale) {
		return "locale must be a lowercase BCP-47 subset tag (e.g. \"tr\", \"en\", \"pt-br\")"
	}
	if strings.TrimSpace(row.Label) == "" || strings.TrimSpace(row.ShortLabel) == "" {
		return "label and short_label are required"
	}
	// Embedded EN is the terminal i18n fallback (ADR-0001 K8).
	if row.Locale == string(i18n.DefaultLocale) && !row.Enabled {
		return "the default locale cannot be disabled"
	}
	return ""
}

// i18nBundleResponse carries one effective message bundle plus its source.
type i18nBundleResponse struct {
	Locale  string          `json:"locale"`
	Source  string          `json:"source"` // "database" | "embedded" | "none"
	Version int             `json:"version,omitempty"`
	Bundle  json.RawMessage `json:"bundle"`
}

// AdminGetI18nBundle exports the effective bundle for one locale: the DB row
// when present, otherwise the embedded catalog.
func (h *AIHandler) AdminGetI18nBundle(w http.ResponseWriter, r *http.Request) {
	locale := chi.URLParam(r, "locale")
	if !localePattern.MatchString(locale) {
		writeError(w, http.StatusBadRequest, "invalid locale")
		return
	}
	row, err := h.deps.MetaRepo.GetI18nBundle(r.Context(), locale)
	switch {
	case err == nil:
		writeJSON(w, http.StatusOK, i18nBundleResponse{Locale: locale, Source: "database", Version: row.Version, Bundle: row.Bundle})
		return
	case !errors.Is(err, sql.ErrNoRows):
		writeInternalError(r.Context(), w, http.StatusInternalServerError, "failed to load i18n bundle", err)
		return
	}
	if embedded, ok := i18n.EmbeddedBundle(i18n.Locale(locale)); ok {
		raw, marshalErr := sonic.Marshal(embedded)
		if marshalErr != nil {
			writeInternalError(r.Context(), w, http.StatusInternalServerError, "failed to encode embedded bundle", marshalErr)
			return
		}
		writeJSON(w, http.StatusOK, i18nBundleResponse{Locale: locale, Source: "embedded", Bundle: raw})
		return
	}
	writeJSON(w, http.StatusOK, i18nBundleResponse{Locale: locale, Source: "none", Bundle: json.RawMessage("{}")})
}

// AdminUpsertI18nBundle imports a message bundle for one locale (nested JSON
// object with string leaves) and invalidates the runtime cache.
func (h *AIHandler) AdminUpsertI18nBundle(w http.ResponseWriter, r *http.Request) {
	locale := chi.URLParam(r, "locale")
	if !localePattern.MatchString(locale) {
		writeError(w, http.StatusBadRequest, "invalid locale")
		return
	}
	decoded, ok := decodeJSON[map[string]any](w, r)
	if !ok {
		return
	}
	input := *decoded
	if len(input) == 0 {
		writeError(w, http.StatusBadRequest, "bundle must be a non-empty JSON object")
		return
	}
	if path, valid := validateBundleLeaves(input, ""); !valid {
		writeError(w, http.StatusBadRequest, "bundle values must be strings or nested objects (invalid at "+path+")")
		return
	}
	raw, err := sonic.Marshal(input)
	if err != nil {
		writeInternalError(r.Context(), w, http.StatusInternalServerError, "failed to encode bundle", err)
		return
	}
	if err := h.deps.MetaRepo.UpsertI18nBundle(r.Context(), locale, raw); err != nil {
		writeInternalError(r.Context(), w, http.StatusInternalServerError, "failed to save bundle", err)
		return
	}
	i18n.InvalidateRuntime()
	h.AdminGetI18nBundle(w, r)
}

// validateBundleLeaves walks a bundle and reports the first non-string leaf.
func validateBundleLeaves(node map[string]any, prefix string) (string, bool) {
	for key, value := range node {
		path := key
		if prefix != "" {
			path = prefix + "." + key
		}
		switch v := value.(type) {
		case string:
		case map[string]any:
			if p, ok := validateBundleLeaves(v, path); !ok {
				return p, false
			}
		default:
			_ = v
			return path, false
		}
	}
	return "", true
}

// i18nCoverageResponse reports translation coverage of a locale against the
// effective DefaultLocale catalog.
type i18nCoverageResponse struct {
	Locale      string   `json:"locale"`
	TotalKeys   int      `json:"total_keys"`
	Translated  int      `json:"translated"`
	CoveragePct float64  `json:"coverage_pct"`
	MissingKeys []string `json:"missing_keys"`
}

// AdminI18nCoverage lists the message keys a locale's effective bundle is
// missing relative to the effective DefaultLocale catalog.
func (h *AIHandler) AdminI18nCoverage(w http.ResponseWriter, r *http.Request) {
	locale := chi.URLParam(r, "locale")
	if !localePattern.MatchString(locale) {
		writeError(w, http.StatusBadRequest, "invalid locale")
		return
	}
	reference, err := h.effectiveI18nBundle(r, string(i18n.DefaultLocale))
	if err != nil {
		writeInternalError(r.Context(), w, http.StatusInternalServerError, "failed to load reference bundle", err)
		return
	}
	target, err := h.effectiveI18nBundle(r, locale)
	if err != nil {
		writeInternalError(r.Context(), w, http.StatusInternalServerError, "failed to load target bundle", err)
		return
	}

	referenceKeys := flattenBundleKeys(reference, "")
	targetKeys := make(map[string]struct{}, len(referenceKeys))
	for _, key := range flattenBundleKeys(target, "") {
		targetKeys[key] = struct{}{}
	}
	missing := make([]string, 0)
	for _, key := range referenceKeys {
		if _, ok := targetKeys[key]; !ok {
			missing = append(missing, key)
		}
	}
	sort.Strings(missing)

	out := i18nCoverageResponse{
		Locale:      locale,
		TotalKeys:   len(referenceKeys),
		Translated:  len(referenceKeys) - len(missing),
		MissingKeys: missing,
	}
	if out.TotalKeys > 0 {
		out.CoveragePct = float64(out.Translated) / float64(out.TotalKeys) * 100
	}
	writeJSON(w, http.StatusOK, out)
}

// effectiveI18nBundle resolves DB bundle → embedded bundle for one locale.
func (h *AIHandler) effectiveI18nBundle(r *http.Request, locale string) (map[string]any, error) {
	row, err := h.deps.MetaRepo.GetI18nBundle(r.Context(), locale)
	switch {
	case err == nil:
		var b map[string]any
		if jsonErr := sonic.Unmarshal(row.Bundle, &b); jsonErr != nil {
			return nil, jsonErr
		}
		return b, nil
	case !errors.Is(err, sql.ErrNoRows):
		return nil, err
	}
	if embedded, ok := i18n.EmbeddedBundle(i18n.Locale(locale)); ok {
		return embedded, nil
	}
	return map[string]any{}, nil
}

func flattenBundleKeys(node map[string]any, prefix string) []string {
	keys := make([]string, 0, len(node))
	for key, value := range node {
		path := key
		if prefix != "" {
			path = prefix + "." + key
		}
		if child, ok := value.(map[string]any); ok {
			keys = append(keys, flattenBundleKeys(child, path)...)
			continue
		}
		keys = append(keys, path)
	}
	return keys
}
