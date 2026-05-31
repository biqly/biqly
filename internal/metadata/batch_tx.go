package metadata

import (
	"context"
	"database/sql"
	"fmt"

	platformdb "github.com/biqly/biqly/internal/platform/db"
)

func execBatchInTx(ctx context.Context, db *sql.DB, op string, fn func(*sql.Tx) error) error {
	if err := platformdb.RunInTx(ctx, db, fn); err != nil {
		return fmt.Errorf("%s: %w", op, err)
	}
	return nil
}
