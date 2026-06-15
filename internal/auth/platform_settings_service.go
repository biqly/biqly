package auth

import "context"

func (s *Service) SetPlatformSettingsRepository(r *PlatformSettingsRepository) {
	s.platformSettings = r
}

func (s *Service) GetPlatformSettings(ctx context.Context) (PlatformSettings, error) {
	if s.platformSettings == nil {
		return PlatformSettings{SelfSignupEnabled: true}, nil
	}
	return s.platformSettings.Get(ctx)
}

func (s *Service) SelfSignupEnabled(ctx context.Context) (bool, error) {
	settings, err := s.GetPlatformSettings(ctx)
	if err != nil {
		return false, err
	}
	return settings.SelfSignupEnabled, nil
}

func (s *Service) FirstUserSetupRequired(ctx context.Context) (bool, error) {
	return s.userRepo.FirstUserSetupRequired(ctx)
}

func (s *Service) RegistrationAllowed(ctx context.Context) (bool, error) {
	enabled, err := s.SelfSignupEnabled(ctx)
	if err != nil {
		return false, err
	}
	if enabled {
		return true, nil
	}
	return s.FirstUserSetupRequired(ctx)
}

func (s *Service) UpdatePlatformSettings(ctx context.Context, actorUserID string, selfSignupEnabled bool) (PlatformSettings, error) {
	if err := s.requireSuperAdmin(ctx, actorUserID); err != nil {
		return PlatformSettings{}, err
	}
	if s.platformSettings == nil {
		return PlatformSettings{}, ErrPlatformSettingsNotFound
	}
	return s.platformSettings.SetSelfSignupEnabled(ctx, selfSignupEnabled, actorUserID)
}

func (s *Service) requireSuperAdmin(ctx context.Context, userID string) error {
	isSuper, err := s.IsSuperAdmin(ctx, userID)
	if err != nil {
		return err
	}
	if !isSuper {
		return ErrNotSuperAdmin
	}
	return nil
}
