package lexicon

import (
	"context"
	"log/slog"
	"sort"
	"sync"
	"time"

	"github.com/biqly/biqly/internal/metadata"
)

// snapshot is an immutable, fully-merged view served to lookups.
type snapshot struct {
	terms    map[string]map[string][]string // domain → key → union of terms
	temporal []TemporalPhrase               // sorted by phrase
}

func buildSnapshot(entries []Entry) *snapshot {
	s := &snapshot{terms: make(map[string]map[string][]string)}
	temporalByPhrase := make(map[string][]string)
	for _, e := range entries {
		if e.Domain == DomainTemporalPhrase {
			temporalByPhrase[e.Key] = unionTerms(temporalByPhrase[e.Key], e.InterpretationKeys)
			continue
		}
		byKey := s.terms[e.Domain]
		if byKey == nil {
			byKey = make(map[string][]string)
			s.terms[e.Domain] = byKey
		}
		byKey[e.Key] = unionTerms(byKey[e.Key], e.Terms)
	}
	phrases := make([]string, 0, len(temporalByPhrase))
	for phrase := range temporalByPhrase {
		phrases = append(phrases, phrase)
	}
	sort.Strings(phrases)
	s.temporal = make([]TemporalPhrase, 0, len(phrases))
	for _, phrase := range phrases {
		s.temporal = append(s.temporal, TemporalPhrase{Phrase: phrase, InterpretationKeys: temporalByPhrase[phrase]})
	}
	return s
}

// unionTerms appends additions preserving first-seen order, skipping duplicates.
func unionTerms(base, additions []string) []string {
	seen := make(map[string]struct{}, len(base)+len(additions))
	for _, t := range base {
		seen[t] = struct{}{}
	}
	for _, t := range additions {
		if _, ok := seen[t]; ok {
			continue
		}
		seen[t] = struct{}{}
		base = append(base, t)
	}
	return base
}

func (s *snapshot) temporalPhrases() []TemporalPhrase { return s.temporal }

func (s *snapshot) termsFor(domain, key string) []string {
	byKey, ok := s.terms[domain]
	if !ok {
		return nil
	}
	terms := byKey[key]
	if len(terms) == 0 {
		return nil
	}
	// Callers may append to the result; never hand out the snapshot's slice.
	out := make([]string, len(terms))
	copy(out, terms)
	return out
}

func (s *snapshot) domainTerms(domain string) map[string][]string {
	return s.terms[domain]
}

// staticStore serves the embedded defaults; used until the DB store is wired
// and as the terminal fallback.
type staticStore struct {
	snap *snapshot
}

// NewStaticStore builds a store over the embedded default entries.
func NewStaticStore() Store {
	return &staticStore{snap: buildSnapshot(DefaultEntries())}
}

func (s *staticStore) TemporalPhrases() []TemporalPhrase { return s.snap.temporalPhrases() }
func (s *staticStore) Terms(domain, key string) []string { return s.snap.termsFor(domain, key) }
func (s *staticStore) DomainTerms(domain string) map[string][]string {
	return s.snap.domainTerms(domain)
}
func (*staticStore) Invalidate() {}

// lexiconRepo is the metadata-repository subset the DB store needs.
type lexiconRepo interface {
	ListActiveNLLexicon(ctx context.Context) ([]metadata.NLLexiconEntry, error)
}

// dbStoreTTL bounds cross-replica staleness after an admin PUT; the writing
// replica invalidates immediately, others converge within the TTL (ADR-0001 K6).
const dbStoreTTL = 30 * time.Second

// dbLoadTimeout bounds the snapshot refresh DB call. Lookups run on hot paths
// without a request context, so the load uses its own background deadline.
const dbLoadTimeout = 3 * time.Second

type dbStore struct {
	repo     lexiconRepo
	defaults *snapshot

	mu      sync.Mutex
	snap    *snapshot
	expires time.Time
}

// NewDBStore returns a Store reading ai_nl_lexicon with the embedded defaults
// as per-domain fallback: a domain with no active DB rows (or any load error)
// serves the embedded entries for that domain.
func NewDBStore(repo lexiconRepo) Store {
	return &dbStore{repo: repo, defaults: buildSnapshot(DefaultEntries())}
}

func (s *dbStore) current() *snapshot {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.snap != nil && time.Now().Before(s.expires) {
		return s.snap
	}
	s.snap = s.load()
	s.expires = time.Now().Add(dbStoreTTL)
	return s.snap
}

func (s *dbStore) load() *snapshot {
	ctx, cancel := context.WithTimeout(context.Background(), dbLoadTimeout)
	defer cancel()
	rows, err := s.repo.ListActiveNLLexicon(ctx)
	if err != nil {
		slog.WarnContext(ctx, "nl lexicon load failed, serving embedded defaults", "error", err)
		return s.defaults
	}
	entries := make([]Entry, 0, len(rows))
	for _, row := range rows {
		entry, decodeErr := entryFromMetadata(row)
		if decodeErr != nil {
			slog.WarnContext(ctx, "nl lexicon row skipped", "domain", row.Domain, "key", row.Key, "locale", row.Locale, "error", decodeErr)
			continue
		}
		entries = append(entries, entry)
	}
	snap := buildSnapshot(entries)
	s.fillEmptyDomainsFromDefaults(snap)
	return snap
}

// fillEmptyDomainsFromDefaults applies the ADR-0001 K5 per-domain fallback.
func (s *dbStore) fillEmptyDomainsFromDefaults(snap *snapshot) {
	for domain, byKey := range s.defaults.terms {
		if len(snap.terms[domain]) == 0 {
			snap.terms[domain] = byKey
		}
	}
	if len(snap.temporal) == 0 {
		snap.temporal = s.defaults.temporal
	}
}

func (s *dbStore) TemporalPhrases() []TemporalPhrase { return s.current().temporalPhrases() }

func (s *dbStore) Terms(domain, key string) []string { return s.current().termsFor(domain, key) }

func (s *dbStore) DomainTerms(domain string) map[string][]string {
	return s.current().domainTerms(domain)
}

func (s *dbStore) Invalidate() {
	s.mu.Lock()
	s.expires = time.Time{}
	s.mu.Unlock()
}
