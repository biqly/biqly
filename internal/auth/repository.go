package auth

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	platformdb "github.com/biqly/biqly/internal/platform/db"
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
	ErrEmailAlreadyVerified = errors.New("email is already verified")
)

type UserRepository struct {
	db  *sql.DB
	enc *security.Encryption
}

func NewUserRepository(db *sql.DB, enc *security.Encryption) *UserRepository {
	return &UserRepository{db: db, enc: enc}
}

func (r *UserRepository) CreateUser(ctx context.Context, email, passwordHash, displayName string) (*User, error) {
	var user User
	err := platformdb.RunInTx(ctx, r.db, func(tx *sql.Tx) error {
		var exists bool
		if err := tx.QueryRowContext(ctx, "SELECT EXISTS(SELECT 1 FROM users WHERE email = $1)", email).Scan(&exists); err != nil {
			return fmt.Errorf("check user existence: %w", err)
		}
		if exists {
			return ErrUserAlreadyExists
		}

		query := `
			INSERT INTO users (email, password_hash, display_name, password_changed_at)
			VALUES ($1, $2, $3, NOW())
			RETURNING id, email, username, display_name, avatar_url, is_active, email_verified, created_at, updated_at
		`
		var usernameNull, displayNameNull, avatarURLNull sql.NullString
		if err := tx.QueryRowContext(ctx, query, email, passwordHash, displayName).Scan(
			&user.ID, &user.Email, &usernameNull, &displayNameNull, &avatarURLNull,
			&user.IsActive, &user.EmailVerified, &user.CreatedAt, &user.UpdatedAt,
		); err != nil {
			return fmt.Errorf("insert user: %w", err)
		}
		if err := insertPasswordHistory(ctx, tx, user.ID, passwordHash); err != nil {
			return fmt.Errorf("insert password history: %w", err)
		}

		user.Username = platformdb.StringPtrFromNull(usernameNull)
		user.DisplayName = platformdb.StringPtrFromNull(displayNameNull)
		user.AvatarURL = platformdb.StringPtrFromNull(avatarURLNull)

		// Automatically create a personal workspace for the user and assign roles.
		return bootstrapUserWorkspace(ctx, tx, user.ID, displayName, email)
	})
	if err != nil {
		return nil, err
	}

	return &user, nil
}

// CreateDirectoryUser just-in-time provisions a passwordless user authenticated
// by an external directory (LDAP). The email is marked verified (the directory
// is the source of truth) and the user gets a personal workspace + default role,
// mirroring OAuth provisioning. Returns ErrUserAlreadyExists if the email exists.
func (r *UserRepository) CreateDirectoryUser(ctx context.Context, email, displayName string) (*User, error) {
	var user User
	err := platformdb.RunInTx(ctx, r.db, func(tx *sql.Tx) error {
		var exists bool
		if err := tx.QueryRowContext(ctx, "SELECT EXISTS(SELECT 1 FROM users WHERE email = $1)", email).Scan(&exists); err != nil {
			return fmt.Errorf("check user existence: %w", err)
		}
		if exists {
			return ErrUserAlreadyExists
		}
		var usernameNull, displayNameNull, avatarURLNull sql.NullString
		if err := tx.QueryRowContext(ctx, `
			INSERT INTO users (email, password_hash, display_name, email_verified)
			VALUES ($1, NULL, $2, TRUE)
			RETURNING id, email, username, display_name, avatar_url, is_active, email_verified, created_at, updated_at
		`, email, displayName).Scan(
			&user.ID, &user.Email, &usernameNull, &displayNameNull, &avatarURLNull,
			&user.IsActive, &user.EmailVerified, &user.CreatedAt, &user.UpdatedAt,
		); err != nil {
			return fmt.Errorf("insert directory user: %w", err)
		}
		user.Username = platformdb.StringPtrFromNull(usernameNull)
		user.DisplayName = platformdb.StringPtrFromNull(displayNameNull)
		user.AvatarURL = platformdb.StringPtrFromNull(avatarURLNull)
		return bootstrapUserWorkspace(ctx, tx, user.ID, displayName, email)
	})
	if err != nil {
		return nil, err
	}
	return &user, nil
}

