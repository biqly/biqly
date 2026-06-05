package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

// RunInTx begins a transaction on db, invokes fn with it, and commits when fn
// returns nil. If fn returns an error the transaction is rolled back and the
// error is returned directly unless rollback also fails. A failed begin or
// commit is wrapped for context.
func RunInTx(ctx context.Context, db *sql.DB, fn func(*sql.Tx) error) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	if err := fn(tx); err != nil {
		if rollbackErr := tx.Rollback(); rollbackErr != nil {
			return errors.Join(err, fmt.Errorf("rollback tx: %w", rollbackErr))
		}
		return err
	}
	if err := tx.Commit(); err != nil {
		if rollbackErr := tx.Rollback(); rollbackErr != nil && !errors.Is(rollbackErr, sql.ErrTxDone) {
			return errors.Join(fmt.Errorf("commit tx: %w", err), fmt.Errorf("rollback tx after commit failure: %w", rollbackErr))
		}
		return fmt.Errorf("commit tx: %w", err)
	}
	return nil
}
