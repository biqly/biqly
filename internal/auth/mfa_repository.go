package auth

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/biqly/biqly/internal/security"
	"github.com/lib/pq"
)

var ErrMFANotEnrolled = errors.New("mfa not enrolled")

type MFAEnrollment struct {
	UserID         string
	Method         string
	Secret         string
	RecoveryCodes  []string
	BypassCodes    []string
	Enabled        bool
	VerifiedAt     *time.Time
	LastUsedAt     *time.Time
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

type MFARepository struct {
	db  *sql.DB
	enc *security.Encryption
}

func NewMFARepository(db *sql.DB, enc *security.Encryption) *MFARepository {
	return &MFARepository{db: db, enc: enc}
}

func (r *MFARepository) encryptSecret(secret string) (string, error) {
	if r.enc == nil {
		return secret, nil
	}
	return r.enc.Encrypt(secret)
}

func (r *MFARepository) decryptSecret(value string) (string, error) {
	if r.enc == nil {
		return value, nil
	}
	return r.enc.Decrypt(value)
}

// Upsert stores a pending enrollment (enabled=false until verified).
func (r *MFARepository) Upsert(ctx context.Context, userID, method, secret string, recoveryHashes []string) error {
	encSecret, err := r.encryptSecret(secret)
	if err != nil {
		return err
	}
	query := `
		INSERT INTO user_mfa (user_id, method, secret_encrypted, recovery_codes, bypass_codes, enabled, verified_at, updated_at)
		VALUES ($1, $2, $3, $4, '{}', FALSE, NULL, NOW())
		ON CONFLICT (user_id) DO UPDATE SET
			method = EXCLUDED.method,
			secret_encrypted = EXCLUDED.secret_encrypted,
			recovery_codes = EXCLUDED.recovery_codes,
			bypass_codes = '{}',
			enabled = FALSE,
			verified_at = NULL,
			updated_at = NOW()
	`
	_, err = r.db.ExecContext(ctx, query, userID, method, []byte(encSecret), pq.Array(recoveryHashes))
	return err
}

func (r *MFARepository) Get(ctx context.Context, userID string) (*MFAEnrollment, error) {
	var enc []byte
	var codes pq.StringArray
	var bypassCodes pq.StringArray
	var enrol MFAEnrollment
	var verifiedAt, lastUsedAt sql.NullTime
	err := r.db.QueryRowContext(ctx, `
		SELECT user_id, method, secret_encrypted, recovery_codes, bypass_codes, enabled, verified_at, last_used_at, created_at, updated_at
		FROM user_mfa WHERE user_id = $1
	`, userID).Scan(
		&enrol.UserID, &enrol.Method, &enc, &codes, &bypassCodes, &enrol.Enabled,
		&verifiedAt, &lastUsedAt, &enrol.CreatedAt, &enrol.UpdatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrMFANotEnrolled
	} else if err != nil {
		return nil, err
	}
	secret, err := r.decryptSecret(string(enc))
	if err != nil {
		return nil, err
	}
	enrol.Secret = secret
	enrol.RecoveryCodes = []string(codes)
	enrol.BypassCodes = []string(bypassCodes)
	if verifiedAt.Valid {
		t := verifiedAt.Time
		enrol.VerifiedAt = &t
	}
	if lastUsedAt.Valid {
		t := lastUsedAt.Time
		enrol.LastUsedAt = &t
	}
	return &enrol, nil
}

func (r *MFARepository) Enable(ctx context.Context, userID string) error {
	_, err := r.db.ExecContext(ctx, `
		UPDATE user_mfa SET enabled = TRUE, verified_at = COALESCE(verified_at, NOW()), updated_at = NOW()
		WHERE user_id = $1
	`, userID)
	return err
}

func (r *MFARepository) Disable(ctx context.Context, userID string) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM user_mfa WHERE user_id = $1`, userID)
	return err
}

func (r *MFARepository) MarkUsed(ctx context.Context, userID string) error {
	_, err := r.db.ExecContext(ctx, `UPDATE user_mfa SET last_used_at = NOW(), updated_at = NOW() WHERE user_id = $1`, userID)
	return err
}

// ConsumeRecoveryCode removes a single matched hash from the array.
// Returns true if the code was found and removed.
func (r *MFARepository) ConsumeRecoveryCode(ctx context.Context, userID, hash string) (bool, error) {
	res, err := r.db.ExecContext(ctx, `
		UPDATE user_mfa SET recovery_codes = array_remove(recovery_codes, $2), updated_at = NOW()
		WHERE user_id = $1 AND $2 = ANY(recovery_codes)
	`, userID, hash)
	if err != nil {
		return false, err
	}
	rows, _ := res.RowsAffected()
	return rows > 0, nil
}

func (r *MFARepository) ReplaceRecoveryCodes(ctx context.Context, userID string, hashes []string) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE user_mfa SET recovery_codes = $2, updated_at = NOW() WHERE user_id = $1`,
		userID, pq.Array(hashes))
	return err
}

func (r *MFARepository) AddBypassCode(ctx context.Context, userID, hash string) error {
	_, err := r.db.ExecContext(ctx, `
		UPDATE user_mfa SET bypass_codes = array_append(bypass_codes, $2), updated_at = NOW()
		WHERE user_id = $1
	`, userID, hash)
	return err
}

func (r *MFARepository) ConsumeBypassCode(ctx context.Context, userID, hash string) (bool, error) {
	res, err := r.db.ExecContext(ctx, `
		UPDATE user_mfa SET bypass_codes = array_remove(bypass_codes, $2), updated_at = NOW()
		WHERE user_id = $1 AND $2 = ANY(bypass_codes)
	`, userID, hash)
	if err != nil {
		return false, err
	}
	rows, _ := res.RowsAffected()
	return rows > 0, nil
}
