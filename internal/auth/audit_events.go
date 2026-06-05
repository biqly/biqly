package auth

const (
	AuditLoginSuccess     = "login.success"
	AuditLoginFailed      = "login.failed"
	AuditLoginLocked      = "login.locked"
	AuditLoginMFARequired = "login.mfa_required"
	AuditLogout           = "logout"
	AuditRegister         = "user.register"
	AuditPasswordReset    = "password.reset"
	AuditPasswordChange   = "password.change"
	AuditEmailVerified    = "email.verified"
	AuditRefresh          = "session.refresh"
	AuditSessionRevoked   = "session.revoked"
	AuditOAuthLogin       = "oauth.login"

	AuditMFAEnrolled        = "mfa.enrolled"
	AuditMFAVerified        = "mfa.verified"
	AuditMFADisabled        = "mfa.disabled"
	AuditMFABypassGenerated = "mfa.bypass_code_generated" //nolint:gosec // audit event name, not a credential

	AuditRoleAssigned  = "role.assigned"
	AuditRoleRemoved   = "role.removed"
	AuditDSGrant       = "datasource.grant"
	AuditDSRevoke      = "datasource.revoke"
	AuditDSUpdate      = "datasource.update_level"
	AuditDSRequest     = "datasource.request_access"
	AuditShareCreate   = "share.create"
	AuditShareRevoke   = "share.revoke"
	AuditAuditExport   = "audit.export"
	AuditGDPRDataDump  = "user.data_export"
	AuditAdminBlockSod = "admin.blocked_self_change"

	AuditAccountFrozen      = "account.frozen"
	AuditAccountUnfrozen    = "account.unfrozen"
	AuditAccountSoftDeleted = "account.soft_deleted"
	AuditAccountRestored    = "account.restored"
	AuditAccountPurged      = "account.purged"
	AuditAccountUnlocked    = "account.unlocked"
	AuditLoginNewDevice     = "login.new_device"
	AuditSessionEvicted     = "session.evicted"
	AuditAdminForceLogout   = "admin.force_logout"
	AuditPasswordExpired    = "password.expired"
)

type AuditResult string

const (
	AuditResultSuccess AuditResult = "success"
	AuditResultFailure AuditResult = "failure"
	AuditResultDenied  AuditResult = "denied"
)
