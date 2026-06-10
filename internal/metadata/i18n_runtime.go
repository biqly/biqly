package metadata

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/biqly/biqly/internal/i18n"
	"github.com/bytedance/sonic"
)

// I18nLocaleRow is one i18n_locales registry row (ADR-0001 K8).
type I18nLocaleRow struct {
	Locale                   string    `json:"locale" db:"locale"`
	Label                    string    `json:"label" db:"label"`
	ShortLabel               string    `json:"short_label" db:"short_label"`
	QuestionLetters          string    `json:"question_letters" db:"question_letters"`
	QuestionSignals          []string  `json:"question_signals" db:"question_signals"`
	UsesMetadataTranslations bool      `json:"uses_metadata_translations" db:"uses_metadata_translations"`
	Enabled                  bool      `json:"enabled" db:"enabled"`
	UpdatedAt                time.Time `json:"updated_at" db:"updated_at"`
}

// I18nBundleRow is one DB-managed message bundle (i18n_bundles).
type I18nBundleRow struct {
	Locale    string          `json:"locale" db:"locale"`
	Bundle    json.RawMessage `json:"bundle" db:"bundle"`
	Version   int             `json:"version" db:"version"`
	UpdatedAt time.Time       `json:"updated_at" db:"updated_at"`
}

// ListI18nLocales returns every locale registry row ordered by locale.
func (r *Repository) ListI18nLocales(ctx context.Context) ([]I18nLocaleRow, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT locale, label, short_label, question_letters, question_signals,
		       uses_metadata_translations, enabled, updated_at
		FROM i18n_locales
		ORDER BY locale
	`)
	if err != nil {
		return nil, fmt.Errorf("list i18n locales: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var out []I18nLocaleRow
	for rows.Next() {
		var row I18nLocaleRow
		var signals []byte
		if err := rows.Scan(&row.Locale, &row.Label, &row.ShortLabel, &row.QuestionLetters,
			&signals, &row.UsesMetadataTranslations, &row.Enabled, &row.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan i18n locale row: %w", err)
		}
		if len(signals) > 0 {
			if err := sonic.Unmarshal(signals, &row.QuestionSignals); err != nil {
				return nil, fmt.Errorf("decode i18n locale signals %q: %w", row.Locale, err)
			}
		}
		out = append(out, row)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate i18n locale rows: %w", err)
	}
	return out, nil
}

// CountI18nLocales returns the registry row count (startup seed guard).
func (r *Repository) CountI18nLocales(ctx context.Context) (int, error) {
	var count int
	if err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM i18n_locales`).Scan(&count); err != nil {
		return 0, fmt.Errorf("count i18n locales: %w", err)
	}
	return count, nil
}

