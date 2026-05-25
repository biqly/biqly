package auth

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"
)

type AuditEntry struct {
	ID         string          `json:"id"`
	UserID     *string         `json:"user_id,omitempty"`
	Action     string          `json:"action"`
	Resource   *string         `json:"resource,omitempty"`
	ResourceID *string         `json:"resource_id,omitempty"`
	Metadata   json.RawMessage `json:"metadata,omitempty"`
	IPAddress  *string         `json:"ip_address,omitempty"`
	CreatedAt  time.Time       `json:"created_at"`
}

type AuditFilter struct {
	UserID string
	Action string
	Limit  int
}

type AuditService struct {
	db *sql.DB
}

func NewAuditService(db *sql.DB) *AuditService {
	return &AuditService{db: db}
}

func (s *AuditService) Log(ctx context.Context, userID *string, action string, resource, resourceID *string, metadata any, ipAddress *string) error {
	var metaJSON []byte
	if metadata != nil {
		var err error
		metaJSON, err = json.Marshal(metadata)
		if err != nil {
			return fmt.Errorf("marshal audit metadata: %w", err)
		}
	}

	_, err := s.db.ExecContext(ctx, `
		INSERT INTO audit_log (user_id, action, resource, resource_id, metadata, ip_address)
		VALUES ($1, $2, $3, $4, $5::jsonb, $6::inet)
	`, userID, action, resource, resourceID, sql.NullString{String: string(metaJSON), Valid: len(metaJSON) > 0}, ipAddress)
	if err != nil {
		return fmt.Errorf("insert audit log: %w", err)
	}
	return nil
}

func (s *AuditService) List(ctx context.Context, filter AuditFilter) ([]AuditEntry, error) {
	limit := filter.Limit
	if limit <= 0 {
		limit = 100
	}
	if limit > 1000 {
		limit = 1000
	}

	const queryAll = `SELECT id, user_id, action, resource, resource_id, metadata, ip_address::text, created_at
		FROM audit_log ORDER BY created_at DESC LIMIT $1`
	const queryByUser = `SELECT id, user_id, action, resource, resource_id, metadata, ip_address::text, created_at
		FROM audit_log WHERE user_id = $1 ORDER BY created_at DESC LIMIT $2`
	const queryByAction = `SELECT id, user_id, action, resource, resource_id, metadata, ip_address::text, created_at
		FROM audit_log WHERE action = $1 ORDER BY created_at DESC LIMIT $2`
	const queryByUserAction = `SELECT id, user_id, action, resource, resource_id, metadata, ip_address::text, created_at
		FROM audit_log WHERE user_id = $1 AND action = $2 ORDER BY created_at DESC LIMIT $3`

	var (
		rows *sql.Rows
		err  error
	)
	switch {
	case filter.UserID != "" && filter.Action != "":
		rows, err = s.db.QueryContext(ctx, queryByUserAction, filter.UserID, filter.Action, limit)
	case filter.UserID != "":
		rows, err = s.db.QueryContext(ctx, queryByUser, filter.UserID, limit)
	case filter.Action != "":
		rows, err = s.db.QueryContext(ctx, queryByAction, filter.Action, limit)
	default:
		rows, err = s.db.QueryContext(ctx, queryAll, limit)
	}
	if err != nil {
		return nil, fmt.Errorf("query audit log: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var entries []AuditEntry
	for rows.Next() {
		var e AuditEntry
		var userID, resource, resourceID, ip sql.NullString
		var metadata []byte
		if err := rows.Scan(&e.ID, &userID, &e.Action, &resource, &resourceID, &metadata, &ip, &e.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan audit entry: %w", err)
		}
		if userID.Valid {
			s := userID.String
			e.UserID = &s
		}
		if resource.Valid {
			s := resource.String
			e.Resource = &s
		}
		if resourceID.Valid {
			s := resourceID.String
			e.ResourceID = &s
		}
		if ip.Valid {
			s := ip.String
			e.IPAddress = &s
		}
		if len(metadata) > 0 {
			e.Metadata = metadata
		}
		entries = append(entries, e)
	}
	return entries, rows.Err()
}
