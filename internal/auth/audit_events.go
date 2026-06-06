package auth

const (
	AuditSessionRevoked = "session.revoked"

	AuditMFABypassGenerated = "mfa.bypass_code_generated" //nolint:gosec // audit event name, not a credential

	AuditRoleAssigned  = "role.assigned"
	AuditRoleRemoved   = "role.removed"
	AuditAuditExport   = "audit.export"
	AuditGDPRDataDump  = "user.data_export"
	AuditAdminBlockSod = "admin.blocked_self_change"

	AuditAccountFrozen      = "account.frozen"
	AuditAccountUnfrozen    = "account.unfrozen"
	AuditAccountSoftDeleted = "account.soft_deleted"
	AuditAccountRestored    = "account.restored"
	AuditAccountUnlocked    = "account.unlocked"
	AuditAdminForceLogout   = "admin.force_logout"
)

type AuditResult string

const (
	AuditResultSuccess AuditResult = "success"
)
