package mail

import (
	"context"
	"time"
)

// EmailSender dispatches the transactional emails the auth domain relies on.
type EmailSender interface {
	SendEmailVerification(ctx context.Context, email, token string) error
	SendPasswordReset(ctx context.Context, email, token string) error
	SendEmailChangeConfirmation(ctx context.Context, email, token string, newEmail bool) error
	SendAccountUnlock(ctx context.Context, email, token string) error
	SendNewDeviceLogin(ctx context.Context, email string, info DeviceLoginInfo) error
	SendAccountDeletionScheduled(ctx context.Context, email string, purgeAt time.Time) error
	SendDuplicateRegistrationNotice(ctx context.Context, email string) error
	SendMagicLink(ctx context.Context, email, token string) error
}