// bootstrapUserWorkspace provisions a newly created user: it creates a personal
// workspace, assigns the global viewer role, and adds the user as admin of their
// personal workspace. It must run inside the same transaction as user creation.
func bootstrapUserWorkspace(ctx context.Context, tx *sql.Tx, userID, displayName, email string) error {
	workspaceQuery := `
		INSERT INTO workspaces (name, slug, is_personal, created_by)
		VALUES ($1, $2, TRUE, $3)
		RETURNING id
	`
	workspaceSlug := userID + "-personal"
	workspaceName := displayName + "'s Workspace"
	if displayName == "" {
		workspaceName = email + "'s Workspace"
	}

	var workspaceID string
	if err := tx.QueryRowContext(ctx, workspaceQuery, workspaceName, workspaceSlug, userID).Scan(&workspaceID); err != nil {
		return fmt.Errorf("create personal workspace: %w", err)
	}

	// Get default viewer role ID and assign it as the global role.
	var roleID string
	if err := tx.QueryRowContext(ctx, "SELECT id FROM roles WHERE name = 'viewer'").Scan(&roleID); err != nil {
		return fmt.Errorf("get default role: %w", err)
	}

	userRoleQuery := `
		INSERT INTO user_roles (user_id, role_id, scope_type, scope_id)
		VALUES ($1, $2, 'global', '00000000-0000-0000-0000-000000000000')
	`
	if _, err := tx.ExecContext(ctx, userRoleQuery, userID, roleID); err != nil {
		return fmt.Errorf("assign default role: %w", err)
	}

	// Add the user as 'admin' of their personal workspace.
	var adminRoleID string
	if err := tx.QueryRowContext(ctx, "SELECT id FROM roles WHERE name = 'admin'").Scan(&adminRoleID); err != nil {
		return fmt.Errorf("get admin role: %w", err)
	}

	workspaceMemberQuery := `
		INSERT INTO workspace_members (workspace_id, user_id, role_id)
		VALUES ($1, $2, $3)
	`
	if _, err := tx.ExecContext(ctx, workspaceMemberQuery, workspaceID, userID, adminRoleID); err != nil {
		return fmt.Errorf("add member to workspace: %w", err)
	}

	return nil
}

// scanUser scans a full user row (11 columns) into a *User, converting
// nullable SQL columns into pointer fields. Callers map sql.ErrNoRows to
// ErrUserNotFound as appropriate.
func scanUser(s platformdb.Scanner) (*User, error) {
	var user User
	var usernameNull, displayNameNull, avatarURLNull, passwordHashNull sql.NullString
	var lastLoginNull sql.NullTime

	if err := s.Scan(
		&user.ID, &user.Email, &usernameNull, &displayNameNull, &avatarURLNull, &passwordHashNull,
		&user.IsActive, &user.EmailVerified, &user.CreatedAt, &user.UpdatedAt, &lastLoginNull,
	); err != nil {
		return nil, err
	}

	user.Username = platformdb.StringPtrFromNull(usernameNull)
	user.DisplayName = platformdb.StringPtrFromNull(displayNameNull)
	user.AvatarURL = platformdb.StringPtrFromNull(avatarURLNull)
	user.PasswordHash = platformdb.StringPtrFromNull(passwordHashNull)
	user.LastLoginAt = platformdb.TimePtrFromNull(lastLoginNull)

	return &user, nil
}

func scanUserPublic(s platformdb.Scanner) (*User, error) {
	var user User
	var usernameNull, displayNameNull, avatarURLNull sql.NullString
	var lastLoginNull sql.NullTime

	if err := s.Scan(
		&user.ID, &user.Email, &usernameNull, &displayNameNull, &avatarURLNull,
		&user.IsActive, &user.EmailVerified, &user.CreatedAt, &user.UpdatedAt, &lastLoginNull,
	); err != nil {
		return nil, err
	}

	user.Username = platformdb.StringPtrFromNull(usernameNull)
	user.DisplayName = platformdb.StringPtrFromNull(displayNameNull)
	user.AvatarURL = platformdb.StringPtrFromNull(avatarURLNull)
	user.LastLoginAt = platformdb.TimePtrFromNull(lastLoginNull)

	return &user, nil
}

func (r *UserRepository) GetUserByEmail(ctx context.Context, email string) (*User, error) {
	query := `
		SELECT id, email, username, display_name, avatar_url, password_hash, is_active, email_verified, created_at, updated_at, last_login_at
		FROM users
		WHERE email = $1
	`
	user, err := scanUser(r.db.QueryRowContext(ctx, query, email))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrUserNotFound
	} else if err != nil {
		return nil, err
	}
	return user, nil
}

