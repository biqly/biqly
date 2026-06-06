package rbac

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

type AccessLevel string

var ErrDatasourceAccessDenied = errors.New("datasource access denied")

type DatasourceAccess struct {
	ID           string      `json:"id"`
	UserID       string      `json:"user_id"`
	DatasourceID string      `json:"datasource_id"`
	AccessLevel  AccessLevel `json:"access_level"`
	GrantedBy    *string     `json:"granted_by,omitempty"`
	GrantedAt    time.Time   `json:"granted_at"`
}

type DatasourceAccessService struct {
	db       *sql.DB
	redis    *redis.Client
	rbac     *Service
	cacheTTL time.Duration
}

func NewDatasourceAccessService(db *sql.DB, redisClient *redis.Client, rbac *Service) *DatasourceAccessService {
	return &DatasourceAccessService{
		db:       db,
		redis:    redisClient,
		rbac:     rbac,
		cacheTTL: 5 * time.Minute,
	}
}

func (*DatasourceAccessService) cacheKey(userID string) string {
	return fmt.Sprintf("user:%s:datasources", userID)
}

func (s *DatasourceAccessService) Grant(ctx context.Context, userID, datasourceID, level string, grantedBy string) (*DatasourceAccess, error) {
	if !IsValidLevel(level) {
		return nil, fmt.Errorf("invalid access level: %s", level)
	}

	query := `
		INSERT INTO datasource_access (user_id, datasource_id, access_level, granted_by)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (user_id, datasource_id)
		DO UPDATE SET access_level = EXCLUDED.access_level, granted_by = EXCLUDED.granted_by, granted_at = NOW()
		RETURNING id, user_id, datasource_id, access_level, granted_by, granted_at
	`
	var grantedByVal sql.NullString
	if grantedBy != "" {
		grantedByVal = sql.NullString{String: grantedBy, Valid: true}
	}

	var access DatasourceAccess
	var grantedByNull sql.NullString
	err := s.db.QueryRowContext(ctx, query, userID, datasourceID, level, grantedByVal).Scan(
		&access.ID, &access.UserID, &access.DatasourceID, &access.AccessLevel, &grantedByNull, &access.GrantedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("grant datasource access: %w", err)
	}
	if grantedByNull.Valid {
		access.GrantedBy = new(grantedByNull.String)
	}

	if err := s.InvalidateCache(ctx, userID); err != nil {
		return nil, fmt.Errorf("invalidate datasource access cache: %w", err)
	}
	return &access, nil
}

