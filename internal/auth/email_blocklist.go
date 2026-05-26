package auth

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

// EmailBlockListRepo persists addresses that must not receive any
// transactional email — typically because they bounced, were marked as spam,
// or were manually blocked by an operator.
type EmailBlockListRepo interface {
	IsBlocked(ctx context.Context, email string) (bool, error)
	Block(ctx context.Context, email, reason, createdBy string) error
	Unblock(ctx context.Context, email string) error
	List(ctx context.Context, limit, offset int) ([]BlockedEmail, error)
}

type BlockedEmail struct {
	Email     string    `json:"email"`
	Reason    string    `json:"reason"`
	BlockedAt time.Time `json:"blocked_at"`
	CreatedBy string    `json:"created_by,omitempty"`
}

type sqlEmailBlockListRepo struct {
	db *sql.DB
}

// NewEmailBlockListRepo returns a PostgreSQL-backed block list repository.
func NewEmailBlockListRepo(db *sql.DB) EmailBlockListRepo {
	if db == nil {
		return nil
	}
	return &sqlEmailBlockListRepo{db: db}
}

func (r *sqlEmailBlockListRepo) IsBlocked(ctx context.Context, email string) (bool, error) {
	if email == "" {
		return false, nil
	}
	normalized, err := NormalizeEmail(email)
	if err != nil {
		return false, err
	}
	var exists bool
	err = r.db.QueryRowContext(ctx,
		"SELECT EXISTS (SELECT 1 FROM email_block_list WHERE email = $1)",
		normalized,
	).Scan(&exists)
	if err != nil {
		return false, err
	}
	return exists, nil
}

func (r *sqlEmailBlockListRepo) Block(ctx context.Context, email, reason, createdBy string) error {
	normalized, err := NormalizeEmail(email)
	if err != nil {
		return err
	}
	if reason == "" {
		return errors.New("block reason is required")
	}
	var creator any
	if createdBy != "" {
		creator = createdBy
	}
	_, err = r.db.ExecContext(ctx,
		`INSERT INTO email_block_list (email, reason, created_by) VALUES ($1, $2, $3)
		 ON CONFLICT (email) DO UPDATE SET reason = EXCLUDED.reason, blocked_at = NOW(), created_by = EXCLUDED.created_by`,
		normalized, reason, creator,
	)
	return err
}

func (r *sqlEmailBlockListRepo) Unblock(ctx context.Context, email string) error {
	normalized, err := NormalizeEmail(email)
	if err != nil {
		return err
	}
	_, err = r.db.ExecContext(ctx, "DELETE FROM email_block_list WHERE email = $1", normalized)
	return err
}

func (r *sqlEmailBlockListRepo) List(ctx context.Context, limit, offset int) ([]BlockedEmail, error) {
	if limit <= 0 || limit > 1000 {
		limit = 100
	}
	if offset < 0 {
		offset = 0
	}
	rows, err := r.db.QueryContext(ctx,
		`SELECT email, reason, blocked_at, COALESCE(created_by, '') FROM email_block_list ORDER BY blocked_at DESC LIMIT $1 OFFSET $2`,
		limit, offset,
	)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var out []BlockedEmail
	for rows.Next() {
		var b BlockedEmail
		if err := rows.Scan(&b.Email, &b.Reason, &b.BlockedAt, &b.CreatedBy); err != nil {
			return nil, err
		}
		out = append(out, b)
	}
	return out, rows.Err()
}

// memoryEmailBlockListRepo is an in-memory implementation suitable for tests
// and for environments running without persistent storage.
type memoryEmailBlockListRepo struct {
	entries map[string]BlockedEmail
}

func NewMemoryEmailBlockListRepo() EmailBlockListRepo {
	return &memoryEmailBlockListRepo{entries: map[string]BlockedEmail{}}
}

func (r *memoryEmailBlockListRepo) IsBlocked(_ context.Context, email string) (bool, error) {
	normalized, err := NormalizeEmail(email)
	if err != nil {
		return false, err
	}
	_, ok := r.entries[normalized]
	return ok, nil
}

func (r *memoryEmailBlockListRepo) Block(_ context.Context, email, reason, createdBy string) error {
	normalized, err := NormalizeEmail(email)
	if err != nil {
		return err
	}
	if reason == "" {
		return errors.New("block reason is required")
	}
	r.entries[normalized] = BlockedEmail{Email: normalized, Reason: reason, BlockedAt: time.Now(), CreatedBy: createdBy}
	return nil
}

func (r *memoryEmailBlockListRepo) Unblock(_ context.Context, email string) error {
	normalized, err := NormalizeEmail(email)
	if err != nil {
		return err
	}
	delete(r.entries, normalized)
	return nil
}

func (r *memoryEmailBlockListRepo) List(_ context.Context, limit, offset int) ([]BlockedEmail, error) {
	out := make([]BlockedEmail, 0, len(r.entries))
	for _, v := range r.entries {
		out = append(out, v)
	}
	if offset >= len(out) {
		return nil, nil
	}
	end := offset + limit
	if limit <= 0 || end > len(out) {
		end = len(out)
	}
	return out[offset:end], nil
}
