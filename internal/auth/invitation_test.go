package auth

import (
	"context"
	"database/sql"
	"net/url"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/biqly/biqly/internal/auth/rbac"
	"github.com/biqly/biqly/internal/mail"
	"github.com/biqly/biqly/internal/testutil"
)

type invitationTestEnv struct {
	ctx        context.Context
	dbPool     *sql.DB
	service    *Service
	userRepo   *UserRepository
	rbacRepo   *rbac.RBACRepository
	mockMailer *mail.MockEmailSender
	normalUser *User
	superUser  *User
}

func newInvitationTestEnv(t *testing.T) invitationTestEnv {
	t.Helper()

	dbPool := testutil.OpenAuthDB(t)
	ctx := context.Background()
	testutil.ResetAuthIntegrationTables(ctx, t, dbPool, "DELETE FROM user_invitations")

	config := &Config{
		JWTAccessTTL:  5 * time.Minute,
		JWTRefreshTTL: 24 * time.Hour,
	}

	jwtMgr, err := NewJWTManager("", "", config.JWTAccessTTL)
	require.NoError(t, err)

	userRepo := NewUserRepository(dbPool, nil)
	rbacRepo := rbac.NewRBACRepository(dbPool)
	sessionMgr := NewSessionManager(dbPool)
	mockMailer := mail.NewMockEmailSender()
	service := NewAuthService(userRepo, rbacRepo, sessionMgr, jwtMgr, config, nil, mockMailer)

	// First user in empty DB is bootstrapped as super_admin; create super before normal.
	superUser, err := userRepo.CreateUser(ctx, "super@example.com", "SecurePass123!", "Super User")
	require.NoError(t, err)

	normalUser, err := userRepo.CreateUser(ctx, "normal@example.com", "SecurePass123!", "Normal User")
	require.NoError(t, err)

	return invitationTestEnv{
		ctx:        ctx,
		dbPool:     dbPool,
		service:    service,
		userRepo:   userRepo,
		rbacRepo:   rbacRepo,
		mockMailer: mockMailer,
		normalUser: normalUser,
		superUser:  superUser,
	}
}

