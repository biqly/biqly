package auth

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/biqly/biqly/internal/auth/rbac"
	platformdb "github.com/biqly/biqly/internal/platform/db"
)

var (
	ErrInvitationNotFound = errors.New("invitation not found")
	ErrInvitationExpired  = errors.New("invitation has expired")
	ErrInvitationClaimed  = errors.New("invitation has already been claimed")
	ErrNotSuperAdmin      = errors.New("only super admins can invite users")
	ErrRoleNotFound       = errors.New("role not found")
)

type Invitation struct {
	ID        string     `json:"id"`
	Email     string     `json:"email"`
	Token     *string    `json:"token,omitempty"`
	RoleID    string     `json:"role_id"`
	RoleName  string     `json:"role_name"`
	InvitedBy string     `json:"invited_by"`
	CreatedAt time.Time  `json:"created_at"`
	ExpiresAt time.Time  `json:"expires_at"`
	ClaimedAt *time.Time `json:"claimed_at,omitempty"`
}

type InviteUserRequest struct {
	Email    string `json:"email"`
	RoleName string `json:"role_name"`
}

type ClaimInvitationRequest struct {
	Password    string `json:"password"`
	DisplayName string `json:"display_name"`
}

// InviteUser creates or updates an invitation for a given email and sends an email.
func (s *AuthService) InviteUser(ctx context.Context, actorUserID, email, roleName string) error {
	// 1. Authorize: actor must be a super admin.
	isSuper, err := s.IsSuperAdmin(ctx, actorUserID)
	if err != nil {
		return fmt.Errorf("check super admin: %w", err)
	}
	if !isSuper {
		return ErrNotSuperAdmin
	}

	// 2. Validate email
	normEmail, err := NormalizeEmail(email)
	if err != nil {
		return err
	}

	// 3. Check if user already exists
	_, err = s.userRepo.GetUserByEmail(ctx, normEmail)
	if err == nil {
		return errors.New("user already exists with this email")
	} else if !errors.Is(err, ErrUserNotFound) {
		return fmt.Errorf("check user existence: %w", err)
	}

	// 4. Retrieve Role ID by roleName
	var roleID string
	err = s.userRepo.db.QueryRowContext(ctx, "SELECT id FROM roles WHERE name = $1", roleName).Scan(&roleID)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrRoleNotFound
	} else if err != nil {
		return fmt.Errorf("get role: %w", err)
	}

	// 5. Generate secure token
	token, err := s.generateSecureToken()
	if err != nil {
		return fmt.Errorf("generate secure token: %w", err)
	}

	expiresAt := time.Now().Add(48 * time.Hour)

	// 6. Create/Upsert invitation in DB
	query := `
		INSERT INTO user_invitations (email, token, role_id, invited_by, expires_at, claimed_at)
		VALUES ($1, $2, $3, $4, $5, NULL)
		ON CONFLICT (email)
		DO UPDATE SET token = EXCLUDED.token, role_id = EXCLUDED.role_id, invited_by = EXCLUDED.invited_by, expires_at = EXCLUDED.expires_at, claimed_at = NULL, created_at = NOW()
	`
	_, err = s.userRepo.db.ExecContext(ctx, query, normEmail, token, roleID, actorUserID, expiresAt)
	if err != nil {
		return fmt.Errorf("upsert invitation: %w", err)
	}

	// 7. Send Invitation Email
	if s.emailSender != nil {
		err = s.emailSender.SendInvitation(ctx, normEmail, token, roleName, expiresAt)
		if err != nil {
			return fmt.Errorf("send invitation email: %w", err)
		}
	}

	return nil
}

// GetInvitation retrieves a valid, unclaimed invitation by token.
func (s *AuthService) GetInvitation(ctx context.Context, token string) (*Invitation, error) {
	if token == "" {
		return nil, ErrInvitationNotFound
	}

	query := `
		SELECT ui.id, ui.email, ui.token, ui.role_id, r.name as role_name, ui.invited_by, ui.created_at, ui.expires_at, ui.claimed_at
		FROM user_invitations ui
		JOIN roles r ON ui.role_id = r.id
		WHERE ui.token = $1
	`
	var invite Invitation
	var claimedAtNull sql.NullTime
	var tokenNull sql.NullString

	err := s.userRepo.db.QueryRowContext(ctx, query, token).Scan(
		&invite.ID, &invite.Email, &tokenNull, &invite.RoleID, &invite.RoleName,
		&invite.InvitedBy, &invite.CreatedAt, &invite.ExpiresAt, &claimedAtNull,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrInvitationNotFound
	} else if err != nil {
		return nil, fmt.Errorf("get invitation: %w", err)
	}

	if tokenNull.Valid {
		invite.Token = &tokenNull.String
	}
	if claimedAtNull.Valid {
		invite.ClaimedAt = &claimedAtNull.Time
		return nil, ErrInvitationClaimed
	}

	if time.Now().After(invite.ExpiresAt) {
		return nil, ErrInvitationExpired
	}

	return &invite, nil
}

