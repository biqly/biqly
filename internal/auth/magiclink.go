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
	"time"

	platformdb "github.com/biqly/biqly/internal/platform/db"
)

const (
	// MagicLinkTokenTTL is how long a generated magic-link token remains
	// valid. Short enough to bound the window of exposure if the email is
	// intercepted; long enough to survive an MX-side delay.
	MagicLinkTokenTTL = 10 * time.Minute

	// MagicLinkRequestCooldown rate-limits how often the same address can
	// request a magic link. Enforced via Redis when available; falls back
	// to "no limit" when Redis is not wired.
	MagicLinkRequestCooldown = 60 * time.Second
)

var (
	ErrMagicLinkInvalid = errors.New("magic link token is invalid or expired")
	ErrMagicLinkUsed    = errors.New("magic link token has already been used")
)

// MagicLinkRepository persists the hashed magic-link tokens so the plaintext
// is never stored at rest.
type MagicLinkRepository struct {
	db *sql.DB
}

func NewMagicLinkRepository(db *sql.DB) *MagicLinkRepository {
	return &MagicLinkRepository{db: db}
}

// Issue stores a hashed token and returns nothing — callers already have the
// plaintext (which must be sent to the user's email). user_id may be empty
// if the address has no existing account; consumption will then return
// ErrMagicLinkInvalid so non-users cannot create accounts through the link.
func (r *MagicLinkRepository) Issue(ctx context.Context, plaintext, email, userID, ipAddress string, expiresAt time.Time) error {
	hash := hashMagicLink(plaintext)
	var uid, ip any
	if userID != "" {
		uid = userID
	}
	if ipAddress != "" {
		ip = ipAddress
	}
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO magic_link_tokens (token_hash, email, user_id, expires_at, ip_address)
		 VALUES ($1, $2, $3, $4, $5)`,
		hash, email, uid, expiresAt, ip,
	)
	return err
}

// Consume atomically marks the token as used and returns the linked user ID
// (or error if the token is unknown, expired, or already consumed).
func (r *MagicLinkRepository) Consume(ctx context.Context, plaintext string) (string, error) {
	hash := hashMagicLink(plaintext)
	var userID sql.NullString
	err := platformdb.RunInTx(ctx, r.db, func(tx *sql.Tx) error {
		var (
			expiresAt  time.Time
			consumedAt sql.NullTime
		)
		err := tx.QueryRowContext(ctx,
			`SELECT user_id, expires_at, consumed_at FROM magic_link_tokens WHERE token_hash = $1 FOR UPDATE`,
			hash,
		).Scan(&userID, &expiresAt, &consumedAt)
		if errors.Is(err, sql.ErrNoRows) {
			return ErrMagicLinkInvalid
		}
		if err != nil {
			return err
		}
		if consumedAt.Valid {
			return ErrMagicLinkUsed
		}
		if time.Now().After(expiresAt) {
			return ErrMagicLinkInvalid
		}
		if !userID.Valid || userID.String == "" {
			return ErrMagicLinkInvalid
		}
		_, err = tx.ExecContext(ctx, `UPDATE magic_link_tokens SET consumed_at = NOW() WHERE token_hash = $1`, hash)
		return err
	})
	if err != nil {
		return "", err
	}
	return userID.String, nil
}

// PurgeExpired drops tokens whose expires_at has passed by at least the
// supplied grace period. Intended to be called from a periodic janitor.
func (r *MagicLinkRepository) PurgeExpired(ctx context.Context, grace time.Duration) (int64, error) {
	cutoff := time.Now().Add(-grace)
	res, err := r.db.ExecContext(ctx, `DELETE FROM magic_link_tokens WHERE expires_at < $1`, cutoff)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

func hashMagicLink(plaintext string) string {
	sum := sha256.Sum256([]byte(plaintext))
	return hex.EncodeToString(sum[:])
}

func generateMagicLinkToken() (string, error) {
	var b [32]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", fmt.Errorf("read random bytes: %w", err)
	}
	return base64.URLEncoding.WithPadding(base64.NoPadding).EncodeToString(b[:]), nil
}