func TestInvitationFlow(t *testing.T) {
	env := newInvitationTestEnv(t)

	// 2. Normal user tries to invite someone — must fail with ErrNotSuperAdmin
	inviteEmail := "invited@example.com"
	err := env.service.InviteUser(env.ctx, env.normalUser.ID, inviteEmail, "developer")
	assert.ErrorIs(t, err, ErrNotSuperAdmin)

	// 3. Super admin invites someone — must succeed
	err = env.service.InviteUser(env.ctx, env.superUser.ID, inviteEmail, "developer")
	require.NoError(t, err)

	// Verify email mock received the send invitation call
	require.Contains(t, env.mockMailer.SentEmails, inviteEmail)
	assert.Len(t, env.mockMailer.SentEmails[inviteEmail], 1)
	assert.Contains(t, env.mockMailer.SentEmails[inviteEmail][0], "Invitation token:")

	msg := env.mockMailer.SentEmails[inviteEmail][0]
	// Extract token from the log message: "Invitation token: <token>, role: ..."
	tokenStart := len("Invitation token: ")
	tokenEnd := tokenStart
	for tokenEnd < len(msg) && msg[tokenEnd] != ',' {
		tokenEnd++
	}
	token := msg[tokenStart:tokenEnd]
	assert.NotEmpty(t, token)

	// 4. Inviting an existing user must fail
	err = env.service.InviteUser(env.ctx, env.superUser.ID, "normal@example.com", "developer")
	assert.Error(t, err)

	// 5. Get invitation by token — must return details
	invite, err := env.service.GetInvitation(env.ctx, token)
	require.NoError(t, err)
	assert.Equal(t, inviteEmail, invite.Email)
	assert.Equal(t, "developer", invite.RoleName)

	// 6. Re-inviting a pending email rotates the token: the old link dies and
	// the newly emailed link takes over. (Tokens are stored hashed, so the
	// previous plaintext cannot be resent — rotation is the only consistent
	// behavior.)
	err = env.service.InviteUser(env.ctx, env.superUser.ID, inviteEmail, "developer")
	require.NoError(t, err)
	_, err = env.service.GetInvitation(env.ctx, token)
	assert.ErrorIs(t, err, ErrInvitationNotFound, "old token must be invalidated by re-invite")

	require.Len(t, env.mockMailer.SentEmails[inviteEmail], 2)
	msg = env.mockMailer.SentEmails[inviteEmail][1]
	tokenEnd = tokenStart
	for tokenEnd < len(msg) && msg[tokenEnd] != ',' {
		tokenEnd++
	}
	token = msg[tokenStart:tokenEnd]
	require.NotEmpty(t, token)

	reinvited, err := env.service.GetInvitation(env.ctx, token)
	require.NoError(t, err)
	assert.Equal(t, inviteEmail, reinvited.Email)
	assert.Equal(t, "developer", reinvited.RoleName)

	// 7. Claim invitation — must succeed, create user, and issue session pending verification
	claimUA := "Invite-Claim-Agent"
	claimIP := "192.168.1.1"
	claimResp, err := env.service.ClaimInvitation(env.ctx, token, "NewSecPass1!", "Invited User", claimUA, claimIP)
	require.NoError(t, err)

	assert.NotEmpty(t, claimResp.AccessToken)
	assert.NotEmpty(t, claimResp.RefreshToken)
	assert.Equal(t, inviteEmail, claimResp.Email)
	assert.True(t, claimResp.VerificationPending)

	// Verify claimed user details in DB
	claimedUser, err := env.userRepo.GetUserByID(env.ctx, claimResp.UserID)
	require.NoError(t, err)
	assert.Equal(t, inviteEmail, claimedUser.Email)
	assert.Equal(t, "Invited User", *claimedUser.DisplayName)
	assert.False(t, claimedUser.EmailVerified)
	assert.True(t, claimedUser.IsActive)

	// Verify the claimed user was assigned the 'developer' global role
	roles, err := env.rbacRepo.GetUserRoles(env.ctx, claimedUser.ID)
	require.NoError(t, err)
	assert.Contains(t, roles, "developer")

	// 8. Trying to get the invitation again must fail with ErrInvitationClaimed
	_, err = env.service.GetInvitation(env.ctx, token)
	assert.ErrorIs(t, err, ErrInvitationClaimed)
}

func TestInvitationManagement(t *testing.T) {
	env := newInvitationTestEnv(t)

	// 1. Create a couple of invitations
	err := env.service.InviteUser(env.ctx, env.superUser.ID, "invite1@example.com", "viewer")
	require.NoError(t, err)

	err = env.service.InviteUser(env.ctx, env.superUser.ID, "invite2@example.com", "analyst")
	require.NoError(t, err)

	// 2. Normal user tries to list invitations — must fail
	_, err = env.service.ListInvitations(env.ctx, env.normalUser.ID)
	assert.ErrorIs(t, err, ErrNotSuperAdmin)

	// 3. Super admin lists invitations — must succeed
	invites, err := env.service.ListInvitations(env.ctx, env.superUser.ID)
	require.NoError(t, err)
	require.Len(t, invites, 2)
	assert.Equal(t, "invite2@example.com", invites[0].Email)
	assert.Equal(t, "analyst", invites[0].RoleName)
	assert.Equal(t, "invite1@example.com", invites[1].Email)
	assert.Equal(t, "viewer", invites[1].RoleName)

	invitationID := invites[0].ID
	resentEmail := "invite2@example.com"

	// 4. Resend invitation by super admin — must succeed and issue a fresh token via email
	err = env.service.ResendInvitation(env.ctx, env.superUser.ID, invitationID)
	require.NoError(t, err)

	require.Contains(t, env.mockMailer.SentEmails, resentEmail)
	require.GreaterOrEqual(t, len(env.mockMailer.SentEmails[resentEmail]), 2)
	latestMsg := env.mockMailer.SentEmails[resentEmail][len(env.mockMailer.SentEmails[resentEmail])-1]
	tokenStart := len("Invitation token: ")
	tokenEnd := tokenStart
	for tokenEnd < len(latestMsg) && latestMsg[tokenEnd] != ',' {
		tokenEnd++
	}
	newToken := latestMsg[tokenStart:tokenEnd]
	require.NotEmpty(t, newToken)

	resentInvite, err := env.service.GetInvitation(env.ctx, newToken)
	require.NoError(t, err)
	assert.Equal(t, resentEmail, resentInvite.Email)

	// 5. Revoke invitation by normal user — must fail
	err = env.service.RevokeInvitation(env.ctx, env.normalUser.ID, invitationID)
	assert.ErrorIs(t, err, ErrNotSuperAdmin)

	// 6. Revoke invitation by super admin — must succeed
	err = env.service.RevokeInvitation(env.ctx, env.superUser.ID, invitationID)
	require.NoError(t, err)

	// Verify count is now 1
	invitesFinal, err := env.service.ListInvitations(env.ctx, env.superUser.ID)
	require.NoError(t, err)
	assert.Len(t, invitesFinal, 1)
	assert.Equal(t, "invite1@example.com", invitesFinal[0].Email)
}

