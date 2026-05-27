package auth

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
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
	From   *time.Time
	To     *time.Time
	Limit  int
	Offset int
}

type AuditService struct {
	db     *sql.DB
	logger *slog.Logger
}

func NewAuditService(db *sql.DB) *AuditService {
	return &AuditService{db: db, logger: slog.Default().With("subsystem", "audit")}
}

func (s *AuditService) WithLogger(l *slog.Logger) *AuditService {
	return &AuditService{db: s.db, logger: l.With("subsystem", "audit")}
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
	s.emitStructured(ctx, userID, action, resource, resourceID, ipAddress, metadata, AuditResultSuccess)
	return nil
}

func (s *AuditService) LogResult(ctx context.Context, userID *string, action string, resource, resourceID *string, metadata any, ipAddress *string, result AuditResult) error {
	if err := s.Log(ctx, userID, action, resource, resourceID, metadata, ipAddress); err != nil {
		return err
	}
	return nil
}

func (s *AuditService) emitStructured(_ context.Context, userID *string, action string, resource, resourceID, ipAddress *string, metadata any, result AuditResult) {
	if s.logger == nil {
		return
	}
	attrs := []slog.Attr{
		slog.String("event", action),
		slog.String("result", string(result)),
	}
	if userID != nil {
		attrs = append(attrs, slog.String("user_id", *userID))
	}
	if resource != nil {
		attrs = append(attrs, slog.String("resource", *resource))
	}
	if resourceID != nil {
		attrs = append(attrs, slog.String("resource_id", *resourceID))
	}
	if ipAddress != nil {
		attrs = append(attrs, slog.String("ip", MaskIP(*ipAddress)))
	}
	if metadata != nil {
		masked := maskAuditMetadata(metadata)
		if masked != nil {
			attrs = append(attrs, slog.Any("metadata", masked))
		}
	}
	s.logger.LogAttrs(context.Background(), slog.LevelInfo, "audit", attrs...)
}

func maskAuditMetadata(metadata any) any {
	bs, err := json.Marshal(metadata)
	if err != nil {
		return nil
	}
	var m map[string]any
	if err := json.Unmarshal(bs, &m); err != nil {
		return nil
	}
	for k, v := range m {
		s, ok := v.(string)
		if !ok {
			continue
		}
		switch k {
		case "email", "old_email", "new_email":
			m[k] = MaskEmail(s)
		case "ip", "ip_address", "remote_addr":
			m[k] = MaskIP(s)
		case "token", "refresh_token", "access_token", "recovery_code":
			m[k] = MaskToken(s)
		}
	}
	return m
}

func (s *AuditService) Count(ctx context.Context, filter AuditFilter) (int, error) {
	var parts []string
	args := []any{}
	idx := 1
	if filter.UserID != "" {
		parts = append(parts, fmt.Sprintf("user_id = $%d", idx))
		args = append(args, filter.UserID)
		idx++
	}
	if filter.Action != "" {
		parts = append(parts, fmt.Sprintf("action = $%d", idx))
		args = append(args, filter.Action)
		idx++
	}
	if filter.From != nil {
		parts = append(parts, fmt.Sprintf("created_at >= $%d", idx))
		args = append(args, *filter.From)
		idx++
	}
	if filter.To != nil {
		parts = append(parts, fmt.Sprintf("created_at < $%d", idx))
		args = append(args, *filter.To)
	}
	where := ""
	if len(parts) > 0 {
		where = " WHERE " + strings.Join(parts, " AND ")
	}
	q := "SELECT COUNT(*) FROM audit_log" + where // nosemgrep: go.lang.security.audit.database.string-formatted-query.string-formatted-query
	var count int
	err := s.db.QueryRowContext(ctx, q, args...).Scan(&count)
	return count, err
}

func (s *AuditService) List(ctx context.Context, filter AuditFilter) ([]AuditEntry, error) {
	limit := filter.Limit
	if limit <= 0 {
		limit = 100
	}
	if limit > 1000 {
		limit = 1000
	}
	offset := filter.Offset
	if offset < 0 {
		offset = 0
	}

	var parts []string
	args := []any{}
	idx := 1
	if filter.UserID != "" {
		parts = append(parts, fmt.Sprintf("user_id = $%d", idx))
		args = append(args, filter.UserID)
		idx++
	}
	if filter.Action != "" {
		parts = append(parts, fmt.Sprintf("action = $%d", idx))
		args = append(args, filter.Action)
		idx++
	}
	if filter.From != nil {
		parts = append(parts, fmt.Sprintf("created_at >= $%d", idx))
		args = append(args, *filter.From)
		idx++
	}
	if filter.To != nil {
		parts = append(parts, fmt.Sprintf("created_at < $%d", idx))
		args = append(args, *filter.To)
		idx++
	}
	where := ""
	if len(parts) > 0 {
		where = " WHERE " + strings.Join(parts, " AND ")
	}
	args = append(args, limit)
	offsetPlaceholder := ""
	if offset > 0 {
		args = append(args, offset)
		offsetPlaceholder = fmt.Sprintf(" OFFSET $%d", idx+1)
	}
	q := fmt.Sprintf(
		`SELECT id, user_id, action, resource, resource_id, metadata, ip_address::text, created_at
		FROM audit_log%s ORDER BY created_at DESC LIMIT $%d%s`, where, idx, offsetPlaceholder)

	rows, err := s.db.QueryContext(ctx, q, args...)
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

var ErrAuditImmutable = errors.New("audit_log is append-only")
