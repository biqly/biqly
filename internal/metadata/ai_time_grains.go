package metadata

import (
	"context"
	"fmt"
	"time"

	"github.com/biqly/biqly/internal/platform/db/pgarray"
)

// TimeGrain represents a customizable time grain configuration.
type TimeGrain struct {
	Grain        string    `json:"grain"`
	Suffix       string    `json:"suffix"`
	RequiresTime bool      `json:"requires_time"`
	Synonyms     []string  `json:"synonyms"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// CountTimeGrains returns the total number of time grains stored.
func (r *Repository) CountTimeGrains(ctx context.Context) (int, error) {
	var count int
	err := r.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM ai_time_grains").Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("count time grains: %w", err)
	}
	return count, nil
}

// ListTimeGrains returns all customizable time grains.
func (r *Repository) ListTimeGrains(ctx context.Context) ([]TimeGrain, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT grain, suffix, requires_time, synonyms, created_at, updated_at
		FROM ai_time_grains
		ORDER BY grain
	`)
	if err != nil {
		return nil, fmt.Errorf("list time grains: %w", err)
	}
	defer func() { _ = rows.Close() }()

	grains := make([]TimeGrain, 0, 8)
	for rows.Next() {
		var tg TimeGrain
		err := rows.Scan(
			&tg.Grain,
			&tg.Suffix,
			&tg.RequiresTime,
			pgarray.Scan(&tg.Synonyms),
			&tg.CreatedAt,
			&tg.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("scan time grain: %w", err)
		}
		grains = append(grains, tg)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows error in time grains: %w", err)
	}

	return grains, nil
}

// UpdateTimeGrain updates an existing time grain.
func (r *Repository) UpdateTimeGrain(ctx context.Context, tg TimeGrain) error {
	query := `
		UPDATE ai_time_grains
		SET suffix = $2, requires_time = $3, synonyms = $4, updated_at = now()
		WHERE grain = $1
	`
	res, err := r.db.ExecContext(ctx, query, tg.Grain, tg.Suffix, tg.RequiresTime, pgarray.Strings(tg.Synonyms))
	if err != nil {
		return fmt.Errorf("update time grain: %w", err)
	}

	rows, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("rows affected update time grain: %w", err)
	}
	if rows == 0 {
		return fmt.Errorf("time grain not found: %s", tg.Grain)
	}

	return nil
}

// UpsertTimeGrain inserts or updates a time grain.
func (r *Repository) UpsertTimeGrain(ctx context.Context, tg TimeGrain) error {
	query := `
		INSERT INTO ai_time_grains (grain, suffix, requires_time, synonyms, created_at, updated_at)
		VALUES ($1, $2, $3, $4, now(), now())
		ON CONFLICT (grain) DO UPDATE SET
			suffix = EXCLUDED.suffix,
			requires_time = EXCLUDED.requires_time,
			synonyms = EXCLUDED.synonyms,
			updated_at = now()
	`
	_, err := r.db.ExecContext(ctx, query, tg.Grain, tg.Suffix, tg.RequiresTime, pgarray.Strings(tg.Synonyms))
	if err != nil {
		return fmt.Errorf("upsert time grain: %w", err)
	}
	return nil
}
