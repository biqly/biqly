package mfa_test

import (
	"strings"
	"testing"

	"github.com/biqly/biqly/internal/auth"
	"github.com/biqly/biqly/internal/auth/mfa"
	"github.com/biqly/biqly/internal/auth/mfatest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMFABypassCodeFlow(t *testing.T) {
	stack := mfatest.NewIntegrationStack(t)
	users := stack.SeedBypassTestUsers(t)

	// 1. Try to generate bypass code when user has not enrolled in MFA at all
	_, err := stack.Auth.GenerateMFABypassCode(stack.Ctx, users.SuperActorID, users.TargetUserID)
	assert.ErrorIs(t, err, mfa.ErrMFANotEnrolled)

	// Enroll target user in MFA
	enrollResult, err := stack.MFA.Enroll(stack.Ctx, users.TargetUserID, users.TargetEmail)
	require.NoError(t, err)

	// 2. Try to generate bypass code when MFA enrollment is pending (not verified yet)
	_, err = stack.Auth.GenerateMFABypassCode(stack.Ctx, users.SuperActorID, users.TargetUserID)
	assert.ErrorIs(t, err, mfa.ErrMFANotEnabled)

	// Verify and enable MFA
	// Since we don't have a real time TOTP code for the secret in tests easily, let's update user_mfa directly to enabled
	_, err = stack.DB.ExecContext(stack.Ctx, "UPDATE user_mfa SET enabled = TRUE, verified_at = NOW() WHERE user_id = $1", users.TargetUserID)
	require.NoError(t, err)

	// 3. Try to generate bypass code as a normal user (not super admin)
	_, err = stack.Auth.GenerateMFABypassCode(stack.Ctx, users.NormalActorID, users.TargetUserID)
	assert.ErrorIs(t, err, auth.ErrSuperAdminRequired)

	// 4. Generate bypass code as a super admin
	bypassCode, err := stack.Auth.GenerateMFABypassCode(stack.Ctx, users.SuperActorID, users.TargetUserID)
	require.NoError(t, err)
	assert.True(t, strings.HasPrefix(bypassCode, "BYPASS-"))
	assert.Len(t, bypassCode, 15) // BYPASS- (7 chars) + 8 chars Base32 = 15 chars

	// 5. Verify case-insensitive, padded bypass code verification works
	paddedBypassCode := "  " + strings.ToLower(bypassCode) + "  "
	err = stack.MFA.VerifyCode(stack.Ctx, users.TargetUserID, paddedBypassCode)
	require.NoError(t, err)

	// 6. Verification code is single-use, so verifying it again should fail
	err = stack.MFA.VerifyCode(stack.Ctx, users.TargetUserID, bypassCode)
	assert.ErrorIs(t, err, mfa.ErrMFACodeInvalid)

	// 7. Verify TOTP secret is intact after bypass code consumption
	enrollment, err := stack.MFARepo.Get(stack.Ctx, users.TargetUserID)
	require.NoError(t, err)
	assert.Equal(t, enrollResult.Secret, enrollment.Secret)
}
