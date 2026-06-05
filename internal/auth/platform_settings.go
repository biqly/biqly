package auth

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

var ErrPlatformSettingsNotFound = errors.New("platform settings not found")

type PlatformSettings struct {
	SelfSignupEnabled bool       `json:"self_signup_enabled"`
	UpdatedAt         time.Time  `json:"updated_at"`
	UpdatedBy         *string    `json:"updated_by,omitempty"`
}

type PlatformSettingsRepository struct {
	db *sql.DB
}

func NewPlatformSettingsRepository(db *sql.DB) *PlatformSettingsRepository {
	return &PlatformSettingsRepository{db: db}
}

func (r *PlatformSettingsRepository) Get(ctx context.Context) (PlatformSettings, error) {
	var s PlatformSettings
	var updatedBy sql.NullString
	err := r.db.QueryRowContext(ctx, `
		SELECT self_signup_enabled, updated_at, updated_by
		FROM platform_settings
		WHERE id = 1
	`).Scan(&s.SelfSignupEnabled, &s.UpdatedAt, &updatedBy)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return PlatformSettings{}, ErrPlatformSettingsNotFound
		}
		return PlatformSettings{}, fmt.Errorf("get platform settings: %w", err)
	}
	if updatedBy.Valid {
		s.UpdatedBy = new(updatedBy.String)
	}
	return s, nil
}

func (r *PlatformSettingsRepository) SetSelfSignupEnabled(ctx context.Context, enabled bool, updatedBy string) (PlatformSettings, error) {
	var s PlatformSettings
	var updatedByOut sql.NullString
	err := r.db.QueryRowContext(ctx, `
		UPDATE platform_settings
		SET self_signup_enabled = $1,
		    updated_at = NOW(),
		    updated_by = NULLIF($2::text, '')::uuid
		WHERE id = 1
		RETURNING self_signup_enabled, updated_at, updated_by
	`, enabled, updatedBy).Scan(&s.SelfSignupEnabled, &s.UpdatedAt, &updatedByOut)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return PlatformSettings{}, ErrPlatformSettingsNotFound
		}
		return PlatformSettings{}, fmt.Errorf("update platform settings: %w", err)
	}
	if updatedByOut.Valid {
		s.UpdatedBy = new(updatedByOut.String)
	}
	return s, nil
}
