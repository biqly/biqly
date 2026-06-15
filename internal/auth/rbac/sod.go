package rbac

import (
	"context"
	"errors"
	"slices"
)

var ErrSeparationOfDuties = errors.New("separation of duties: super_admin cannot modify their own privileged state")

var ErrCannotDeactivateSelf = errors.New("cannot deactivate your own account")

var ErrPrivilegedRoleEscalation = errors.New("only super_admin may assign or remove privileged roles")

func (r *RBACRepository) EnforceSelfModificationGuard(ctx context.Context, callerID, targetUserID string, action string) error {
	if callerID != "" && callerID == targetUserID && action == "user.deactivate" {
		return ErrCannotDeactivateSelf
	}
	if callerID == "" || callerID != targetUserID {
		return nil
	}
	roles, err := r.GetUserRoles(ctx, callerID)
	if err != nil {
		return err
	}
	if slices.Contains(roles, RoleSuperAdmin) {
		return ErrSeparationOfDuties
	}
	_ = action
	return nil
}

func (r *RBACRepository) EnforcePrivilegedRoleAssignmentGuard(ctx context.Context, callerID, roleID string) error {
	privileged, err := r.RoleGrantsAdminPermissions(ctx, roleID)
	if err != nil {
		return err
	}
	if !privileged {
		return nil
	}
	roles, err := r.GetUserRoles(ctx, callerID)
	if err != nil {
		return err
	}
	if !slices.Contains(roles, RoleSuperAdmin) {
		return ErrPrivilegedRoleEscalation
	}
	return nil
}
