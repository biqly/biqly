package auth

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/biqly/biqly/internal/security"
	"github.com/go-webauthn/webauthn/protocol"
	"github.com/go-webauthn/webauthn/webauthn"
	"github.com/google/uuid"
	"github.com/lib/pq"
	"golang.org/x/oauth2"
)

var (
	ErrUserNotFound         = errors.New("user not found")
	ErrUserAlreadyExists    = errors.New("user already exists")
	ErrOAuthAccountNotFound = errors.New("oauth account not found")
)

type UserRepository struct {
	db  *sql.DB
	enc *security.Encryption
}

func NewUserRepository(db *sql.DB, enc *security.Encryption) *UserRepository {
	return &UserRepository{db: db, enc: enc}
}

func (r *UserRepository) CreateUser(ctx context.Context, email, passwordHash, displayName string) (*User, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()

	var exists bool
	err = tx.QueryRowContext(ctx, "SELECT EXISTS(SELECT 1 FROM users WHERE email = $1)", email).Scan(&exists)
	if err != nil {
		return nil, fmt.Errorf("check user existence: %w", err)
	}
	if exists {
		return nil, ErrUserAlreadyExists
	}

	var user User
	query := `
		INSERT INTO users (email, password_hash, display_name)
		VALUES ($1, $2, $3)
		RETURNING id, email, username, display_name, avatar_url, is_active, email_verified, created_at, updated_at
	`
	var usernameNull, displayNameNull, avatarURLNull sql.NullString
	err = tx.QueryRowContext(ctx, query, email, passwordHash, displayName).Scan(
		&user.ID, &user.Email, &usernameNull, &displayNameNull, &avatarURLNull,
		&user.IsActive, &user.EmailVerified, &user.CreatedAt, &user.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("insert user: %w", err)
	}
	if err := insertPasswordHistory(ctx, tx, user.ID, passwordHash); err != nil {
		return nil, fmt.Errorf("insert password history: %w", err)
	}

	if usernameNull.Valid {
		user.Username = &usernameNull.String
	}
	if displayNameNull.Valid {
		user.DisplayName = &displayNameNull.String
	}
	if avatarURLNull.Valid {
		user.AvatarURL = &avatarURLNull.String
	}

	// Automatically create a personal workspace for the user
	workspaceQuery := `
		INSERT INTO workspaces (name, slug, is_personal, created_by)
		VALUES ($1, $2, TRUE, $3)
		RETURNING id
	`
	workspaceSlug := fmt.Sprintf("%s-personal", user.ID)
	workspaceName := fmt.Sprintf("%s's Workspace", displayName)
	if displayName == "" {
		workspaceName = fmt.Sprintf("%s's Workspace", email)
	}

	var workspaceID string
	err = tx.QueryRowContext(ctx, workspaceQuery, workspaceName, workspaceSlug, user.ID).Scan(&workspaceID)
	if err != nil {
		return nil, fmt.Errorf("create personal workspace: %w", err)
	}

	// Get default viewer role ID (or analyst or user roles)
	var roleID string
	err = tx.QueryRowContext(ctx, "SELECT id FROM roles WHERE name = 'viewer'").Scan(&roleID)
	if err != nil {
		return nil, fmt.Errorf("get default role: %w", err)
	}

	// Assign global role as viewer (default)
	userRoleQuery := `
		INSERT INTO user_roles (user_id, role_id, scope_type, scope_id)
		VALUES ($1, $2, 'global', '00000000-0000-0000-0000-000000000000')
	`
	_, err = tx.ExecContext(ctx, userRoleQuery, user.ID, roleID)
	if err != nil {
		return nil, fmt.Errorf("assign default role: %w", err)
	}

	// Also add member to personal workspace as 'admin' of that workspace
	var adminRoleID string
	err = tx.QueryRowContext(ctx, "SELECT id FROM roles WHERE name = 'admin'").Scan(&adminRoleID)
	if err != nil {
		return nil, fmt.Errorf("get admin role: %w", err)
	}

	workspaceMemberQuery := `
		INSERT INTO workspace_members (workspace_id, user_id, role_id)
		VALUES ($1, $2, $3)
	`
	_, err = tx.ExecContext(ctx, workspaceMemberQuery, workspaceID, user.ID, adminRoleID)
	if err != nil {
		return nil, fmt.Errorf("add member to workspace: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}

	return &user, nil
}

func (r *UserRepository) GetUserByEmail(ctx context.Context, email string) (*User, error) {
	var user User
	var usernameNull, displayNameNull, avatarURLNull, passwordHashNull sql.NullString
	var lastLoginNull sql.NullTime

	query := `
		SELECT id, email, username, display_name, avatar_url, password_hash, is_active, email_verified, created_at, updated_at, last_login_at
		FROM users
		WHERE email = $1
	`
	err := r.db.QueryRowContext(ctx, query, email).Scan(
		&user.ID, &user.Email, &usernameNull, &displayNameNull, &avatarURLNull, &passwordHashNull,
		&user.IsActive, &user.EmailVerified, &user.CreatedAt, &user.UpdatedAt, &lastLoginNull,
	)
	if err == sql.ErrNoRows {
		return nil, ErrUserNotFound
	} else if err != nil {
		return nil, err
	}

	if usernameNull.Valid {
		user.Username = &usernameNull.String
	}
	if displayNameNull.Valid {
		user.DisplayName = &displayNameNull.String
	}
	if avatarURLNull.Valid {
		user.AvatarURL = &avatarURLNull.String
	}
	if passwordHashNull.Valid {
		user.PasswordHash = &passwordHashNull.String
	}
	if lastLoginNull.Valid {
		user.LastLoginAt = &lastLoginNull.Time
	}

	return &user, nil
}

func (r *UserRepository) GetUserByID(ctx context.Context, id string) (*User, error) {
	var user User
	var usernameNull, displayNameNull, avatarURLNull, passwordHashNull sql.NullString
	var lastLoginNull sql.NullTime

	query := `
		SELECT id, email, username, display_name, avatar_url, password_hash, is_active, email_verified, created_at, updated_at, last_login_at
		FROM users
		WHERE id = $1
	`
	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&user.ID, &user.Email, &usernameNull, &displayNameNull, &avatarURLNull, &passwordHashNull,
		&user.IsActive, &user.EmailVerified, &user.CreatedAt, &user.UpdatedAt, &lastLoginNull,
	)
	if err == sql.ErrNoRows {
		return nil, ErrUserNotFound
	} else if err != nil {
		return nil, err
	}

	if usernameNull.Valid {
		user.Username = &usernameNull.String
	}
	if displayNameNull.Valid {
		user.DisplayName = &displayNameNull.String
	}
	if avatarURLNull.Valid {
		user.AvatarURL = &avatarURLNull.String
	}
	if passwordHashNull.Valid {
		user.PasswordHash = &passwordHashNull.String
	}
	if lastLoginNull.Valid {
		user.LastLoginAt = &lastLoginNull.Time
	}

	return &user, nil
}

func (r *UserRepository) ListUsers(ctx context.Context) ([]User, error) {
	query := `
		SELECT id, email, username, display_name, avatar_url, password_hash, is_active, email_verified, created_at, updated_at, last_login_at
		FROM users
		ORDER BY created_at DESC
	`
	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var users []User
	for rows.Next() {
		var user User
		var usernameNull, displayNameNull, avatarURLNull, passwordHashNull sql.NullString
		var lastLoginNull sql.NullTime

		err := rows.Scan(
			&user.ID, &user.Email, &usernameNull, &displayNameNull, &avatarURLNull, &passwordHashNull,
			&user.IsActive, &user.EmailVerified, &user.CreatedAt, &user.UpdatedAt, &lastLoginNull,
		)
		if err != nil {
			return nil, err
		}

		if usernameNull.Valid {
			user.Username = &usernameNull.String
		}
		if displayNameNull.Valid {
			user.DisplayName = &displayNameNull.String
		}
		if avatarURLNull.Valid {
			user.AvatarURL = &avatarURLNull.String
		}
		if passwordHashNull.Valid {
			user.PasswordHash = &passwordHashNull.String
		}
		if lastLoginNull.Valid {
			user.LastLoginAt = &lastLoginNull.Time
		}

		users = append(users, user)
	}
	return users, rows.Err()
}

func (r *UserRepository) UpdateLastLogin(ctx context.Context, userID string) error {
	query := `UPDATE users SET last_login_at = NOW() WHERE id = $1`
	_, err := r.db.ExecContext(ctx, query, userID)
	return err
}

func (r *UserRepository) UpdateUserActiveStatus(ctx context.Context, userID string, isActive bool) error {
	query := `UPDATE users SET is_active = $1, updated_at = NOW() WHERE id = $2`
	_, err := r.db.ExecContext(ctx, query, isActive, userID)
	return err
}

func (r *UserRepository) GetPersonalWorkspaceID(ctx context.Context, userID string) (string, error) {
	var id string
	err := r.db.QueryRowContext(ctx, "SELECT id FROM workspaces WHERE created_by = $1 AND is_personal = TRUE LIMIT 1", userID).Scan(&id)
	return id, err
}

// GetActiveOrPersonalWorkspaceID returns the user's active workspace if still a
// valid membership, otherwise falls back to the personal workspace. The active
// pointer is treated as a hint: if the workspace was deleted or the user lost
// access, we silently degrade to personal so token issuance never fails.
func (r *UserRepository) GetActiveOrPersonalWorkspaceID(ctx context.Context, userID string) (string, error) {
	var active sql.NullString
	if err := r.db.QueryRowContext(ctx, "SELECT active_workspace_id FROM users WHERE id = $1", userID).Scan(&active); err != nil {
		return "", err
	}
	if active.Valid && active.String != "" {
		var ok bool
		if err := r.db.QueryRowContext(ctx,
			"SELECT EXISTS(SELECT 1 FROM workspace_members WHERE workspace_id = $1 AND user_id = $2)",
			active.String, userID).Scan(&ok); err == nil && ok {
			return active.String, nil
		}
	}
	return r.GetPersonalWorkspaceID(ctx, userID)
}

func (r *UserRepository) SetActiveWorkspaceID(ctx context.Context, userID, workspaceID string) error {
	_, err := r.db.ExecContext(ctx,
		"UPDATE users SET active_workspace_id = $1, updated_at = NOW() WHERE id = $2",
		workspaceID, userID)
	return err
}

func (r *UserRepository) encryptToken(token string) (string, error) {
	if token == "" {
		return "", nil
	}
	if r.enc == nil {
		return token, nil
	}
	return r.enc.Encrypt(token)
}

func (r *UserRepository) GetOAuthAccount(ctx context.Context, provider, providerUID string) (string, error) {
	var userID string
	query := `SELECT user_id FROM oauth_accounts WHERE provider = $1 AND provider_uid = $2`
	err := r.db.QueryRowContext(ctx, query, provider, providerUID).Scan(&userID)
	if errors.Is(err, sql.ErrNoRows) {
		return "", ErrOAuthAccountNotFound
	} else if err != nil {
		return "", err
	}
	return userID, nil
}

func (r *UserRepository) LinkOAuthAccount(ctx context.Context, userID, provider, providerUID string, token *oauth2.Token) error {
	encAccess, err := r.encryptToken(token.AccessToken)
	if err != nil {
		return err
	}
	encRefresh, err := r.encryptToken(token.RefreshToken)
	if err != nil {
		return err
	}

	var expiresAt *time.Time
	if !token.Expiry.IsZero() {
		expiresAt = &token.Expiry
	}

	query := `
		INSERT INTO oauth_accounts (user_id, provider, provider_uid, access_token, refresh_token, token_expires_at)
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (provider, provider_uid) DO UPDATE
		SET access_token = EXCLUDED.access_token,
		    refresh_token = EXCLUDED.refresh_token,
		    token_expires_at = EXCLUDED.token_expires_at,
		    updated_at = NOW()
	`
	_, err = r.db.ExecContext(ctx, query, userID, provider, providerUID, encAccess, encRefresh, expiresAt)
	return err
}

func (r *UserRepository) CreateUserWithOAuth(ctx context.Context, email, displayName, provider, providerUID string, token *oauth2.Token) (*User, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()

	var exists bool
	err = tx.QueryRowContext(ctx, "SELECT EXISTS(SELECT 1 FROM users WHERE email = $1)", email).Scan(&exists)
	if err != nil {
		return nil, fmt.Errorf("check user existence: %w", err)
	}

	var user User
	var userID string
	var usernameNull, displayNameNull, avatarURLNull sql.NullString

	if exists {
		queryUser := `SELECT id, email, username, display_name, avatar_url, is_active, email_verified, created_at, updated_at FROM users WHERE email = $1`
		err = tx.QueryRowContext(ctx, queryUser, email).Scan(
			&user.ID, &user.Email, &usernameNull, &displayNameNull, &avatarURLNull,
			&user.IsActive, &user.EmailVerified, &user.CreatedAt, &user.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("get existing user: %w", err)
		}
		userID = user.ID
	} else {
		queryInsert := `
			INSERT INTO users (email, password_hash, display_name, email_verified)
			VALUES ($1, NULL, $2, TRUE)
			RETURNING id, email, username, display_name, avatar_url, is_active, email_verified, created_at, updated_at
		`
		err = tx.QueryRowContext(ctx, queryInsert, email, displayName).Scan(
			&user.ID, &user.Email, &usernameNull, &displayNameNull, &avatarURLNull,
			&user.IsActive, &user.EmailVerified, &user.CreatedAt, &user.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("insert user: %w", err)
		}
		userID = user.ID

		workspaceQuery := `
			INSERT INTO workspaces (name, slug, is_personal, created_by)
			VALUES ($1, $2, TRUE, $3)
			RETURNING id
		`
		workspaceSlug := fmt.Sprintf("%s-personal", userID)
		workspaceName := fmt.Sprintf("%s's Workspace", displayName)
		if displayName == "" {
			workspaceName = fmt.Sprintf("%s's Workspace", email)
		}

		var workspaceID string
		err = tx.QueryRowContext(ctx, workspaceQuery, workspaceName, workspaceSlug, userID).Scan(&workspaceID)
		if err != nil {
			return nil, fmt.Errorf("create personal workspace: %w", err)
		}

		var roleID string
		err = tx.QueryRowContext(ctx, "SELECT id FROM roles WHERE name = 'viewer'").Scan(&roleID)
		if err != nil {
			return nil, fmt.Errorf("get default role: %w", err)
		}

		userRoleQuery := `
			INSERT INTO user_roles (user_id, role_id, scope_type, scope_id)
			VALUES ($1, $2, 'global', '00000000-0000-0000-0000-000000000000')
		`
		_, err = tx.ExecContext(ctx, userRoleQuery, userID, roleID)
		if err != nil {
			return nil, fmt.Errorf("assign default role: %w", err)
		}

		var adminRoleID string
		err = tx.QueryRowContext(ctx, "SELECT id FROM roles WHERE name = 'admin'").Scan(&adminRoleID)
		if err != nil {
			return nil, fmt.Errorf("get admin role: %w", err)
		}

		workspaceMemberQuery := `
			INSERT INTO workspace_members (workspace_id, user_id, role_id)
			VALUES ($1, $2, $3)
		`
		_, err = tx.ExecContext(ctx, workspaceMemberQuery, workspaceID, userID, adminRoleID)
		if err != nil {
			return nil, fmt.Errorf("add member to workspace: %w", err)
		}
	}

	if usernameNull.Valid {
		user.Username = &usernameNull.String
	}
	if displayNameNull.Valid {
		user.DisplayName = &displayNameNull.String
	}
	if avatarURLNull.Valid {
		user.AvatarURL = &avatarURLNull.String
	}

	encAccess, err := r.encryptToken(token.AccessToken)
	if err != nil {
		return nil, err
	}
	encRefresh, err := r.encryptToken(token.RefreshToken)
	if err != nil {
		return nil, err
	}

	var expiresAt *time.Time
	if !token.Expiry.IsZero() {
		expiresAt = &token.Expiry
	}

	oauthQuery := `
		INSERT INTO oauth_accounts (user_id, provider, provider_uid, access_token, refresh_token, token_expires_at)
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (provider, provider_uid) DO UPDATE
		SET access_token = EXCLUDED.access_token,
		    refresh_token = EXCLUDED.refresh_token,
		    token_expires_at = EXCLUDED.token_expires_at,
		    updated_at = NOW()
	`
	_, err = tx.ExecContext(ctx, oauthQuery, userID, provider, providerUID, encAccess, encRefresh, expiresAt)
	if err != nil {
		return nil, fmt.Errorf("link oauth account: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}

	return &user, nil
}

func (r *UserRepository) UnlinkOAuthAccount(ctx context.Context, userID, provider string) error {
	query := `DELETE FROM oauth_accounts WHERE user_id = $1 AND provider = $2`
	_, err := r.db.ExecContext(ctx, query, userID, provider)
	return err
}

func (r *UserRepository) SaveWebAuthnChallenge(ctx context.Context, challenge []byte, userID *string, expiresAt time.Time) error {
	query := `
		INSERT INTO webauthn_challenges (challenge, user_id, expires_at)
		VALUES ($1, $2, $3)
	`
	var uid sql.NullString
	if userID != nil {
		uid = sql.NullString{String: *userID, Valid: true}
	}
	_, err := r.db.ExecContext(ctx, query, challenge, uid, expiresAt)
	return err
}

func (r *UserRepository) GetWebAuthnChallenge(ctx context.Context, challenge []byte) (*string, error) {
	var userID sql.NullString
	var expiresAt time.Time

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()

	querySelect := `
		SELECT user_id, expires_at
		FROM webauthn_challenges
		WHERE challenge = $1
	`
	err = tx.QueryRowContext(ctx, querySelect, challenge).Scan(&userID, &expiresAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	} else if err != nil {
		return nil, err
	}

	queryDelete := `
		DELETE FROM webauthn_challenges
		WHERE challenge = $1
	`
	_, err = tx.ExecContext(ctx, queryDelete, challenge)
	if err != nil {
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}

	if time.Now().After(expiresAt) {
		return nil, nil
	}

	var res *string
	if userID.Valid {
		s := userID.String
		res = &s
	}
	return res, nil
}

func (r *UserRepository) GetPasskeysByUserID(ctx context.Context, userID string) ([]webauthn.Credential, error) {
	query := `
		SELECT credential_id, public_key, attestation_type, transport, sign_count, aaguid
		FROM passkeys
		WHERE user_id = $1
	`
	rows, err := r.db.QueryContext(ctx, query, userID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var credentials []webauthn.Credential
	for rows.Next() {
		var credID, pubKey []byte
		var attType string
		var transports []string
		var signCount int64
		var aaguidStr sql.NullString

		err = rows.Scan(&credID, &pubKey, &attType, pq.Array(&transports), &signCount, &aaguidStr)
		if err != nil {
			return nil, err
		}

		var aaguid []byte
		if aaguidStr.Valid && aaguidStr.String != "" {
			u, err := uuid.Parse(aaguidStr.String)
			if err == nil {
				aaguid = u[:]
			}
		}

		webauthnTransports := make([]protocol.AuthenticatorTransport, len(transports))
		for i, t := range transports {
			webauthnTransports[i] = protocol.AuthenticatorTransport(t)
		}

		cred := webauthn.Credential{
			ID:              credID,
			PublicKey:       pubKey,
			AttestationType: attType,
			Transport:       webauthnTransports,
			Authenticator: webauthn.Authenticator{
				AAGUID: aaguid,
				//nolint:gosec
				SignCount: uint32(signCount),
			},
		}
		credentials = append(credentials, cred)
	}

	if err = rows.Err(); err != nil {
		return nil, err
	}

	return credentials, nil
}

func (r *UserRepository) SavePasskey(ctx context.Context, userID string, cred *webauthn.Credential, name string) error {
	transports := make([]string, len(cred.Transport))
	for i, t := range cred.Transport {
		transports[i] = string(t)
	}

	var aaguidStr *string
	if len(cred.Authenticator.AAGUID) > 0 {
		u, err := uuid.FromBytes(cred.Authenticator.AAGUID)
		if err == nil {
			s := u.String()
			aaguidStr = &s
		}
	}

	query := `
		INSERT INTO passkeys (user_id, credential_id, public_key, attestation_type, transport, sign_count, name, aaguid)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`
	_, err := r.db.ExecContext(ctx, query,
		userID,
		cred.ID,
		cred.PublicKey,
		cred.AttestationType,
		pq.Array(transports),
		int64(cred.Authenticator.SignCount),
		name,
		aaguidStr,
	)
	return err
}

func (r *UserRepository) UpdatePasskeySignCount(ctx context.Context, credentialID []byte, signCount uint32) error {
	query := `
		UPDATE passkeys
		SET sign_count = $1, last_used_at = NOW()
		WHERE credential_id = $2
	`
	_, err := r.db.ExecContext(ctx, query, int64(signCount), credentialID)
	return err
}

func (r *UserRepository) DeletePasskey(ctx context.Context, userID string, passkeyID string) error {
	query := `
		DELETE FROM passkeys
		WHERE user_id = $1 AND id = $2
	`
	_, err := r.db.ExecContext(ctx, query, userID, passkeyID)
	return err
}

func (r *UserRepository) GetUserPasskeys(ctx context.Context, userID string) ([]PasskeyInfo, error) {
	query := `
		SELECT id, name, created_at, last_used_at
		FROM passkeys
		WHERE user_id = $1
		ORDER BY created_at DESC
	`
	rows, err := r.db.QueryContext(ctx, query, userID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var passkeys []PasskeyInfo
	for rows.Next() {
		var p PasskeyInfo
		var nameNull sql.NullString
		var lastUsedNull sql.NullTime

		err = rows.Scan(&p.ID, &nameNull, &p.CreatedAt, &lastUsedNull)
		if err != nil {
			return nil, err
		}

		if nameNull.Valid {
			p.Name = nameNull.String
		}
		if lastUsedNull.Valid {
			p.LastUsedAt = &lastUsedNull.Time
		}

		passkeys = append(passkeys, p)
	}

	if err = rows.Err(); err != nil {
		return nil, err
	}

	return passkeys, nil
}

func (r *UserRepository) GetUserByEmailOrUsername(ctx context.Context, loginStr string) (*User, error) {
	var user User
	var usernameNull, displayNameNull, avatarURLNull, passwordHashNull sql.NullString
	var lastLoginNull sql.NullTime

	query := `
		SELECT id, email, username, display_name, avatar_url, password_hash, is_active, email_verified, created_at, updated_at, last_login_at
		FROM users
		WHERE email = $1 OR username = $1
	`
	err := r.db.QueryRowContext(ctx, query, loginStr).Scan(
		&user.ID, &user.Email, &usernameNull, &displayNameNull, &avatarURLNull, &passwordHashNull,
		&user.IsActive, &user.EmailVerified, &user.CreatedAt, &user.UpdatedAt, &lastLoginNull,
	)
	if err == sql.ErrNoRows {
		return nil, ErrUserNotFound
	} else if err != nil {
		return nil, err
	}

	if usernameNull.Valid {
		user.Username = &usernameNull.String
	}
	if displayNameNull.Valid {
		user.DisplayName = &displayNameNull.String
	}
	if avatarURLNull.Valid {
		user.AvatarURL = &avatarURLNull.String
	}
	if passwordHashNull.Valid {
		user.PasswordHash = &passwordHashNull.String
	}
	if lastLoginNull.Valid {
		user.LastLoginAt = &lastLoginNull.Time
	}

	return &user, nil
}

func (r *UserRepository) CreateEmailVerificationToken(ctx context.Context, userID, token string, expiresAt time.Time) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	_, err = tx.ExecContext(ctx, "DELETE FROM email_verification_tokens WHERE user_id = $1", userID)
	if err != nil {
		return err
	}

	query := `
		INSERT INTO email_verification_tokens (user_id, token, expires_at)
		VALUES ($1, $2, $3)
	`
	_, err = tx.ExecContext(ctx, query, userID, token, expiresAt)
	if err != nil {
		return err
	}

	return tx.Commit()
}

func (r *UserRepository) VerifyEmailToken(ctx context.Context, token string) (string, error) {
	var userID string
	var expiresAt time.Time
	var usedAt sql.NullTime

	query := `
		SELECT user_id, expires_at, used_at
		FROM email_verification_tokens
		WHERE token = $1
	`
	err := r.db.QueryRowContext(ctx, query, token).Scan(&userID, &expiresAt, &usedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return "", errors.New("invalid verification token")
	} else if err != nil {
		return "", err
	}

	if usedAt.Valid {
		return "", errors.New("verification token already used")
	}

	if time.Now().After(expiresAt) {
		return "", errors.New("verification token expired")
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return "", err
	}
	defer func() { _ = tx.Rollback() }()

	_, err = tx.ExecContext(ctx, "UPDATE email_verification_tokens SET used_at = NOW() WHERE token = $1", token)
	if err != nil {
		return "", err
	}

	_, err = tx.ExecContext(ctx, "UPDATE users SET email_verified = TRUE WHERE id = $1", userID)
	if err != nil {
		return "", err
	}

	if err := tx.Commit(); err != nil {
		return "", err
	}

	return userID, nil
}

func (r *UserRepository) CreatePasswordResetToken(ctx context.Context, userID, token string, expiresAt time.Time) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	_, err = tx.ExecContext(ctx, "DELETE FROM password_reset_tokens WHERE user_id = $1", userID)
	if err != nil {
		return err
	}

	query := `
		INSERT INTO password_reset_tokens (user_id, token, expires_at)
		VALUES ($1, $2, $3)
	`
	_, err = tx.ExecContext(ctx, query, userID, token, expiresAt)
	if err != nil {
		return err
	}

	return tx.Commit()
}

func (r *UserRepository) VerifyPasswordResetToken(ctx context.Context, token string) (string, error) {
	var userID string
	var expiresAt time.Time
	var usedAt sql.NullTime

	query := `
		SELECT user_id, expires_at, used_at
		FROM password_reset_tokens
		WHERE token = $1
	`
	err := r.db.QueryRowContext(ctx, query, token).Scan(&userID, &expiresAt, &usedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return "", errors.New("invalid password reset token")
	} else if err != nil {
		return "", err
	}

	if usedAt.Valid {
		return "", errors.New("password reset token already used")
	}

	if time.Now().After(expiresAt) {
		return "", errors.New("password reset token expired")
	}

	return userID, nil
}

func (r *UserRepository) UpdateUserPassword(ctx context.Context, userID, newPasswordHash string) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	query := `UPDATE users SET password_hash = $1, updated_at = NOW() WHERE id = $2`
	_, err = tx.ExecContext(ctx, query, newPasswordHash, userID)
	if err != nil {
		return err
	}
	if err := insertPasswordHistory(ctx, tx, userID, newPasswordHash); err != nil {
		return err
	}
	return tx.Commit()
}

func (r *UserRepository) MarkPasswordResetTokenUsed(ctx context.Context, token string) error {
	query := `UPDATE password_reset_tokens SET used_at = NOW() WHERE token = $1`
	_, err := r.db.ExecContext(ctx, query, token)
	return err
}

func (r *UserRepository) CreateEmailChangeRequest(
	ctx context.Context,
	userID, oldEmail, newEmail, oldToken, newToken string,
	notBefore, expiresAt time.Time,
) (*EmailChangeRequest, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()

	_, err = tx.ExecContext(ctx, "DELETE FROM email_change_requests WHERE user_id = $1 AND completed_at IS NULL", userID)
	if err != nil {
		return nil, err
	}

	query := `
		INSERT INTO email_change_requests (
			user_id, old_email, new_email, old_email_token, new_email_token, not_before, expires_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id, user_id, old_email, new_email, old_email_token, new_email_token,
		          old_email_confirmed_at, new_email_confirmed_at, requested_at,
		          not_before, expires_at, completed_at
	`
	req, err := scanEmailChangeRequest(tx.QueryRowContext(ctx, query, userID, oldEmail, newEmail, oldToken, newToken, notBefore, expiresAt))
	if err != nil {
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return req, nil
}

func (r *UserRepository) ConfirmEmailChangeToken(ctx context.Context, token string, now time.Time) (*EmailChangeRequest, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()

	query := `
		SELECT id, user_id, old_email, new_email, old_email_token, new_email_token,
		       old_email_confirmed_at, new_email_confirmed_at, requested_at,
		       not_before, expires_at, completed_at
		FROM email_change_requests
		WHERE old_email_token = $1 OR new_email_token = $1
		FOR UPDATE
	`
	req, err := scanEmailChangeRequest(tx.QueryRowContext(ctx, query, token))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, errors.New("invalid email change token")
	} else if err != nil {
		return nil, err
	}

	if req.CompletedAt != nil {
		return nil, errors.New("email change already completed")
	}
	if now.After(req.ExpiresAt) {
		return nil, errors.New("email change token expired")
	}

	switch token {
	case req.OldEmailToken:
		_, err = tx.ExecContext(ctx, "UPDATE email_change_requests SET old_email_confirmed_at = COALESCE(old_email_confirmed_at, $1) WHERE id = $2", now, req.ID)
		if err != nil {
			return nil, err
		}
		if req.OldEmailConfirmedAt == nil {
			req.OldEmailConfirmedAt = &now
		}
	case req.NewEmailToken:
		_, err = tx.ExecContext(ctx, "UPDATE email_change_requests SET new_email_confirmed_at = COALESCE(new_email_confirmed_at, $1) WHERE id = $2", now, req.ID)
		if err != nil {
			return nil, err
		}
		if req.NewEmailConfirmedAt == nil {
			req.NewEmailConfirmedAt = &now
		}
	default:
		return nil, errors.New("invalid email change token")
	}

	if req.OldEmailConfirmedAt != nil && req.NewEmailConfirmedAt != nil && !now.Before(req.NotBefore) {
		_, err = tx.ExecContext(ctx, "UPDATE users SET email = $1, email_verified = TRUE, updated_at = NOW() WHERE id = $2", req.NewEmail, req.UserID)
		if err != nil {
			return nil, err
		}
		_, err = tx.ExecContext(ctx, "UPDATE email_change_requests SET completed_at = $1 WHERE id = $2", now, req.ID)
		if err != nil {
			return nil, err
		}
		req.CompletedAt = &now
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return req, nil
}

type scanner interface {
	Scan(dest ...any) error
}

func scanEmailChangeRequest(s scanner) (*EmailChangeRequest, error) {
	var req EmailChangeRequest
	var oldConfirmed, newConfirmed, completed sql.NullTime
	err := s.Scan(
		&req.ID,
		&req.UserID,
		&req.OldEmail,
		&req.NewEmail,
		&req.OldEmailToken,
		&req.NewEmailToken,
		&oldConfirmed,
		&newConfirmed,
		&req.RequestedAt,
		&req.NotBefore,
		&req.ExpiresAt,
		&completed,
	)
	if err != nil {
		return nil, err
	}
	if oldConfirmed.Valid {
		req.OldEmailConfirmedAt = &oldConfirmed.Time
	}
	if newConfirmed.Valid {
		req.NewEmailConfirmedAt = &newConfirmed.Time
	}
	if completed.Valid {
		req.CompletedAt = &completed.Time
	}
	return &req, nil
}

func (r *UserRepository) PasswordWasUsed(ctx context.Context, userID, password string, limit int) (bool, error) {
	if limit <= 0 {
		return false, nil
	}
	rows, err := r.db.QueryContext(ctx, `
		SELECT password_hash
		FROM password_history
		WHERE user_id = $1
		ORDER BY created_at DESC
		LIMIT $2
	`, userID, limit)
	if err != nil {
		return false, err
	}
	defer func() { _ = rows.Close() }()

	for rows.Next() {
		var hash string
		if err := rows.Scan(&hash); err != nil {
			return false, err
		}
		if VerifyPassword(password, hash) {
			return true, nil
		}
	}
	if err := rows.Err(); err != nil {
		return false, err
	}
	return false, nil
}

type txExecutor interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
}

func insertPasswordHistory(ctx context.Context, tx txExecutor, userID, passwordHash string) error {
	if passwordHash == "" {
		return nil
	}
	_, err := tx.ExecContext(ctx, `
		INSERT INTO password_history (user_id, password_hash)
		VALUES ($1, $2)
	`, userID, passwordHash)
	return err
}
