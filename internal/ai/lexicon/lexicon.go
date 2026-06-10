// Package lexicon serves locale-dimensioned natural-language vocabulary
// (synonyms, phrases, intent tokens) used by routing, ambiguity detection, and
// semantic model generation. Storage contract (ADR-0001): embedded defaults are
// the seed and fallback; the ai_nl_lexicon table is the authoritative overlay;
// matching always operates on the union of all active locales (K4).
package lexicon

import (
	"sync"
)

// Lexicon domains (ADR-0001 K2).
const (
	DomainTemporalPhrase = "temporal_phrase"
	DomainGrainSynonym   = "grain_synonym"
	DomainSoftDelete     = "soft_delete"
	DomainIntentToken    = "intent_token"
	DomainRowCount       = "row_count"
	DomainTokenSynonym   = "token_synonym" //nolint:gosec // NL lexicon domain name, not a credential
	DomainMetricSynonym  = "metric_synonym"
)

// Domains lists every valid lexicon domain (admin input validation).
var Domains = []string{
	DomainTemporalPhrase,
	DomainGrainSynonym,
	DomainSoftDelete,
	DomainIntentToken,
	DomainRowCount,
	DomainTokenSynonym,
	DomainMetricSynonym,
}

// Entry is one locale-scoped lexicon row. Term domains use Terms; the
// temporal_phrase domain uses InterpretationKeys (Key holds the phrase).
type Entry struct {
	Locale             string
	Domain             string
	Key                string
	Terms              []string
	InterpretationKeys []string
}

// TemporalPhrase is a vague relative-time phrase and the i18n interpretation
// keys (clarification.temporal.*) it expands to.
type TemporalPhrase struct {
	Phrase             string
	InterpretationKeys []string
}

// Store serves lexicon lookups from an in-process snapshot. Implementations
// must keep lookups cache-backed: per-request DB access is not allowed on the
// hot path (ADR-0001 K6).
type Store interface {
	// TemporalPhrases returns all active vague temporal phrases (union across
	// locales), ordered by phrase for deterministic detection output.
	TemporalPhrases() []TemporalPhrase
	// Terms returns the union of terms for one (domain, key) across locales.
	Terms(domain, key string) []string
	// DomainTerms returns key → union-of-terms for a whole domain.
	DomainTerms(domain string) map[string][]string
	// Invalidate drops the cached snapshot so the next lookup reloads.
	Invalidate()
}

var (
	activeMu    sync.RWMutex
	activeStore Store = NewStaticStore()
)

// Active returns the process-wide lexicon store (embedded defaults until
// SetActive wires the DB-backed store at startup).
func Active() Store {
	activeMu.RLock()
	defer activeMu.RUnlock()
	return activeStore
}

// SetActive swaps the process-wide lexicon store and returns the previous one
// (tests restore it via defer).
func SetActive(s Store) Store {
	activeMu.Lock()
	defer activeMu.Unlock()
	prev := activeStore
	if s != nil {
		activeStore = s
	}
	return prev
}
