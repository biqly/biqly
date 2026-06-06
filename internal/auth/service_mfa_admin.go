package auth

import (
	"context"
)

func (s *Service) GenerateMFABypassCode(ctx context.Context, actorUserID, targetUserID string) (string, error) {
	isSuper, err := s.IsSuperAdmin(ctx, actorUserID)
	if err != nil {
		return "", err
	}
	if !isSuper {
		return "", ErrSuperAdminRequired
	}
	if actorUserID == targetUserID {
		return "", ErrSuperAdminRequired
	}

	if s.mfaSvc == nil {
		return "", ErrMFANotEnabled
	}

	// Verify target user exists
	_, err = s.userRepo.GetUserByID(ctx, targetUserID)
	if err != nil {
		return "", err
	}

	return s.mfaSvc.GenerateBypassCode(ctx, targetUserID)
}
