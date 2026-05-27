package auth

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/biqly/biqly/internal/mail"
)

func TestInvitationFlow(t *testing.T) {
	dbPool := openTestDBPool(t)
	ctx := context.Background()

	// Clear test tables to keep tests clean and repeatable
	_, _ = dbPool.ExecContext(ctx, "DELETE FROM user_invitations")
	_, _ = dbPool.ExecContext(ctx, "DELETE FROM sessions")
	_, _ = dbPool.ExecContext(ctx, "DELETE FROM workspace_members")
	_, _ = dbPool.ExecContext(ctx, "DELETE FROM workspaces")
	_, _ = dbPool.ExecContext(ctx, "DELETE FROM user_roles")
	_, _ = dbPool.ExecContext(ctx, "DELETE FROM users")

	config := &Config{
		JWTAccessTTL:  5 * time.Minute,
		JWTRefreshTTL: 24 * time.Hour,
	}

	jwtMgr, err := NewJWTManager("", "", config.JWTAccessTTL)
	require.NoError(t, err)

	userRepo := NewUserRepository(dbPool, nil)
	rbacRepo := NewRBACRepository(dbPool)
	sessionMgr := NewSessionManager(dbPool)
	mockMailer := mail.NewMockEmailSender()
	service := NewAuthService(userRepo, rbacRepo, sessionMgr, jwtMgr, config, nil, mockMailer)

	// 1. Create a normal user (not super admin) and a super admin user
	normalUser, err := userRepo.CreateUser(ctx, "normal@example.com", "SecurePass123!", "Normal User")
	require.NoError(t, err)

	superUser, err := userRepo.CreateUser(ctx, "super@example.com", "SecurePass123!", "Super User")
	require.NoError(t, err)

	// Make superUser a super_admin role holder
	var superRoleID string
	err = dbPool.QueryRowContext(ctx, "SELECT id FROM roles WHERE name = 'super_admin'").Scan(&superRoleID)
	require.NoError(t, err)

	_, err = dbPool.ExecContext(ctx, "INSERT INTO user_roles (user_id, role_id, scope_type, scope_id) VALUES ($1, $2, 'global', '00000000-0000-0000-0000-000000000000')", superUser.ID, superRoleID)
	require.NoError(t, err)

	// 2. Normal user tries to invite someone — must fail with ErrNotSuperAdmin
	inviteEmail := "invited@example.com"
	err = service.InviteUser(ctx, normalUser.ID, inviteEmail, "developer")
	assert.ErrorIs(t, err, ErrNotSuperAdmin)

	// 3. Super admin invites someone — must succeed
	err = service.InviteUser(ctx, superUser.ID, inviteEmail, "developer")
	require.NoError(t, err)

	// Verify email mock received the send invitation call
	require.Contains(t, mockMailer.SentEmails, inviteEmail)
	assert.Len(t, mockMailer.SentEmails[inviteEmail], 1)
	assert.Contains(t, mockMailer.SentEmails[inviteEmail][0], "Invitation token:")

	msg := mockMailer.SentEmails[inviteEmail][0]
	// Extract token from the log message: "Invitation token: <token>, role: ..."
	tokenStart := len("Invitation token: ")
	tokenEnd := tokenStart
	for tokenEnd < len(msg) && msg[tokenEnd] != ',' {
		tokenEnd++
	}
	token := msg[tokenStart:tokenEnd]
	assert.NotEmpty(t, token)

	// 4. Inviting an existing user must fail
	err = service.InviteUser(ctx, superUser.ID, "normal@example.com", "developer")
	assert.Error(t, err)

	// 5. Get invitation by token — must return details
	invite, err := service.GetInvitation(ctx, token)
	require.NoError(t, err)
	assert.Equal(t, inviteEmail, invite.Email)
	assert.Equal(t, "developer", invite.RoleName)

	// 6. Claim invitation — must succeed, create user, mark email verified, and log in
	claimUA := "Invite-Claim-Agent"
	claimIP := "192.168.1.1"
	claimResp, err := service.ClaimInvitation(ctx, token, "NewSecPass1!", "Invited User", claimUA, claimIP)
	require.NoError(t, err)

	assert.NotEmpty(t, claimResp.AccessToken)
	assert.NotEmpty(t, claimResp.RefreshToken)
	assert.Equal(t, inviteEmail, claimResp.Email)

	// Verify claimed user details in DB
	claimedUser, err := userRepo.GetUserByID(ctx, claimResp.UserID)
	require.NoError(t, err)
	assert.Equal(t, inviteEmail, claimedUser.Email)
	assert.Equal(t, "Invited User", *claimedUser.DisplayName)
	assert.True(t, claimedUser.EmailVerified)
	assert.True(t, claimedUser.IsActive)

	// Verify the claimed user was assigned the 'developer' global role
	roles, err := rbacRepo.GetUserRoles(ctx, claimedUser.ID)
	require.NoError(t, err)
	assert.Contains(t, roles, "developer")

	// 7. Trying to get the invitation again must fail with ErrInvitationClaimed
	_, err = service.GetInvitation(ctx, token)
	assert.ErrorIs(t, err, ErrInvitationClaimed)
}

