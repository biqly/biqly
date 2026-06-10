package lexicon

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/biqly/biqly/internal/metadata"
)

// nlLexiconValue is the JSONB payload of one ai_nl_lexicon row.
type nlLexiconValue struct {
	Terms              []string `json:"terms,omitempty"`
	InterpretationKeys []string `json:"interpretation_keys,omitempty"`
}

// EntryToMetadata converts a lexicon entry to its DB row form.
func EntryToMetadata(e Entry) (metadata.NLLexiconEntry, error) {
	raw, err := json.Marshal(nlLexiconValue{Terms: e.Terms, InterpretationKeys: e.InterpretationKeys})
	if err != nil {
		return metadata.NLLexiconEntry{}, fmt.Errorf("encode nl lexicon value %s/%s/%s: %w", e.Locale, e.Domain, e.Key, err)
	}
	return metadata.NLLexiconEntry{
		Locale:   e.Locale,
		Domain:   e.Domain,
		Key:      e.Key,
		Value:    raw,
		IsActive: true,
	}, nil
}

func entryFromMetadata(row metadata.NLLexiconEntry) (Entry, error) {
	var value nlLexiconValue
	if err := json.Unmarshal(row.Value, &value); err != nil {
		return Entry{}, fmt.Errorf("decode nl lexicon value: %w", err)
	}
	return Entry{
		Locale:             row.Locale,
		Domain:             row.Domain,
		Key:                row.Key,
		Terms:              value.Terms,
		InterpretationKeys: value.InterpretationKeys,
	}, nil
}

// DefaultMetadataEntries returns the embedded defaults in DB row form,
// optionally filtered to one domain (empty string = all).
func DefaultMetadataEntries(domain string) ([]metadata.NLLexiconEntry, error) {
	defaults := DefaultEntries()
	out := make([]metadata.NLLexiconEntry, 0, len(defaults))
	for _, e := range defaults {
		if domain != "" && e.Domain != domain {
			continue
		}
		row, err := EntryToMetadata(e)
		if err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	return out, nil
}

// seedRepo is the repository subset Seed needs.
type seedRepo interface {
	CountNLLexicon(ctx context.Context) (int, error)
	UpsertNLLexiconEntries(ctx context.Context, entries []metadata.NLLexiconEntry) error
}

// Seed populates ai_nl_lexicon from the embedded defaults when the table is
// empty (idempotent, SeedTimeGrains pattern).
func Seed(ctx context.Context, repo seedRepo) error {
	if repo == nil {
		return nil
	}
	count, err := repo.CountNLLexicon(ctx)
	if err != nil {
		return fmt.Errorf("seed nl lexicon count: %w", err)
	}
	if count > 0 {
		return nil
	}
	rows, err := DefaultMetadataEntries("")
	if err != nil {
		return fmt.Errorf("seed nl lexicon defaults: %w", err)
	}
	if err := repo.UpsertNLLexiconEntries(ctx, rows); err != nil {
		return fmt.Errorf("seed nl lexicon: %w", err)
	}
	slog.InfoContext(ctx, "seeded ai_nl_lexicon table with embedded defaults", "rows", len(rows))
	return nil
}
