package drift

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	platformdb "github.com/biqly/biqly/internal/platform/db"
	"github.com/bytedance/sonic"
)

// Repository handles database operations for drift reports.
type Repository struct {
	db *sql.DB
}

// NewRepository creates a new Repository.
func NewRepository(db *sql.DB) *Repository {
	return &Repository{db: db}
}

// InsertReport persists a new drift report.
func (r *Repository) InsertReport(ctx context.Context, report *DriftReport) error {
	driftsJSON, err := sonic.Marshal(report.Drifts)
	if err != nil {
		return fmt.Errorf("marshal drifts: %w", err)
	}

	query := `
		INSERT INTO drift_reports (model_id, datasource_id, sync_event_id, severity, drifts, resolved, resolved_by, resolved_at, detected_at, created_at)
		VALUES ($1::uuid, $2::uuid, $3::uuid, $4, $5::jsonb, $6, $7, $8, $9, now())
		RETURNING id::text
	`
	err = r.db.QueryRowContext(
		ctx,
		query,
		report.ModelID,
		report.DatasourceID,
		report.SyncEventID,
		report.Severity,
		driftsJSON,
		report.Resolved,
		report.ResolvedBy,
		report.ResolvedAt,
		report.DetectedAt,
	).Scan(&report.ID)
	if err != nil {
		return fmt.Errorf("insert drift report: %w", err)
	}
	return nil
}

// ListUnresolvedByModel fetches unresolved drift reports for a semantic model.
func (r *Repository) ListUnresolvedByModel(ctx context.Context, modelID string) ([]DriftReport, error) {
	query := driftReportSelectSQL() + ` WHERE model_id = $1::uuid AND resolved = false ORDER BY detected_at DESC`
	return platformdb.QuerySliceErr(ctx, r.db, "list unresolved by model", query, []any{modelID}, scanDriftReport)
}

// ListUnresolvedByDatasource fetches unresolved drift reports across all models of a datasource.
func (r *Repository) ListUnresolvedByDatasource(ctx context.Context, dsID string) ([]DriftReport, error) {
	query := driftReportSelectSQL() + ` WHERE datasource_id = $1::uuid AND resolved = false ORDER BY detected_at DESC`
	return platformdb.QuerySliceErr(ctx, r.db, "list unresolved by datasource", query, []any{dsID}, scanDriftReport)
}

// ResolveReport marks a drift report as resolved by a user.
func (r *Repository) ResolveReport(ctx context.Context, id, resolvedBy string) error {
	query := `
		UPDATE drift_reports
		SET resolved = true, resolved_by = $2, resolved_at = now()
		WHERE id = $1::uuid
	`
	_, err := r.db.ExecContext(ctx, query, id, resolvedBy)
	if err != nil {
		return fmt.Errorf("resolve drift report: %w", err)
	}
	return nil
}

// GetLatestByModel retrieves the latest drift report for a model, resolved or not.
func (r *Repository) GetLatestByModel(ctx context.Context, modelID string) (*DriftReport, error) {
	query := `
		SELECT id::text, model_id::text, datasource_id::text, sync_event_id::text, severity, drifts, resolved, resolved_by, resolved_at, detected_at, created_at
		FROM drift_reports
		WHERE model_id = $1::uuid
		ORDER BY detected_at DESC
		LIMIT 1
	`
	row := r.db.QueryRowContext(ctx, query, modelID)

	var rpt DriftReport
	var driftsJSON []byte
	err := row.Scan(
		&rpt.ID,
		&rpt.ModelID,
		&rpt.DatasourceID,
		&rpt.SyncEventID,
		&rpt.Severity,
		&driftsJSON,
		&rpt.Resolved,
		&rpt.ResolvedBy,
		&rpt.ResolvedAt,
		&rpt.DetectedAt,
		&rpt.CreatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil //nolint:nilnil // no prior report is not an error
	}
	if err != nil {
		return nil, fmt.Errorf("get latest by model scan: %w", err)
	}

	if len(driftsJSON) > 0 {
		if err := sonic.Unmarshal(driftsJSON, &rpt.Drifts); err != nil {
			return nil, fmt.Errorf("decode drifts: %w", err)
		}
	} else {
		rpt.Drifts = []DriftItem{}
	}
	return &rpt, nil
}

func driftReportSelectSQL() string {
	return `SELECT id::text, model_id::text, datasource_id::text, sync_event_id::text, severity, drifts, resolved, resolved_by, resolved_at, detected_at, created_at FROM drift_reports`
}

func scanDriftReport(s platformdb.Scanner) (DriftReport, error) {
	var r DriftReport
	var driftsJSON []byte
	err := s.Scan(
		&r.ID,
		&r.ModelID,
		&r.DatasourceID,
		&r.SyncEventID,
		&r.Severity,
		&driftsJSON,
		&r.Resolved,
		&r.ResolvedBy,
		&r.ResolvedAt,
		&r.DetectedAt,
		&r.CreatedAt,
	)
	if err != nil {
		return r, err
	}
	if len(driftsJSON) > 0 {
		if err := sonic.Unmarshal(driftsJSON, &r.Drifts); err != nil {
			return r, fmt.Errorf("decode drifts: %w", err)
		}
	} else {
		r.Drifts = []DriftItem{}
	}
	return r, nil
}