func TestInvitationManagement(t *testing.T) {
	dbPool := openTestDBPool(t)
	ctx := context.Background()

	// Clear test tables to keep tests clean and repeatable
	_, _ = dbPool.ExecContext(ctx, "DELETE FROM user_invitations")
	_, _ = dbPool.ExecContext(ctx, "DELETE FROM sessions")
	_, _ = dbPool.ExecContext(ctx, "DELETE FROM workspace_members")
	_, _ = dbPool.ExecContext(ctx, "DELETE FROM workspaces")
	_, _ = dbPool.ExecContext(ctx, "DELETE FROM user_roles")
	_, _ = dbPool.ExecContext(ctx, "DELETE FROM users")

	config := &Config{
		JWTAccessTTL:  5 * time.Minute,
		JWTRefreshTTL: 24 * time.Hour,
	}

	jwtMgr, err := NewJWTManager("", "", config.JWTAccessTTL)
	require.NoError(t, err)

	userRepo := NewUserRepository(dbPool, nil)
	rbacRepo := NewRBACRepository(dbPool)
	sessionMgr := NewSessionManager(dbPool)
	mockMailer := mail.NewMockEmailSender()
	service := NewAuthService(userRepo, rbacRepo, sessionMgr, jwtMgr, config, nil, mockMailer)

	// Create users
	normalUser, err := userRepo.CreateUser(ctx, "normal@example.com", "SecurePass123!", "Normal User")
	require.NoError(t, err)

	superUser, err := userRepo.CreateUser(ctx, "super@example.com", "SecurePass123!", "Super User")
	require.NoError(t, err)

	var superRoleID string
	err = dbPool.QueryRowContext(ctx, "SELECT id FROM roles WHERE name = 'super_admin'").Scan(&superRoleID)
	require.NoError(t, err)

	_, err = dbPool.ExecContext(ctx, "INSERT INTO user_roles (user_id, role_id, scope_type, scope_id) VALUES ($1, $2, 'global', '00000000-0000-0000-0000-000000000000')", superUser.ID, superRoleID)
	require.NoError(t, err)

	// 1. Create a couple of invitations
	err = service.InviteUser(ctx, superUser.ID, "invite1@example.com", "viewer")
	require.NoError(t, err)

	err = service.InviteUser(ctx, superUser.ID, "invite2@example.com", "analyst")
	require.NoError(t, err)

	// 2. Normal user tries to list invitations — must fail
	_, err = service.ListInvitations(ctx, normalUser.ID)
	assert.ErrorIs(t, err, ErrNotSuperAdmin)

	// 3. Super admin lists invitations — must succeed
	invites, err := service.ListInvitations(ctx, superUser.ID)
	require.NoError(t, err)
	require.Len(t, invites, 2)
	assert.Equal(t, "invite2@example.com", invites[0].Email)
	assert.Equal(t, "analyst", invites[0].RoleName)
	assert.Equal(t, "invite1@example.com", invites[1].Email)
	assert.Equal(t, "viewer", invites[1].RoleName)

	invitationID := invites[0].ID

	// 4. Resend invitation by super admin — must succeed and update token
	oldToken := invites[0].Token
	err = service.ResendInvitation(ctx, superUser.ID, invitationID)
	require.NoError(t, err)

	// Fetch updated list and check token changed
	invitesUpdated, err := service.ListInvitations(ctx, superUser.ID)
	require.NoError(t, err)
	assert.NotEqual(t, oldToken, invitesUpdated[0].Token)

	// 5. Revoke invitation by normal user — must fail
	err = service.RevokeInvitation(ctx, normalUser.ID, invitationID)
	assert.ErrorIs(t, err, ErrNotSuperAdmin)

	// 6. Revoke invitation by super admin — must succeed
	err = service.RevokeInvitation(ctx, superUser.ID, invitationID)
	require.NoError(t, err)

	// Verify count is now 1
	invitesFinal, err := service.ListInvitations(ctx, superUser.ID)
	require.NoError(t, err)
	assert.Len(t, invitesFinal, 1)
	assert.Equal(t, "invite1@example.com", invitesFinal[0].Email)
}