func (r *UserRepository) GetUserByID(ctx context.Context, id string) (*User, error) {
	query := `
		SELECT id, email, username, display_name, avatar_url, password_hash, is_active, email_verified, created_at, updated_at, last_login_at
		FROM users
		WHERE id = $1
	`
	user, err := scanUser(r.db.QueryRowContext(ctx, query, id))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrUserNotFound
	} else if err != nil {
		return nil, err
	}
	return user, nil
}

func (r *UserRepository) ListUsers(ctx context.Context) ([]User, error) {
	query := `
		SELECT id, email, username, display_name, avatar_url, is_active, email_verified, created_at, updated_at, last_login_at
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
		user, err := scanUserPublic(rows)
		if err != nil {
			return nil, err
		}
		users = append(users, *user)
	}
	return users, rows.Err()
}

func (r *UserRepository) ListUsersForAdmin(ctx context.Context) ([]AdminUserListRow, error) {
	query := `
		SELECT
			u.id, u.email, u.username, u.display_name, u.avatar_url, u.password_hash,
			u.is_active, u.email_verified, u.created_at, u.updated_at, u.last_login_at,
			COALESCE(m.enabled, FALSE) AS mfa_enabled,
			(m.user_id IS NOT NULL AND NOT COALESCE(m.enabled, FALSE)) AS mfa_pending,
			COALESCE(pk.passkey_count, 0) AS passkey_count
		FROM users u
		LEFT JOIN user_mfa m ON m.user_id = u.id
		LEFT JOIN (
			SELECT user_id, COUNT(*)::int AS passkey_count
			FROM passkeys
			GROUP BY user_id
		) pk ON pk.user_id = u.id
		ORDER BY u.created_at DESC
	`
	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var out []AdminUserListRow
	for rows.Next() {
		var row AdminUserListRow
		var usernameNull, displayNameNull, avatarURLNull, passwordHashNull sql.NullString
		var lastLoginNull sql.NullTime
		if err := rows.Scan(
			&row.ID, &row.Email, &usernameNull, &displayNameNull, &avatarURLNull, &passwordHashNull,
			&row.IsActive, &row.EmailVerified, &row.CreatedAt, &row.UpdatedAt, &lastLoginNull,
			&row.MFAEnabled, &row.MFAPending, &row.PasskeyCount,
		); err != nil {
			return nil, err
		}
		row.Username = platformdb.StringPtrFromNull(usernameNull)
		row.DisplayName = platformdb.StringPtrFromNull(displayNameNull)
		row.AvatarURL = platformdb.StringPtrFromNull(avatarURLNull)
		row.PasswordHash = platformdb.StringPtrFromNull(passwordHashNull)
		row.LastLoginAt = platformdb.TimePtrFromNull(lastLoginNull)
		out = append(out, row)
	}
	return out, rows.Err()
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

func (r *UserRepository) UpdateUserDisplayName(ctx context.Context, userID, displayName string) error {
	var name any
	if displayName != "" {
		name = displayName
	}
	res, err := r.db.ExecContext(ctx,
		`UPDATE users SET display_name = $1, updated_at = NOW() WHERE id = $2`,
		name, userID,
	)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrUserNotFound
	}
	return nil
}

func (r *UserRepository) UpdateUserAvatarURL(ctx context.Context, userID string, avatarURL *string) error {
	var val any
	if avatarURL != nil && *avatarURL != "" {
		val = *avatarURL
	}
	res, err := r.db.ExecContext(ctx,
		`UPDATE users SET avatar_url = $1, updated_at = NOW() WHERE id = $2`,
		val, userID,
	)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrUserNotFound
	}
	return nil
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
		expiresAt = new(token.Expiry)
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
	var user User
	err := platformdb.RunInTx(ctx, r.db, func(tx *sql.Tx) error {
		var exists bool
		if err := tx.QueryRowContext(ctx, "SELECT EXISTS(SELECT 1 FROM users WHERE email = $1)", email).Scan(&exists); err != nil {
			return fmt.Errorf("check user existence: %w", err)
		}

		var userID string
		var usernameNull, displayNameNull, avatarURLNull sql.NullString

		if exists {
			queryUser := `SELECT id, email, username, display_name, avatar_url, is_active, email_verified, created_at, updated_at FROM users WHERE email = $1`
			if err := tx.QueryRowContext(ctx, queryUser, email).Scan(
				&user.ID, &user.Email, &usernameNull, &displayNameNull, &avatarURLNull,
				&user.IsActive, &user.EmailVerified, &user.CreatedAt, &user.UpdatedAt,
			); err != nil {
				return fmt.Errorf("get existing user: %w", err)
			}
			userID = user.ID
		} else {
			queryInsert := `
				INSERT INTO users (email, password_hash, display_name, email_verified)
				VALUES ($1, NULL, $2, TRUE)
				RETURNING id, email, username, display_name, avatar_url, is_active, email_verified, created_at, updated_at
			`
			if err := tx.QueryRowContext(ctx, queryInsert, email, displayName).Scan(
				&user.ID, &user.Email, &usernameNull, &displayNameNull, &avatarURLNull,
				&user.IsActive, &user.EmailVerified, &user.CreatedAt, &user.UpdatedAt,
			); err != nil {
				return fmt.Errorf("insert user: %w", err)
			}
			userID = user.ID

			if err := bootstrapUserWorkspace(ctx, tx, userID, displayName, email); err != nil {
				return err
			}
		}

		user.Username = platformdb.StringPtrFromNull(usernameNull)
		user.DisplayName = platformdb.StringPtrFromNull(displayNameNull)
		user.AvatarURL = platformdb.StringPtrFromNull(avatarURLNull)

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
			expiresAt = new(token.Expiry)
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
		if _, err := tx.ExecContext(ctx, oauthQuery, userID, provider, providerUID, encAccess, encRefresh, expiresAt); err != nil {
			return fmt.Errorf("link oauth account: %w", err)
		}
		return nil
	})
	if err != nil {
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
	found := false

	err := platformdb.RunInTx(ctx, r.db, func(tx *sql.Tx) error {
		querySelect := `
			SELECT user_id, expires_at
			FROM webauthn_challenges
			WHERE challenge = $1
		`
		err := tx.QueryRowContext(ctx, querySelect, challenge).Scan(&userID, &expiresAt)
		if errors.Is(err, sql.ErrNoRows) {
			return nil
		} else if err != nil {
			return err
		}
		found = true

		queryDelete := `
			DELETE FROM webauthn_challenges
			WHERE challenge = $1
		`
		_, err = tx.ExecContext(ctx, queryDelete, challenge)
		return err
	})
	if err != nil {
		return nil, err
	}

	if !found || time.Now().After(expiresAt) {
		return nil, nil //nolint:nilnil // missing or expired token is not an error
	}

	return platformdb.StringPtrFromNull(userID), nil
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

	if err := rows.Err(); err != nil {
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
			aaguidStr = new(u.String())
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

func (r *UserRepository) UpdatePasskeyName(ctx context.Context, userID string, passkeyID string, name string) error {
	query := `
		UPDATE passkeys
		SET name = $1
		WHERE user_id = $2 AND id = $3
	`
	_, err := r.db.ExecContext(ctx, query, name, userID, passkeyID)
	return err
}

func (r *UserRepository) GetUserIDByCredentialID(ctx context.Context, credentialID []byte) (string, error) {
	var userID string
	err := r.db.QueryRowContext(ctx, "SELECT user_id FROM passkeys WHERE credential_id = $1", credentialID).Scan(&userID)
	if err != nil {
		return "", err
	}
	return userID, nil
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
		p.LastUsedAt = platformdb.TimePtrFromNull(lastUsedNull)

		passkeys = append(passkeys, p)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return passkeys, nil
}

func (r *UserRepository) GetUserByEmailOrUsername(ctx context.Context, loginStr string) (*User, error) {
	query := `
		SELECT id, email, username, display_name, avatar_url, password_hash, is_active, email_verified, created_at, updated_at, last_login_at
		FROM users
		WHERE email = $1 OR username = $1
	`
	user, err := scanUser(r.db.QueryRowContext(ctx, query, loginStr))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrUserNotFound
	} else if err != nil {
		return nil, err
	}
	return user, nil
}

func (r *UserRepository) CreateEmailVerificationToken(ctx context.Context, userID, token string, expiresAt time.Time) error {
	return platformdb.RunInTx(ctx, r.db, func(tx *sql.Tx) error {
		if _, err := tx.ExecContext(ctx, "DELETE FROM email_verification_tokens WHERE user_id = $1", userID); err != nil {
			return err
		}

		query := `
			INSERT INTO email_verification_tokens (user_id, token, expires_at)
			VALUES ($1, $2, $3)
		`
		_, err := tx.ExecContext(ctx, query, userID, token, expiresAt)
		return err
	})
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

	err = platformdb.RunInTx(ctx, r.db, func(tx *sql.Tx) error {
		if _, err := tx.ExecContext(ctx, "UPDATE email_verification_tokens SET used_at = NOW() WHERE token = $1", token); err != nil {
			return err
		}
		_, err := tx.ExecContext(ctx, "UPDATE users SET email_verified = TRUE WHERE id = $1", userID)
		return err
	})
	if err != nil {
		return "", err
	}

	return userID, nil
}

func (r *UserRepository) CreatePasswordResetToken(ctx context.Context, userID, token string, expiresAt time.Time) error {
	return platformdb.RunInTx(ctx, r.db, func(tx *sql.Tx) error {
		if _, err := tx.ExecContext(ctx, "DELETE FROM password_reset_tokens WHERE user_id = $1", userID); err != nil {
			return err
		}

		query := `
			INSERT INTO password_reset_tokens (user_id, token, expires_at)
			VALUES ($1, $2, $3)
		`
		_, err := tx.ExecContext(ctx, query, userID, token, expiresAt)
		return err
	})
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
	return platformdb.RunInTx(ctx, r.db, func(tx *sql.Tx) error {
		query := `UPDATE users SET password_hash = $1, password_changed_at = NOW(), updated_at = NOW() WHERE id = $2`
		if _, err := tx.ExecContext(ctx, query, newPasswordHash, userID); err != nil {
			return err
		}
		return insertPasswordHistory(ctx, tx, userID, newPasswordHash)
	})
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
	var req *EmailChangeRequest
	err := platformdb.RunInTx(ctx, r.db, func(tx *sql.Tx) error {
		if _, err := tx.ExecContext(ctx, "DELETE FROM email_change_requests WHERE user_id = $1 AND completed_at IS NULL", userID); err != nil {
			return err
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
		var err error
		req, err = scanEmailChangeRequest(tx.QueryRowContext(ctx, query, userID, oldEmail, newEmail, oldToken, newToken, notBefore, expiresAt))
		return err
	})
	if err != nil {
		return nil, err
	}
	return req, nil
}

//nolint:gocognit
func (r *UserRepository) ConfirmEmailChangeToken(ctx context.Context, token string, now time.Time) (*EmailChangeRequest, error) {
	var req *EmailChangeRequest
	err := platformdb.RunInTx(ctx, r.db, func(tx *sql.Tx) error {
		query := `
			SELECT id, user_id, old_email, new_email, old_email_token, new_email_token,
			       old_email_confirmed_at, new_email_confirmed_at, requested_at,
			       not_before, expires_at, completed_at
			FROM email_change_requests
			WHERE old_email_token = $1 OR new_email_token = $1
			FOR UPDATE
		`
		var err error
		req, err = scanEmailChangeRequest(tx.QueryRowContext(ctx, query, token))
		if errors.Is(err, sql.ErrNoRows) {
			return errors.New("invalid email change token")
		} else if err != nil {
			return err
		}

		if req.CompletedAt != nil {
			return errors.New("email change already completed")
		}
		if now.After(req.ExpiresAt) {
			return errors.New("email change token expired")
		}

		switch token {
		case req.OldEmailToken:
			if _, err := tx.ExecContext(ctx, "UPDATE email_change_requests SET old_email_confirmed_at = COALESCE(old_email_confirmed_at, $1) WHERE id = $2", now, req.ID); err != nil {
				return err
			}
			if req.OldEmailConfirmedAt == nil {
				req.OldEmailConfirmedAt = new(now)
			}
		case req.NewEmailToken:
			if _, err := tx.ExecContext(ctx, "UPDATE email_change_requests SET new_email_confirmed_at = COALESCE(new_email_confirmed_at, $1) WHERE id = $2", now, req.ID); err != nil {
				return err
			}
			if req.NewEmailConfirmedAt == nil {
				req.NewEmailConfirmedAt = new(now)
			}
		default:
			return errors.New("invalid email change token")
		}

		if req.OldEmailConfirmedAt != nil && req.NewEmailConfirmedAt != nil && !now.Before(req.NotBefore) {
			if _, err := tx.ExecContext(ctx, "UPDATE users SET email = $1, email_verified = TRUE, updated_at = NOW() WHERE id = $2", req.NewEmail, req.UserID); err != nil {
				return err
			}
			if _, err := tx.ExecContext(ctx, "UPDATE email_change_requests SET completed_at = $1 WHERE id = $2", now, req.ID); err != nil {
				return err
			}
			req.CompletedAt = new(now)
		}
		return nil
	})
	if err != nil {
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
	req.OldEmailConfirmedAt = platformdb.TimePtrFromNull(oldConfirmed)
	req.NewEmailConfirmedAt = platformdb.TimePtrFromNull(newConfirmed)
	req.CompletedAt = platformdb.TimePtrFromNull(completed)
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