func (s *DatasourceAccessService) Revoke(ctx context.Context, userID, datasourceID string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM datasource_access WHERE user_id = $1 AND datasource_id = $2`, userID, datasourceID)
	if err != nil {
		return fmt.Errorf("revoke datasource access: %w", err)
	}
	if err := s.InvalidateCache(ctx, userID); err != nil {
		return fmt.Errorf("invalidate datasource access cache: %w", err)
	}
	return nil
}

func (s *DatasourceAccessService) UpdateLevel(ctx context.Context, accessID, level string) error {
	if !IsValidLevel(level) {
		return fmt.Errorf("invalid access level: %s", level)
	}
	var userID string
	err := s.db.QueryRowContext(ctx,
		`UPDATE datasource_access SET access_level = $1 WHERE id = $2 RETURNING user_id`,
		level, accessID).Scan(&userID)
	if err != nil {
		return fmt.Errorf("update access level: %w", err)
	}
	if err := s.InvalidateCache(ctx, userID); err != nil {
		return fmt.Errorf("invalidate datasource access cache: %w", err)
	}
	return nil
}

func (s *DatasourceAccessService) ListUserAccess(ctx context.Context, userID string) ([]DatasourceAccess, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, user_id, datasource_id, access_level, granted_by, granted_at
		FROM datasource_access WHERE user_id = $1
	`, userID)
	if err != nil {
		return nil, fmt.Errorf("query user access: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var list []DatasourceAccess
	for rows.Next() {
		var a DatasourceAccess
		var grantedBy sql.NullString
		if err := rows.Scan(&a.ID, &a.UserID, &a.DatasourceID, &a.AccessLevel, &grantedBy, &a.GrantedAt); err != nil {
			return nil, err
		}
		if grantedBy.Valid {
			a.GrantedBy = new(grantedBy.String)
		}
		list = append(list, a)
	}
	return list, rows.Err()
}

func (s *DatasourceAccessService) ListAll(ctx context.Context) ([]DatasourceAccess, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, user_id, datasource_id, access_level, granted_by, granted_at
		FROM datasource_access ORDER BY granted_at DESC
	`)
	if err != nil {
		return nil, fmt.Errorf("query all access: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var list []DatasourceAccess
	for rows.Next() {
		var a DatasourceAccess
		var grantedBy sql.NullString
		if err := rows.Scan(&a.ID, &a.UserID, &a.DatasourceID, &a.AccessLevel, &grantedBy, &a.GrantedAt); err != nil {
			return nil, err
		}
		if grantedBy.Valid {
			a.GrantedBy = new(grantedBy.String)
		}
		list = append(list, a)
	}
	return list, rows.Err()
}

// ListAccessibleDatasourceIDs returns datasource IDs the user can access through
// direct grants or workspace membership. Result is cached in Redis.
func (s *DatasourceAccessService) ListAccessibleDatasourceIDs(ctx context.Context, userID string) ([]string, error) {
	if cached, err := s.getCached(ctx, userID); err == nil && cached != nil {
		return cached, nil
	}

	ids, err := s.queryAccessibleDatasourceIDs(ctx, userID)
	if err != nil {
		return nil, err
	}

	if err := s.setCached(ctx, userID, ids); err != nil {
		slog.WarnContext(ctx, "datasource access cache write failed", "user_id", userID, "error", err)
	}
	return ids, nil
}

func (s *DatasourceAccessService) queryAccessibleDatasourceIDs(ctx context.Context, userID string) ([]string, error) {
	query := `
		SELECT DISTINCT datasource_id FROM datasource_access WHERE user_id = $1
		UNION
		SELECT DISTINCT wd.datasource_id
		FROM workspace_datasources wd
		JOIN workspace_members wm ON wd.workspace_id = wm.workspace_id
		WHERE wm.user_id = $1
	`
	rows, err := s.db.QueryContext(ctx, query, userID)
	if err != nil {
		return nil, fmt.Errorf("query accessible datasources: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

// CheckAccess verifies the user has at least the required level for datasource.
// super_admin bypass applies.
func (s *DatasourceAccessService) CheckAccess(ctx context.Context, userID, datasourceID, requiredLevel string) (err error) {
	defer func() {
		if err == nil {
			MetricDatasourceAccessChecks.WithLabelValues("allowed").Inc()
		} else {
			MetricDatasourceAccessChecks.WithLabelValues("denied").Inc()
		}
	}()

	if !IsValidLevel(requiredLevel) {
		return fmt.Errorf("invalid required level: %s", requiredLevel)
	}

	if s.rbac != nil {
		isSuper, errSuper := s.rbac.IsSuperAdmin(ctx, userID)
		if errSuper == nil && isSuper {
			return nil
		}
	}

	var directLevel sql.NullString
	errDirect := s.db.QueryRowContext(ctx, `
		SELECT access_level FROM datasource_access WHERE user_id = $1 AND datasource_id = $2
	`, userID, datasourceID).Scan(&directLevel)
	if errDirect != nil && !errors.Is(errDirect, sql.ErrNoRows) {
		return fmt.Errorf("check direct access: %w", errDirect)
	}

	if directLevel.Valid && levelSatisfies(directLevel.String, requiredLevel) {
		return nil
	}

	var workspaceLevel sql.NullString
	errWorkspace := s.db.QueryRowContext(ctx, `
		SELECT wd.access_level
		FROM workspace_datasources wd
		JOIN workspace_members wm ON wd.workspace_id = wm.workspace_id
		WHERE wm.user_id = $1 AND wd.datasource_id = $2
		ORDER BY CASE wd.access_level
			WHEN 'admin' THEN 3 WHEN 'write' THEN 2 WHEN 'read' THEN 1 ELSE 0
		END DESC
		LIMIT 1
	`, userID, datasourceID).Scan(&workspaceLevel)
	if errWorkspace != nil && !errors.Is(errWorkspace, sql.ErrNoRows) {
		return fmt.Errorf("check workspace access: %w", errWorkspace)
	}

	if workspaceLevel.Valid && levelSatisfies(workspaceLevel.String, requiredLevel) {
		return nil
	}

	return ErrDatasourceAccessDenied
}

func (s *DatasourceAccessService) InvalidateCache(ctx context.Context, userID string) error {
	if s.redis == nil {
		return nil
	}
	return s.redis.Del(ctx, s.cacheKey(userID)).Err()
}

func (s *DatasourceAccessService) getCached(ctx context.Context, userID string) ([]string, error) {
	if s.redis == nil {
		return nil, errors.New("no redis")
	}
	members, err := s.redis.SMembers(ctx, s.cacheKey(userID)).Result()
	if err != nil {
		return nil, err
	}
	if len(members) == 0 {
		return nil, nil
	}
	// Sentinel value to distinguish "empty cache" from "no access"
	if len(members) == 1 && members[0] == "__none__" {
		return []string{}, nil
	}
	return members, nil
}

func (s *DatasourceAccessService) setCached(ctx context.Context, userID string, ids []string) error {
	if s.redis == nil {
		return nil
	}
	key := s.cacheKey(userID)
	pipe := s.redis.TxPipeline()
	pipe.Del(ctx, key)
	if len(ids) == 0 {
		pipe.SAdd(ctx, key, "__none__")
	} else {
		args := make([]any, len(ids))
		for i, id := range ids {
			args[i] = id
		}
		pipe.SAdd(ctx, key, args...)
	}
	pipe.Expire(ctx, key, s.cacheTTL)
	_, err := pipe.Exec(ctx)
	return err
}

func IsValidLevel(level string) bool {
	switch strings.ToLower(level) {
	case "read", "write", "admin":
		return true
	}
	return false
}

func levelSatisfies(have, required string) bool {
	rank := map[string]int{"read": 1, "write": 2, "admin": 3}
	return rank[strings.ToLower(have)] >= rank[strings.ToLower(required)]
}
