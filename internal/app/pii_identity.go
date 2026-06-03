package app

import (
	"context"

	"github.com/biqly/biqly/internal/audit"
	"github.com/biqly/biqly/internal/config"
	"github.com/biqly/biqly/internal/core"
	bimw "github.com/biqly/biqly/internal/http/middleware"
	"github.com/biqly/biqly/internal/metadata"
)

// jwtIdentity resolves the calling user's ID and roles from the JWT auth
// middleware context. Used by core.PIIPolicyService to pick masking defaults.
func jwtIdentity(ctx context.Context) (string, []string) {
	return bimw.UserID(ctx), bimw.UserRoles(ctx)
}

// providePIIPolicyService builds the per-user PII masking resolver, or nil
// when the PII subsystem is disabled by config.
func providePIIPolicyService(cfg *config.Config, metaRepo *metadata.Repository, auditLogger *audit.Logger) *core.PIIPolicyService {
	if cfg != nil && !cfg.PII.Enabled {
		return nil
	}
	return core.NewPIIPolicyService(metaRepo, jwtIdentity).WithAudit(auditLogger)
}
