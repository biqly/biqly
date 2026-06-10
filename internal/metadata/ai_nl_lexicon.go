package metadata

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"
)

// NLLexiconEntry is one row of ai_nl_lexicon: locale-scoped NL vocabulary
// (ADR-0001). Value carries the domain-specific JSON payload
// ({"terms": [...]} or {"interpretation_keys": [...]}).
type NLLexiconEntry struct {
	Locale    string          `json:"locale" db:"locale"`
	Domain    string          `json:"domain" db:"domain"`
	Key       string          `json:"key" db:"key"`
	Value     json.RawMessage `json:"value" db:"value"`
	IsActive  bool            `json:"is_active" db:"is_active"`
	UpdatedAt time.Time       `json:"updated_at" db:"updated_at"`
}

const nlLexiconSelectCols = `locale, domain, key, value, is_active, updated_at`

// ListNLLexicon returns all rows (active and inactive) for the admin surface,
// optionally filtered by locale and/or domain (empty string = no filter).
func (r *Repository) ListNLLexicon(ctx context.Context, locale, domain string) ([]NLLexiconEntry, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT `+nlLexiconSelectCols+`
		FROM ai_nl_lexicon
		WHERE ($1 = '' OR locale = $1) AND ($2 = '' OR domain = $2)
		ORDER BY domain, key, locale
	`, locale, domain)
	if err != nil {
		return nil, fmt.Errorf("list nl lexicon: %w", err)
	}
	defer func() { _ = rows.Close() }()
	return scanNLLexiconRows(rows)
}

// ListActiveNLLexicon returns every active row; the lexicon store merges them
// into its in-process snapshot.
func (r *Repository) ListActiveNLLexicon(ctx context.Context) ([]NLLexiconEntry, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT `+nlLexiconSelectCols+`
		FROM ai_nl_lexicon
		WHERE is_active = TRUE
		ORDER BY domain, key, locale
	`)
	if err != nil {
		return nil, fmt.Errorf("list active nl lexicon: %w", err)
	}
	defer func() { _ = rows.Close() }()
	return scanNLLexiconRows(rows)
}

func scanNLLexiconRows(rows *sql.Rows) ([]NLLexiconEntry, error) {
	var out []NLLexiconEntry
	for rows.Next() {
		var e NLLexiconEntry
		var raw []byte
		if err := rows.Scan(&e.Locale, &e.Domain, &e.Key, &raw, &e.IsActive, &e.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan nl lexicon row: %w", err)
		}
		e.Value = raw
		out = append(out, e)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate nl lexicon rows: %w", err)
	}
	return out, nil
}

// CountNLLexicon returns the total row count (startup seed guard).
func (r *Repository) CountNLLexicon(ctx context.Context) (int, error) {
	var count int
	if err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM ai_nl_lexicon`).Scan(&count); err != nil {
		return 0, fmt.Errorf("count nl lexicon: %w", err)
	}
	return count, nil
}

// UpsertNLLexiconEntries inserts or updates entries by (locale, domain, key).
func (r *Repository) UpsertNLLexiconEntries(ctx context.Context, entries []NLLexiconEntry) error {
	if len(entries) == 0 {
		return nil
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("upsert nl lexicon begin: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	for _, e := range entries {
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO ai_nl_lexicon (locale, domain, key, value, is_active, updated_at)
			VALUES ($1, $2, $3, $4::jsonb, $5, NOW())
			ON CONFLICT (locale, domain, key)
			DO UPDATE SET value = EXCLUDED.value, is_active = EXCLUDED.is_active, updated_at = NOW()
		`, e.Locale, e.Domain, e.Key, string(e.Value), e.IsActive); err != nil {
			return fmt.Errorf("upsert nl lexicon %s/%s/%s: %w", e.Locale, e.Domain, e.Key, err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("upsert nl lexicon commit: %w", err)
	}
	return nil
}

// ReplaceNLLexiconDomain atomically replaces every row of one domain (reset to
// embedded defaults, ADR-0001 K7).
func (r *Repository) ReplaceNLLexiconDomain(ctx context.Context, domain string, entries []NLLexiconEntry) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("replace nl lexicon begin: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.ExecContext(ctx, `DELETE FROM ai_nl_lexicon WHERE domain = $1`, domain); err != nil {
		return fmt.Errorf("replace nl lexicon delete %s: %w", domain, err)
	}
	for _, e := range entries {
		if e.Domain != domain {
			return fmt.Errorf("replace nl lexicon: entry domain %q does not match %q", e.Domain, domain)
		}
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO ai_nl_lexicon (locale, domain, key, value, is_active, updated_at)
			VALUES ($1, $2, $3, $4::jsonb, $5, NOW())
		`, e.Locale, e.Domain, e.Key, string(e.Value), e.IsActive); err != nil {
			return fmt.Errorf("replace nl lexicon insert %s/%s/%s: %w", e.Locale, e.Domain, e.Key, err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("replace nl lexicon commit: %w", err)
	}
	return nil
}
