package auth

import (
	"context"
	"errors"
	"slices"
)

var ErrSeparationOfDuties = errors.New("separation of duties: super_admin cannot modify their own privileged state")

var ErrCannotDeactivateSelf = errors.New("cannot deactivate your own account")

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
