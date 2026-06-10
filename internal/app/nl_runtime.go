package app

import (
	"context"
	"fmt"

	"github.com/biqly/biqly/internal/ai/lexicon"
	"github.com/biqly/biqly/internal/i18n"
	"github.com/biqly/biqly/internal/metadata"
)

// wireNLRuntime seeds and wires the DB-backed language runtime (ADR-0001):
// the NL lexicon (ai_nl_lexicon) and the i18n locale registry + bundle overlay
// (i18n_locales / i18n_bundles). Empty tables are seeded from embedded
// defaults; the stores keep serving embedded data when the DB is unreachable.
func wireNLRuntime(ctx context.Context, metaRepo *metadata.Repository) error {
	if err := lexicon.Seed(ctx, metaRepo); err != nil {
		return fmt.Errorf("seed nl lexicon: %w", err)
	}
	lexicon.SetActive(lexicon.NewDBStore(metaRepo))

	if err := metadata.SeedI18nLocales(ctx, metaRepo); err != nil {
		return fmt.Errorf("seed i18n locales: %w", err)
	}
	i18n.SetRuntimeProvider(metadata.NewI18nRuntimeProvider(metaRepo))
	return nil
}
