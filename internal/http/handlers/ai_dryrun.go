package handlers

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/biqly/biqly/internal/ai"
	"github.com/biqly/biqly/internal/dialect"
	"github.com/biqly/biqly/internal/query"
	"github.com/biqly/biqly/internal/semantic"
)

// newSQLDryRunValidator returns an ai.SQLValidator that compiles a candidate
// LogicalQuery and runs the dialect-specific EXPLAIN form against db. Dialects
// without single-statement EXPLAIN support (e.g. SQL Server) return "" and the
// validator no-ops — keeping the retry loop dialect-portable.
func newSQLDryRunValidator(db *sql.DB, compiler *query.Compiler, d dialect.Dialect, model *semantic.SemanticModel) ai.SQLValidator {
	return func(ctx context.Context, lq *query.LogicalQuery) error {
		cq, err := compiler.Compile(ctx, *lq, model)
		if err != nil {
			return fmt.Errorf("compile: %w", err)
		}
		explain := d.ExplainSQL(cq.SQL)
		if explain == "" {
			return nil
		}
		rows, err := db.QueryContext(ctx, explain, cq.Args...)
		if err != nil {
			return fmt.Errorf("explain: %w", err)
		}
		_ = rows.Close()
		return nil
	}
}