func TestInvitationRouteTokenDecoding(t *testing.T) {
	token, err := url.PathUnescape("iY7nsYpBr9xdk5_Pn5xSwiVbo-iGTcM53WtyK8A1iHY%3D")
	require.NoError(t, err)
	assert.Equal(t, "iY7nsYpBr9xdk5_Pn5xSwiVbo-iGTcM53WtyK8A1iHY=", token)
}

// inviteAndExtractToken invites email as the super admin and returns the raw
// invitation token captured from the mock mailer.
func (env *invitationTestEnv) inviteAndExtractToken(t *testing.T, email string) string {
	t.Helper()
	require.NoError(t, env.service.InviteUser(env.ctx, env.superUser.ID, email, "developer"))
	require.Contains(t, env.mockMailer.SentEmails, email)
	msgs := env.mockMailer.SentEmails[email]
	msg := msgs[len(msgs)-1]
	require.Contains(t, msg, "Invitation token: ")
	tokenStart := len("Invitation token: ")
	tokenEnd := tokenStart
	for tokenEnd < len(msg) && msg[tokenEnd] != ',' {
		tokenEnd++
	}
	token := msg[tokenStart:tokenEnd]
	require.NotEmpty(t, token)
	return token
}

// TestInvitationClaimRace fires concurrent claims for the same token and
// requires that exactly one wins: the users.email unique constraint must
// serialize the inserts so a double-claim cannot mint two accounts or two
// sessions from one invitation.
func TestInvitationClaimRace(t *testing.T) {
	env := newInvitationTestEnv(t)
	raceEmail := "race-claim@example.com"
	token := env.inviteAndExtractToken(t, raceEmail)

	const attempts = 8
	results := make(chan error, attempts)
	start := make(chan struct{})
	for range attempts {
		go func() {
			<-start
			_, err := env.service.ClaimInvitation(
				env.ctx, token, "RaceSecPass1!", "Race User", "Race-Agent", "10.0.0.1")
			results <- err
		}()
	}
	close(start)

	successes := 0
	for range attempts {
		if err := <-results; err == nil {
			successes++
		}
	}
	assert.Equal(t, 1, successes, "exactly one concurrent claim must succeed")

	// Exactly one user row exists for the invited email.
	var userCount int
	require.NoError(t, env.dbPool.QueryRowContext(env.ctx,
		"SELECT COUNT(*) FROM users WHERE email = $1", raceEmail).Scan(&userCount))
	assert.Equal(t, 1, userCount)

	// The invitation is consumed: further lookups must report it claimed.
	_, err := env.service.GetInvitation(env.ctx, token)
	assert.ErrorIs(t, err, ErrInvitationClaimed)
}
