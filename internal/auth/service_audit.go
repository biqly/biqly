package auth

import (
	"context"
	"log/slog"
)

func (s *Service) SetAuditService(a *AuditService) {
	s.auditSvc = a
}

func (s *Service) auditEvent(ctx context.Context, userID, action string, ipAddress *string) {
	if s.auditSvc == nil {
		return
	}
	uid := userID
	if err := s.auditSvc.Log(ctx, &uid, action, nil, nil, nil, ipAddress); err != nil {
		slog.WarnContext(ctx, "audit log failed", "action", action, "user_id", userID, "err", err)
	}
}