// UpsertI18nLocales inserts or updates registry rows by locale.
func (r *Repository) UpsertI18nLocales(ctx context.Context, rows []I18nLocaleRow) error {
	if len(rows) == 0 {
		return nil
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("upsert i18n locales begin: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	for _, row := range rows {
		signals, err := sonic.Marshal(row.QuestionSignals)
		if err != nil {
			return fmt.Errorf("encode i18n locale signals %q: %w", row.Locale, err)
		}
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO i18n_locales (locale, label, short_label, question_letters,
				question_signals, uses_metadata_translations, enabled, updated_at)
			VALUES ($1, $2, $3, $4, $5::jsonb, $6, $7, NOW())
			ON CONFLICT (locale) DO UPDATE SET
				label = EXCLUDED.label,
				short_label = EXCLUDED.short_label,
				question_letters = EXCLUDED.question_letters,
				question_signals = EXCLUDED.question_signals,
				uses_metadata_translations = EXCLUDED.uses_metadata_translations,
				enabled = EXCLUDED.enabled,
				updated_at = NOW()
		`, row.Locale, row.Label, row.ShortLabel, row.QuestionLetters,
			string(signals), row.UsesMetadataTranslations, row.Enabled); err != nil {
			return fmt.Errorf("upsert i18n locale %q: %w", row.Locale, err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("upsert i18n locales commit: %w", err)
	}
	return nil
}

// ListI18nBundles returns every DB-managed bundle row.
func (r *Repository) ListI18nBundles(ctx context.Context) ([]I18nBundleRow, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT locale, bundle, version, updated_at FROM i18n_bundles ORDER BY locale
	`)
	if err != nil {
		return nil, fmt.Errorf("list i18n bundles: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var out []I18nBundleRow
	for rows.Next() {
		var row I18nBundleRow
		var raw []byte
		if err := rows.Scan(&row.Locale, &raw, &row.Version, &row.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan i18n bundle row: %w", err)
		}
		row.Bundle = raw
		out = append(out, row)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate i18n bundle rows: %w", err)
	}
	return out, nil
}

// GetI18nBundle returns one bundle row; sql.ErrNoRows (wrapped) when absent.
func (r *Repository) GetI18nBundle(ctx context.Context, locale string) (I18nBundleRow, error) {
	var row I18nBundleRow
	var raw []byte
	err := r.db.QueryRowContext(ctx, `
		SELECT locale, bundle, version, updated_at FROM i18n_bundles WHERE locale = $1
	`, locale).Scan(&row.Locale, &raw, &row.Version, &row.UpdatedAt)
	if err != nil {
		return I18nBundleRow{}, fmt.Errorf("get i18n bundle %q: %w", locale, err)
	}
	row.Bundle = raw
	return row, nil
}

// UpsertI18nBundle stores a bundle, bumping its version on update.
func (r *Repository) UpsertI18nBundle(ctx context.Context, locale string, bundleJSON json.RawMessage) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO i18n_bundles (locale, bundle, version, updated_at)
		VALUES ($1, $2::jsonb, 1, NOW())
		ON CONFLICT (locale) DO UPDATE SET
			bundle = EXCLUDED.bundle,
			version = i18n_bundles.version + 1,
			updated_at = NOW()
	`, locale, string(bundleJSON))
	if err != nil {
		return fmt.Errorf("upsert i18n bundle %q: %w", locale, err)
	}
	return nil
}

// i18nRuntimeProvider adapts the repository to i18n.RuntimeProvider.
type i18nRuntimeProvider struct {
	repo *Repository
}

// NewI18nRuntimeProvider returns the DB-backed locale registry + bundle
// overlay provider for i18n.SetRuntimeProvider.
func NewI18nRuntimeProvider(repo *Repository) i18n.RuntimeProvider {
	return &i18nRuntimeProvider{repo: repo}
}

func (p *i18nRuntimeProvider) Locales(ctx context.Context) ([]i18n.RuntimeLocale, error) {
	rows, err := p.repo.ListI18nLocales(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]i18n.RuntimeLocale, 0, len(rows))
	for _, row := range rows {
		out = append(out, i18n.RuntimeLocale{
			Profile: i18n.LocaleProfile{
				Locale:                   i18n.Locale(row.Locale),
				Label:                    row.Label,
				ShortLabel:               row.ShortLabel,
				QuestionLetters:          row.QuestionLetters,
				QuestionSignals:          row.QuestionSignals,
				UsesMetadataTranslations: row.UsesMetadataTranslations,
			},
			Enabled: row.Enabled,
		})
	}
	return out, nil
}

func (p *i18nRuntimeProvider) Bundles(ctx context.Context) (map[i18n.Locale]map[string]any, error) {
	rows, err := p.repo.ListI18nBundles(ctx)
	if err != nil {
		return nil, err
	}
	out := make(map[i18n.Locale]map[string]any, len(rows))
	for _, row := range rows {
		var b map[string]any
		if err := sonic.Unmarshal(row.Bundle, &b); err != nil {
			slog.WarnContext(ctx, "i18n bundle row skipped", "locale", row.Locale, "error", err)
			continue
		}
		out[i18n.Locale(row.Locale)] = b
	}
	return out, nil
}

// SeedI18nLocales populates the locale registry from the embedded EN/TR
// profiles when the table is empty (idempotent, ADR-0001 K7).
func SeedI18nLocales(ctx context.Context, repo *Repository) error {
	if repo == nil {
		return nil
	}
	count, err := repo.CountI18nLocales(ctx)
	if err != nil {
		return fmt.Errorf("seed i18n locales count: %w", err)
	}
	if count > 0 {
		return nil
	}
	profiles := i18n.EmbeddedLocaleProfiles()
	rows := make([]I18nLocaleRow, 0, len(profiles))
	for _, profile := range profiles {
		rows = append(rows, I18nLocaleRow{
			Locale:                   string(profile.Locale),
			Label:                    profile.Label,
			ShortLabel:               profile.ShortLabel,
			QuestionLetters:          profile.QuestionLetters,
			QuestionSignals:          profile.QuestionSignals,
			UsesMetadataTranslations: profile.UsesMetadataTranslations,
			Enabled:                  true,
		})
	}
	if err := repo.UpsertI18nLocales(ctx, rows); err != nil {
		return fmt.Errorf("seed i18n locales: %w", err)
	}
	slog.InfoContext(ctx, "seeded i18n_locales table with embedded profiles", "rows", len(rows))
	return nil
}
