package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	platformdb "github.com/biqly/biqly/internal/platform/db"
)

var (
	ErrAccountFrozen        = errors.New("account is frozen")
	ErrAccountDeleted       = errors.New("account is deleted")
	ErrUnlockTokenInvalid   = errors.New("unlock token is invalid or expired")
	ErrPasswordExpired      = errors.New("password expired; reset required")
	ErrAccountAlreadyFrozen = errors.New("account is already frozen")
	ErrAccountNotFrozen     = errors.New("account is not frozen")
)

type AccountState struct {
	FrozenAt          *time.Time
	DeletedAt         *time.Time
	PurgeAfter        *time.Time
	PasswordChangedAt *time.Time
}

func (s AccountState) IsFrozen() bool  { return s.FrozenAt != nil }
func (s AccountState) IsDeleted() bool { return s.DeletedAt != nil }

func (r *UserRepository) GetAccountState(ctx context.Context, userID string) (AccountState, error) {
	var st AccountState
	var frozen, deleted, purge, pwChanged sql.NullTime
	err := r.db.QueryRowContext(ctx,
		`SELECT frozen_at, deleted_at, purge_after, password_changed_at FROM users WHERE id = $1`,
		userID,
	).Scan(&frozen, &deleted, &purge, &pwChanged)
	if errors.Is(err, sql.ErrNoRows) {
		return st, ErrUserNotFound
	}
	if err != nil {
		return st, fmt.Errorf("query account state: %w", err)
	}
	if frozen.Valid {
		t := frozen.Time
		st.FrozenAt = &t
	}
	if deleted.Valid {
		t := deleted.Time
		st.DeletedAt = &t
	}
	if purge.Valid {
		t := purge.Time
		st.PurgeAfter = &t
	}
	if pwChanged.Valid {
		t := pwChanged.Time
		st.PasswordChangedAt = &t
	}
	return st, nil
}

func (r *UserRepository) FreezeAccount(ctx context.Context, userID string) error {
	res, err := r.db.ExecContext(ctx,
		`UPDATE users SET frozen_at = NOW(), updated_at = NOW() WHERE id = $1 AND frozen_at IS NULL AND deleted_at IS NULL`,
		userID,
	)
	if err != nil {
		return fmt.Errorf("freeze account: %w", err)
	}
	rows, _ := res.RowsAffected()
	if rows == 0 {
		return ErrAccountAlreadyFrozen
	}
	return nil
}

func (r *UserRepository) UnfreezeAccount(ctx context.Context, userID string) error {
	res, err := r.db.ExecContext(ctx,
		`UPDATE users SET frozen_at = NULL, updated_at = NOW() WHERE id = $1 AND frozen_at IS NOT NULL`,
		userID,
	)
	if err != nil {
		return fmt.Errorf("unfreeze account: %w", err)
	}
	rows, _ := res.RowsAffected()
	if rows == 0 {
		return ErrAccountNotFrozen
	}
	return nil
}

func (r *UserRepository) SoftDeleteAccount(ctx context.Context, userID string, purgeAfterDays int) (time.Time, error) {
	if purgeAfterDays <= 0 {
		purgeAfterDays = 30
	}
	purgeAt := time.Now().Add(time.Duration(purgeAfterDays) * 24 * time.Hour)
	_, err := r.db.ExecContext(ctx,
		`UPDATE users SET deleted_at = NOW(), purge_after = $2, is_active = FALSE, updated_at = NOW() WHERE id = $1 AND deleted_at IS NULL`,
		userID, purgeAt,
	)
	if err != nil {
		return time.Time{}, fmt.Errorf("soft delete account: %w", err)
	}
	return purgeAt, nil
}

func (r *UserRepository) RestoreAccount(ctx context.Context, userID string) error {
	res, err := r.db.ExecContext(ctx,
		`UPDATE users SET deleted_at = NULL, purge_after = NULL, is_active = TRUE, updated_at = NOW() WHERE id = $1 AND deleted_at IS NOT NULL`,
		userID,
	)
	if err != nil {
		return fmt.Errorf("restore account: %w", err)
	}
	rows, _ := res.RowsAffected()
	if rows == 0 {
		return ErrUserNotFound
	}
	return nil
}

