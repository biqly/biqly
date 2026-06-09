package metadata

import (
	"context"
	"encoding/json"
	"fmt"
)

// GetAIRuntimeConfig returns the stored JSON value for an admin-managed
// runtime config key. It returns sql.ErrNoRows (wrapped) when the key is unset.
func (r *Repository) GetAIRuntimeConfig(ctx context.Context, key string) (json.RawMessage, error) {
	var raw []byte
	err := r.db.QueryRowContext(ctx, `
		SELECT value FROM ai_runtime_config WHERE key = $1
	`, key).Scan(&raw)
	if err != nil {
		return nil, fmt.Errorf("get ai runtime config %q: %w", key, err)
	}
	return raw, nil
}

// UpsertAIRuntimeConfig stores the JSON value for an admin-managed runtime config key.
func (r *Repository) UpsertAIRuntimeConfig(ctx context.Context, key string, value json.RawMessage) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO ai_runtime_config (key, value, updated_at)
		VALUES ($1, $2::jsonb, NOW())
		ON CONFLICT (key) DO UPDATE SET value = EXCLUDED.value, updated_at = NOW()
	`, key, string(value))
	if err != nil {
		return fmt.Errorf("upsert ai runtime config %q: %w", key, err)
	}
	return nil
}