// ClaimInvitation claims the invitation, creates the user account and workspace, and logs in.
func (s *AuthService) ClaimInvitation(ctx context.Context, token, password, displayName, userAgent, ipAddress string) (*TokenResponse, error) {
	// 1. Retrieve and validate invitation
	invite, err := s.GetInvitation(ctx, token)
	if err != nil {
		return nil, err
	}

	// 2. Validate password policy and sanitize display name
	sanitizedDisplayName, err := SanitizeDisplayName(displayName)
	if err != nil {
		return nil, err
	}
	if sanitizedDisplayName == "" {
		sanitizedDisplayName = invite.Email
	}

	if err := s.config.PasswordPolicy.Validate(password, invite.Email, sanitizedDisplayName); err != nil {
		return nil, err
	}

	// 3. Hash password
	hash, err := HashPassword(password)
	if err != nil {
		return nil, fmt.Errorf("hash password: %w", err)
	}

	// 4. Run database transaction to create user and claim invite
	var user User
	var workspaceID string
	err = platformdb.RunInTx(ctx, s.userRepo.db, func(tx *sql.Tx) error {
		// Double check user doesn't already exist (race condition)
		var exists bool
		if err := tx.QueryRowContext(ctx, "SELECT EXISTS(SELECT 1 FROM users WHERE email = $1)", invite.Email).Scan(&exists); err != nil {
			return fmt.Errorf("check user existence: %w", err)
		}
		if exists {
			return errors.New("user already exists with this email")
		}

		// Insert User (pre-verified)
		userInsertQuery := `
			INSERT INTO users (email, password_hash, display_name, password_changed_at, email_verified)
			VALUES ($1, $2, $3, NOW(), TRUE)
			RETURNING id, email, username, display_name, avatar_url, is_active, email_verified, created_at, updated_at
		`
		var usernameNull, displayNameNull, avatarURLNull sql.NullString
		if err := tx.QueryRowContext(ctx, userInsertQuery, invite.Email, hash, sanitizedDisplayName).Scan(
			&user.ID, &user.Email, &usernameNull, &displayNameNull, &avatarURLNull,
			&user.IsActive, &user.EmailVerified, &user.CreatedAt, &user.UpdatedAt,
		); err != nil {
			return fmt.Errorf("insert user: %w", err)
		}

		if err := insertPasswordHistory(ctx, tx, user.ID, hash); err != nil {
			return fmt.Errorf("insert password history: %w", err)
		}

		// Create personal workspace
		workspaceQuery := `
			INSERT INTO workspaces (name, slug, is_personal, created_by)
			VALUES ($1, $2, TRUE, $3)
			RETURNING id
		`
		workspaceSlug := fmt.Sprintf("%s-personal", user.ID)
		workspaceName := fmt.Sprintf("%s's Workspace", sanitizedDisplayName)

		if err := tx.QueryRowContext(ctx, workspaceQuery, workspaceName, workspaceSlug, user.ID).Scan(&workspaceID); err != nil {
			return fmt.Errorf("create personal workspace: %w", err)
		}

		// Assign the global role preconfigured in the invitation
		userRoleQuery := `
			INSERT INTO user_roles (user_id, role_id, scope_type, scope_id)
			VALUES ($1, $2, 'global', '00000000-0000-0000-0000-000000000000')
		`
		if _, err := tx.ExecContext(ctx, userRoleQuery, user.ID, invite.RoleID); err != nil {
			return fmt.Errorf("assign role: %w", err)
		}

		// Add user as admin to their personal workspace
		var adminRoleID string
		if err := tx.QueryRowContext(ctx, "SELECT id FROM roles WHERE name = 'admin'").Scan(&adminRoleID); err != nil {
			return fmt.Errorf("get admin role: %w", err)
		}

		workspaceMemberQuery := `
			INSERT INTO workspace_members (workspace_id, user_id, role_id)
			VALUES ($1, $2, $3)
		`
		if _, err := tx.ExecContext(ctx, workspaceMemberQuery, workspaceID, user.ID, adminRoleID); err != nil {
			return fmt.Errorf("add member to workspace: %w", err)
		}

		// Update invitation as claimed and clear token
		claimQuery := `
			UPDATE user_invitations
			SET claimed_at = NOW(), token = NULL
			WHERE id = $1
		`
		if _, err := tx.ExecContext(ctx, claimQuery, invite.ID); err != nil {
			return fmt.Errorf("mark invitation claimed: %w", err)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	// 5. Generate active session
	roles, err := s.rbacRepo.GetUserRoles(ctx, user.ID)
	if err != nil {
		return nil, err
	}

	accessToken, err := s.jwtMgr.GenerateTokenWithVerification(user.ID, user.Email, user.EmailVerified, roles, workspaceID, nil)
	if err != nil {
		return nil, fmt.Errorf("generate access token: %w", err)
	}

	refreshToken, err := s.sessionMgr.CreateSession(ctx, user.ID, &userAgent, &ipAddress, s.config.JWTRefreshTTL)
	if err != nil {
		return nil, fmt.Errorf("create session: %w", err)
	}

	return &TokenResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		UserID:       user.ID,
		Email:        user.Email,
		Roles:        roles,
	}, nil
}

// IsSuperAdmin helper checks if a user is a super admin using their roles.
func (s *AuthService) IsSuperAdmin(ctx context.Context, userID string) (bool, error) {
	roles, err := s.rbacRepo.GetUserRoles(ctx, userID)
	if err != nil {
		return false, err
	}
	return slices.Contains(roles, rbac.RoleSuperAdmin), nil
}

// ListInvitations lists all user invitations.
func (s *AuthService) ListInvitations(ctx context.Context, actorUserID string) ([]*Invitation, error) {
	isSuper, err := s.IsSuperAdmin(ctx, actorUserID)
	if err != nil {
		return nil, fmt.Errorf("check super admin: %w", err)
	}
	if !isSuper {
		return nil, ErrNotSuperAdmin
	}

	query := `
		SELECT ui.id, ui.email, ui.token, ui.role_id, r.name as role_name, COALESCE(u.display_name, u.email, ui.invited_by::text) as invited_by, ui.created_at, ui.expires_at, ui.claimed_at
		FROM user_invitations ui
		JOIN roles r ON ui.role_id = r.id
		LEFT JOIN users u ON ui.invited_by = u.id
		ORDER BY ui.created_at DESC
	`
	rows, err := s.userRepo.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("query invitations: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var invites []*Invitation
	for rows.Next() {
		var invite Invitation
		var claimedAtNull sql.NullTime
		var tokenNull sql.NullString
		err := rows.Scan(
			&invite.ID, &invite.Email, &tokenNull, &invite.RoleID, &invite.RoleName,
			&invite.InvitedBy, &invite.CreatedAt, &invite.ExpiresAt, &claimedAtNull,
		)
		if err != nil {
			return nil, fmt.Errorf("scan invitation: %w", err)
		}
		if tokenNull.Valid {
			invite.Token = &tokenNull.String
		}
		if claimedAtNull.Valid {
			invite.ClaimedAt = &claimedAtNull.Time
		}
		invites = append(invites, &invite)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate invitations: %w", err)
	}
	return invites, nil
}

// RevokeInvitation deletes an invitation, rendering its token invalid.
func (s *AuthService) RevokeInvitation(ctx context.Context, actorUserID, invitationID string) error {
	isSuper, err := s.IsSuperAdmin(ctx, actorUserID)
	if err != nil {
		return fmt.Errorf("check super admin: %w", err)
	}
	if !isSuper {
		return ErrNotSuperAdmin
	}

	query := `DELETE FROM user_invitations WHERE id = $1`
	res, err := s.userRepo.db.ExecContext(ctx, query, invitationID)
	if err != nil {
		return fmt.Errorf("delete invitation: %w", err)
	}

	rows, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return ErrInvitationNotFound
	}
	return nil
}

// ResendInvitation updates an invitation with a fresh token and expiration, and triggers email delivery.
func (s *AuthService) ResendInvitation(ctx context.Context, actorUserID, invitationID string) error {
	isSuper, err := s.IsSuperAdmin(ctx, actorUserID)
	if err != nil {
		return fmt.Errorf("check super admin: %w", err)
	}
	if !isSuper {
		return ErrNotSuperAdmin
	}

	// Fetch invitation details to retrieve email and role name
	query := `
		SELECT ui.email, r.name as role_name
		FROM user_invitations ui
		JOIN roles r ON ui.role_id = r.id
		WHERE ui.id = $1
	`
	var email, roleName string
	err = s.userRepo.db.QueryRowContext(ctx, query, invitationID).Scan(&email, &roleName)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrInvitationNotFound
	} else if err != nil {
		return fmt.Errorf("get invitation for resend: %w", err)
	}

	// Generate secure token
	token, err := s.generateSecureToken()
	if err != nil {
		return fmt.Errorf("generate secure token: %w", err)
	}

	expiresAt := time.Now().Add(48 * time.Hour)

	// Update invitation token, expiration and set claimed_at to null
	updateQuery := `
		UPDATE user_invitations
		SET token = $1, expires_at = $2, claimed_at = NULL, created_at = NOW(), invited_by = $3
		WHERE id = $4
	`
	_, err = s.userRepo.db.ExecContext(ctx, updateQuery, token, expiresAt, actorUserID, invitationID)
	if err != nil {
		return fmt.Errorf("update invitation token: %w", err)
	}

	// Send Invitation Email
	if s.emailSender != nil {
		err = s.emailSender.SendInvitation(ctx, email, token, roleName, expiresAt)
		if err != nil {
			return fmt.Errorf("send invitation email: %w", err)
		}
	}

	return nil
}