// PurgeExpiredAccounts removes PII for accounts whose purge_after has passed.
// Returns the number of accounts purged.
func (r *UserRepository) PurgeExpiredAccounts(ctx context.Context, now time.Time) ([]string, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id FROM users WHERE purge_after IS NOT NULL AND purge_after <= $1 AND email NOT LIKE 'purged-%'`,
		now,
	)
	if err != nil {
		return nil, fmt.Errorf("list expired accounts: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	for _, id := range ids {
		if err := r.purgeUser(ctx, id); err != nil {
			return nil, fmt.Errorf("purge user %s: %w", id, err)
		}
	}
	return ids, nil
}

func (r *UserRepository) purgeUser(ctx context.Context, userID string) error {
	return platformdb.RunInTx(ctx, r.db, func(tx *sql.Tx) error {
		stubEmail := fmt.Sprintf("purged-%s@example.invalid", userID)
		if _, err := tx.ExecContext(ctx, `UPDATE users SET
		email = $2,
		username = NULL,
		display_name = NULL,
		avatar_url = NULL,
		password_hash = NULL,
		is_active = FALSE,
		email_verified = FALSE,
		frozen_at = NULL,
		purge_after = NULL,
		updated_at = NOW()
		WHERE id = $1`, userID, stubEmail); err != nil {
		return fmt.Errorf("scrub user row: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM oauth_accounts       WHERE user_id = $1`, userID); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM passkeys              WHERE user_id = $1`, userID); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM sessions              WHERE user_id = $1`, userID); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM email_verification_tokens WHERE user_id = $1`, userID); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM password_reset_tokens WHERE user_id = $1`, userID); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM password_history      WHERE user_id = $1`, userID); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM email_change_requests WHERE user_id = $1`, userID); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM known_devices         WHERE user_id = $1`, userID); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM account_unlock_tokens WHERE user_id = $1`, userID); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM user_mfa              WHERE user_id = $1`, userID); err != nil {
			return err
		}
		return nil
	})
}

// MarkPasswordChanged updates the password_changed_at timestamp; called from
// register / password reset / password change flows.
func (r *UserRepository) MarkPasswordChanged(ctx context.Context, userID string) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE users SET password_changed_at = NOW(), updated_at = NOW() WHERE id = $1`,
		userID,
	)
	return err
}

// DeviceFingerprint hashes (user_agent + IP /24 prefix) into a stable short
// identifier; not security-grade, sufficient to detect "new device" heuristically.
func DeviceFingerprint(userAgent, ipAddress string) string {
	ip := strings.TrimSpace(ipAddress)
	// Truncate IPv4 to /24, IPv6 to /48 to absorb dynamic IP shifts.
	if i := strings.LastIndex(ip, "."); i > 0 && strings.Count(ip, ".") == 3 {
		ip = ip[:i] + ".0"
	} else if strings.Contains(ip, ":") {
		segs := strings.Split(ip, ":")
		if len(segs) > 3 {
			ip = strings.Join(segs[:3], ":")
		}
	}
	sum := sha256.Sum256([]byte(strings.TrimSpace(userAgent) + "|" + ip))
	return hex.EncodeToString(sum[:8])
}

func (r *UserRepository) RecordKnownDevice(ctx context.Context, userID, fingerprint string, ua, ip *string) (isNew bool, err error) {
	var exists bool
	err = r.db.QueryRowContext(ctx, `
		SELECT EXISTS(SELECT 1 FROM known_devices WHERE user_id = $1 AND fingerprint = $2)
	`, userID, fingerprint).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("check known device: %w", err)
	}

	if exists {
		_, err = r.db.ExecContext(ctx, `
			UPDATE known_devices SET last_seen_at = NOW() WHERE user_id = $1 AND fingerprint = $2
		`, userID, fingerprint)
		if err != nil {
			return false, fmt.Errorf("update known device: %w", err)
		}
		return false, nil
	}

	_, err = r.db.ExecContext(ctx, `
		INSERT INTO known_devices (user_id, fingerprint, user_agent, ip_address)
		VALUES ($1, $2, $3, $4::inet)
		ON CONFLICT (user_id, fingerprint) DO UPDATE SET last_seen_at = NOW()
	`, userID, fingerprint, ua, ip)
	if err != nil {
		return false, fmt.Errorf("insert known device: %w", err)
	}

	return true, nil
}

func (r *UserRepository) CreateUnlockToken(ctx context.Context, userID string, ttl time.Duration) (string, error) {
	tok, err := generateOpaque(32)
	if err != nil {
		return "", err
	}
	_, err = r.db.ExecContext(ctx,
		`INSERT INTO account_unlock_tokens (token, user_id, expires_at) VALUES ($1, $2, $3)`,
		tok, userID, time.Now().Add(ttl),
	)
	if err != nil {
		return "", fmt.Errorf("insert unlock token: %w", err)
	}
	return tok, nil
}

func (r *UserRepository) ConsumeUnlockToken(ctx context.Context, token string) (string, error) {
	var userID string
	err := r.db.QueryRowContext(ctx, `
		UPDATE account_unlock_tokens SET consumed_at = NOW()
		WHERE token = $1 AND consumed_at IS NULL AND expires_at > NOW()
		RETURNING user_id
	`, token).Scan(&userID)
	if errors.Is(err, sql.ErrNoRows) {
		return "", ErrUnlockTokenInvalid
	}
	if err != nil {
		return "", fmt.Errorf("consume unlock token: %w", err)
	}
	return userID, nil
}

func generateOpaque(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(b), nil
}
