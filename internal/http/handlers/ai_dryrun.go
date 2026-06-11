package handlers

import (
	"context"
	"database/sql"
	"strings"

	"github.com/biqly/biqly/internal/ai"
	"github.com/biqly/biqly/internal/core"
	"github.com/biqly/biqly/internal/datasource"
	"github.com/biqly/biqly/internal/query"
	"github.com/biqly/biqly/internal/semantic"
	"github.com/biqly/biqly/pkg/queryclient"
)

// newSQLDryRunValidator returns an ai.SQLValidator that compiles a candidate
// LogicalQuery and runs the dialect-specific EXPLAIN form against db. Dialects
// without single-statement EXPLAIN support (e.g. SQL Server) return "" and the
// validator no-ops — keeping the retry loop dialect-portable.
func newSQLDryRunValidator(service *core.QueryService, db *sql.DB, driver datasource.Driver, model *semantic.SemanticModel) ai.SQLValidator {
	return func(ctx context.Context, lq *query.LogicalQuery) error {
		return core.ErrAsError(service.DryRun(ctx, db, lq, model, driver))
	}
}

func newQueryClientDryRunValidator(client *queryclient.Client, model *semantic.SemanticModel) ai.SQLValidator {
	inline := inlineAutoModel(model)
	return func(ctx context.Context, lq *query.LogicalQuery) error {
		_, err := client.DryRunWithModel(ctx, lq, inline)
		return err
	}
}

// inlineAutoModel returns model when it is a synthetic auto-routing model
// (ID prefixed "auto:") that exists only in this process and therefore must
// travel inline to the Query Engine. Published models return nil so the
// engine loads them from the catalog by LogicalQuery.ModelID.
func inlineAutoModel(model *semantic.SemanticModel) *semantic.SemanticModel {
	if model != nil && strings.HasPrefix(model.ID, "auto:") {
		return model
	}
	return nil
}
