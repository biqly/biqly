package auth

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

type GDPRExport struct {
	GeneratedAt        time.Time            `json:"generated_at"`
	User               User                 `json:"user"`
	Workspaces         []Workspace          `json:"workspaces"`
	Passkeys           []PasskeyInfo        `json:"passkeys"`
	OAuthAccounts      []GDPROAuthAccount   `json:"oauth_accounts"`
	Sessions           []GDPRSessionRecord  `json:"sessions"`
	DatasourceAccesses []DatasourceAccess   `json:"datasource_accesses"`
	Shares             []ResourceShare      `json:"shares"`
	AuditEntries       []AuditEntry         `json:"audit_entries"`
}

type GDPROAuthAccount struct {
	Provider       string     `json:"provider"`
	ProviderUID    string     `json:"provider_uid"`
	Scope          *string    `json:"scope,omitempty"`
	TokenExpiresAt *time.Time `json:"token_expires_at,omitempty"`
	CreatedAt      time.Time  `json:"created_at"`
}

type GDPRSessionRecord struct {
	ID         string     `json:"id"`
	UserAgent  *string    `json:"user_agent,omitempty"`
	IPAddress  *string    `json:"ip_address,omitempty"`
	CreatedAt  time.Time  `json:"created_at"`
	ExpiresAt  time.Time  `json:"expires_at"`
	RevokedAt  *time.Time `json:"revoked_at,omitempty"`
	TokenHint  string     `json:"refresh_token_hint"`
}

type GDPRExporter struct {
	db       *sql.DB
	userRepo *UserRepository
	ws       *WorkspaceService
	dsAccess *DatasourceAccessService
	sharing  *SharingService
	audit    *AuditService
	webAuthn *WebAuthnService
}

func NewGDPRExporter(
	db *sql.DB,
	userRepo *UserRepository,
	ws *WorkspaceService,
	dsAccess *DatasourceAccessService,
	sharing *SharingService,
	audit *AuditService,
	webAuthn *WebAuthnService,
) *GDPRExporter {
	return &GDPRExporter{
		db:       db,
		userRepo: userRepo,
		ws:       ws,
		dsAccess: dsAccess,
		sharing:  sharing,
		audit:    audit,
		webAuthn: webAuthn,
	}
}

func (e *GDPRExporter) Export(ctx context.Context, userID string) (*GDPRExport, error) {
	user, err := e.userRepo.GetUserByID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("get user: %w", err)
	}
	user.PasswordHash = nil

	out := &GDPRExport{
		GeneratedAt: time.Now().UTC(),
		User:        *user,
	}

	if e.ws != nil {
		if ws, err := e.ws.ListForUser(ctx, userID); err == nil {
			out.Workspaces = ws
		}
	}

	if e.webAuthn != nil {
		if pk, err := e.webAuthn.GetUserPasskeys(ctx, userID); err == nil {
			out.Passkeys = pk
		}
	}

	if accounts, err := e.queryOAuthAccounts(ctx, userID); err == nil {
		out.OAuthAccounts = accounts
	}

	if sessions, err := e.querySessions(ctx, userID); err == nil {
		out.Sessions = sessions
	}

	if e.dsAccess != nil {
		if ds, err := e.dsAccess.ListUserAccess(ctx, userID); err == nil {
			out.DatasourceAccesses = ds
		}
	}

	if e.sharing != nil {
		if shares, err := e.sharing.ListOwned(ctx, userID, "", ""); err == nil {
			out.Shares = shares
		}
	}

	if e.audit != nil {
		if entries, err := e.audit.List(ctx, AuditFilter{UserID: userID, Limit: 1000}); err == nil {
			out.AuditEntries = entries
		}
	}

	return out, nil
}

func (e *GDPRExporter) queryOAuthAccounts(ctx context.Context, userID string) ([]GDPROAuthAccount, error) {
	rows, err := e.db.QueryContext(ctx, `
		SELECT provider, provider_uid, scope, token_expires_at, created_at
		FROM oauth_accounts WHERE user_id = $1 ORDER BY created_at DESC
	`, userID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var out []GDPROAuthAccount
	for rows.Next() {
		var a GDPROAuthAccount
		var scope sql.NullString
		var exp sql.NullTime
		if err := rows.Scan(&a.Provider, &a.ProviderUID, &scope, &exp, &a.CreatedAt); err != nil {
			return nil, err
		}
		if scope.Valid {
			s := scope.String
			a.Scope = &s
		}
		if exp.Valid {
			t := exp.Time
			a.TokenExpiresAt = &t
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

func (e *GDPRExporter) querySessions(ctx context.Context, userID string) ([]GDPRSessionRecord, error) {
	rows, err := e.db.QueryContext(ctx, `
		SELECT id, refresh_token, user_agent, ip_address::text, created_at, expires_at, revoked_at
		FROM sessions WHERE user_id = $1 ORDER BY created_at DESC LIMIT 200
	`, userID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var out []GDPRSessionRecord
	for rows.Next() {
		var s GDPRSessionRecord
		var refresh string
		var ua, ip sql.NullString
		var revoked sql.NullTime
		if err := rows.Scan(&s.ID, &refresh, &ua, &ip, &s.CreatedAt, &s.ExpiresAt, &revoked); err != nil {
			return nil, err
		}
		s.TokenHint = MaskToken(refresh)
		if ua.Valid {
			v := ua.String
			s.UserAgent = &v
		}
		if ip.Valid {
			v := ip.String
			s.IPAddress = &v
		}
		if revoked.Valid {
			t := revoked.Time
			s.RevokedAt = &t
		}
		out = append(out, s)
	}
	return out, rows.Err()
}
