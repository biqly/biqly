package auth

import "context"

func (s *AuthService) SetPlatformSettingsRepository(r *PlatformSettingsRepository) {
	s.platformSettings = r
}

func (s *AuthService) GetPlatformSettings(ctx context.Context) (PlatformSettings, error) {
	if s.platformSettings == nil {
		return PlatformSettings{SelfSignupEnabled: true}, nil
	}
	return s.platformSettings.Get(ctx)
}

func (s *AuthService) SelfSignupEnabled(ctx context.Context) (bool, error) {
	settings, err := s.GetPlatformSettings(ctx)
	if err != nil {
		return false, err
	}
	return settings.SelfSignupEnabled, nil
}

func (s *AuthService) UpdatePlatformSettings(ctx context.Context, actorUserID string, selfSignupEnabled bool) (PlatformSettings, error) {
	if err := s.requireSuperAdmin(ctx, actorUserID); err != nil {
		return PlatformSettings{}, err
	}
	if s.platformSettings == nil {
		return PlatformSettings{}, ErrPlatformSettingsNotFound
	}
	return s.platformSettings.SetSelfSignupEnabled(ctx, selfSignupEnabled, actorUserID)
}

func (s *AuthService) requireSuperAdmin(ctx context.Context, userID string) error {
	isSuper, err := s.IsSuperAdmin(ctx, userID)
	if err != nil {
		return err
	}
	if !isSuper {
		return ErrNotSuperAdmin
	}
	return nil
}
