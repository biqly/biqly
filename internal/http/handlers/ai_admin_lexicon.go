package handlers

import (
	"encoding/json"
	"net/http"
	"regexp"
	"slices"

	"github.com/biqly/biqly/internal/ai/lexicon"
	"github.com/biqly/biqly/internal/ai/routing"
	"github.com/biqly/biqly/internal/metadata"
	"github.com/bytedance/sonic"
)

// invalidateLexiconCaches refreshes this replica's lexicon-derived caches after
// an admin write; other replicas converge within the store TTLs (ADR-0001 K6).
func invalidateLexiconCaches() {
	lexicon.Active().Invalidate()
	routing.InvalidateRoutingLexicon()
}

// lexiconEntryWire is the admin wire shape of one ai_nl_lexicon row: the JSONB
// value is exposed as typed fields so imports/exports stay human-editable.
type lexiconEntryWire struct {
	Locale             string   `json:"locale"`
	Domain             string   `json:"domain"`
	Key                string   `json:"key"`
	Terms              []string `json:"terms,omitempty"`
	InterpretationKeys []string `json:"interpretation_keys,omitempty"`
	IsActive           *bool    `json:"is_active,omitempty"` // default true on write
}

type lexiconListResponse struct {
	Entries []lexiconEntryWire `json:"entries"`
}

// localePattern accepts the BCP-47 subset the i18n package parses ("tr",
// "en", "de", "pt-br", …).
var localePattern = regexp.MustCompile(`^[a-z]{2,3}(-[a-z0-9]{2,8})?$`)

// AdminListLexicon returns ai_nl_lexicon rows (active and inactive),
// optionally filtered by ?locale= and ?domain=. Doubles as JSON export.
func (h *AIHandler) AdminListLexicon(w http.ResponseWriter, r *http.Request) {
	locale := r.URL.Query().Get("locale")
	domain := r.URL.Query().Get("domain")
	if domain != "" && !slices.Contains(lexicon.Domains, domain) {
		writeError(w, http.StatusBadRequest, "unknown lexicon domain")
		return
	}
	rows, err := h.deps.MetaRepo.ListNLLexicon(r.Context(), locale, domain)
	if err != nil {
		writeInternalError(r.Context(), w, http.StatusInternalServerError, "failed to list lexicon", err)
		return
	}
	out := lexiconListResponse{Entries: make([]lexiconEntryWire, 0, len(rows))}
	for _, row := range rows {
		out.Entries = append(out.Entries, lexiconEntryToWire(row))
	}
	writeJSON(w, http.StatusOK, out)
}

func lexiconEntryToWire(row metadata.NLLexiconEntry) lexiconEntryWire {
	var value struct {
		Terms              []string `json:"terms,omitempty"`
		InterpretationKeys []string `json:"interpretation_keys,omitempty"`
	}
	_ = sonic.Unmarshal(row.Value, &value)
	active := row.IsActive
	return lexiconEntryWire{
		Locale:             row.Locale,
		Domain:             row.Domain,
		Key:                row.Key,
		Terms:              value.Terms,
		InterpretationKeys: value.InterpretationKeys,
		IsActive:           &active,
	}
}

type lexiconUpsertRequest struct {
	Entries []lexiconEntryWire `json:"entries"`
}

type lexiconUpsertResponse struct {
	Updated int `json:"updated"`
}

// AdminUpsertLexicon validates and upserts lexicon entries (JSON import),
// then invalidates the in-process lexicon cache on this replica.
func (h *AIHandler) AdminUpsertLexicon(w http.ResponseWriter, r *http.Request) {
	input, ok := decodeJSON[lexiconUpsertRequest](w, r)
	if !ok {
		return
	}
	if len(input.Entries) == 0 {
		writeError(w, http.StatusBadRequest, "entries must not be empty")
		return
	}
	rows := make([]metadata.NLLexiconEntry, 0, len(input.Entries))
	for _, e := range input.Entries {
		if msg := validateLexiconEntry(e); msg != "" {
			writeError(w, http.StatusBadRequest, msg)
			return
		}
		row, err := wireToLexiconEntry(e)
		if err != nil {
			writeInternalError(r.Context(), w, http.StatusInternalServerError, "failed to encode lexicon entry", err)
			return
		}
		rows = append(rows, row)
	}
	if err := h.deps.MetaRepo.UpsertNLLexiconEntries(r.Context(), rows); err != nil {
		writeInternalError(r.Context(), w, http.StatusInternalServerError, "failed to save lexicon entries", err)
		return
	}
	invalidateLexiconCaches()
	writeJSON(w, http.StatusOK, lexiconUpsertResponse{Updated: len(rows)})
}

func validateLexiconEntry(e lexiconEntryWire) string {
	if !localePattern.MatchString(e.Locale) {
		return "locale must be a lowercase BCP-47 subset tag (e.g. \"tr\", \"en\", \"pt-br\")"
	}
	if !slices.Contains(lexicon.Domains, e.Domain) {
		return "unknown lexicon domain: " + e.Domain
	}
	if e.Key == "" {
		return "key must not be empty"
	}
	if e.Domain == lexicon.DomainTemporalPhrase {
		if len(e.InterpretationKeys) == 0 {
			return "temporal_phrase entries require interpretation_keys"
		}
		return ""
	}
	if len(e.Terms) == 0 {
		return e.Domain + " entries require terms"
	}
	return ""
}

func wireToLexiconEntry(e lexiconEntryWire) (metadata.NLLexiconEntry, error) {
	value := struct {
		Terms              []string `json:"terms,omitempty"`
		InterpretationKeys []string `json:"interpretation_keys,omitempty"`
	}{Terms: e.Terms, InterpretationKeys: e.InterpretationKeys}
	raw, err := sonic.Marshal(value)
	if err != nil {
		return metadata.NLLexiconEntry{}, err
	}
	active := true
	if e.IsActive != nil {
		active = *e.IsActive
	}
	return metadata.NLLexiconEntry{
		Locale:   e.Locale,
		Domain:   e.Domain,
		Key:      e.Key,
		Value:    json.RawMessage(raw),
		IsActive: active,
	}, nil
}

type lexiconResetRequest struct {
	Domain string `json:"domain"`
}

type lexiconResetResponse struct {
	Domain   string `json:"domain"`
	Restored int    `json:"restored"`
}

// AdminResetLexiconDomain replaces every row of one domain with the embedded
// defaults (ADR-0001 K7 escape hatch).
func (h *AIHandler) AdminResetLexiconDomain(w http.ResponseWriter, r *http.Request) {
	input, ok := decodeJSON[lexiconResetRequest](w, r)
	if !ok {
		return
	}
	if !slices.Contains(lexicon.Domains, input.Domain) {
		writeError(w, http.StatusBadRequest, "unknown lexicon domain")
		return
	}
	rows, err := lexicon.DefaultMetadataEntries(input.Domain)
	if err != nil {
		writeInternalError(r.Context(), w, http.StatusInternalServerError, "failed to build default lexicon entries", err)
		return
	}
	if err := h.deps.MetaRepo.ReplaceNLLexiconDomain(r.Context(), input.Domain, rows); err != nil {
		writeInternalError(r.Context(), w, http.StatusInternalServerError, "failed to reset lexicon domain", err)
		return
	}
	invalidateLexiconCaches()
	writeJSON(w, http.StatusOK, lexiconResetResponse{Domain: input.Domain, Restored: len(rows)})
}
