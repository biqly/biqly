package semantic

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	platformdb "github.com/biqly/biqly/internal/platform/db"
)

// ComponentProvider loads the published full semantic model for a component.
// *Repository satisfies this interface.
type ComponentProvider interface {
	GetPublishedFullModel(ctx context.Context, id string) (*SemanticModel, error)
}

// CompositeLimits caps the size of a composite model. A zero value for any
// field means "no limit". Limits are enforced at validate/publish time.
type CompositeLimits struct {
	// MaxComponents caps the number of component models in a composite.
	MaxComponents int
	// MaxCrossJoins caps the number of active cross-model joins.
	MaxCrossJoins int
	// MaxMergedFields caps the combined dimensions + metrics of the resolved model.
	MaxMergedFields int
}

// WithLimits attaches size limits enforced when validating or publishing a
// composite. The zero CompositeLimits leaves all limits disabled, so callers
// may wire it unconditionally.
func (r *CompositeRepository) WithLimits(limits CompositeLimits) *CompositeRepository {
	r.limits = limits
	return r
}

// enforceLimits returns error strings for any composite that exceeds the
// repository's configured limits. resolved may be nil (e.g. when base
// validation already failed); merged-field counting is skipped in that case.
func (r *CompositeRepository) enforceLimits(composite *CompositeModel, resolved *SemanticModel) []string {
	var errs []string
	if r.limits.MaxComponents > 0 && len(composite.Components) > r.limits.MaxComponents {
		errs = append(errs, fmt.Sprintf("composite has %d components; limit is %d", len(composite.Components), r.limits.MaxComponents))
	}
	if r.limits.MaxCrossJoins > 0 {
		active := len(activeCrossJoins(composite.CrossModelJoins))
		if active > r.limits.MaxCrossJoins {
			errs = append(errs, fmt.Sprintf("composite has %d active cross-model joins; limit is %d", active, r.limits.MaxCrossJoins))
		}
	}
	if r.limits.MaxMergedFields > 0 && resolved != nil {
		merged := len(resolved.Dimensions) + len(resolved.Metrics)
		if merged > r.limits.MaxMergedFields {
			errs = append(errs, fmt.Sprintf("resolved composite has %d merged fields; limit is %d", merged, r.limits.MaxMergedFields))
		}
	}
	return errs
}

// CompositePublishResult is returned by composite validate/publish endpoints.
type CompositePublishResult struct {
	Composite  *CompositeModel         `json:"composite,omitempty"`
	Resolved   *SemanticModel          `json:"resolved,omitempty"`
	Validation PublishValidationResult `json:"validation"`
	Version    int                     `json:"version,omitempty"`
}

// resolveComponents loads each component as a published full model keyed by alias.
func resolveComponents(ctx context.Context, composite *CompositeModel, provider ComponentProvider) (map[string]*SemanticModel, PublishValidationResult) {
	result := PublishValidationResult{Valid: true}
	components := make(map[string]*SemanticModel, len(composite.Components))
	for i := range composite.Components {
		comp := composite.Components[i]
		model, err := provider.GetPublishedFullModel(ctx, comp.ModelID)
		if err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("component %q: %s", comp.Alias, err))
			result.Valid = false
			continue
		}
		if model.Status != ModelStatusPublished {
			result.Errors = append(result.Errors, fmt.Sprintf("component %q is not published", comp.Alias))
			result.Valid = false
		}
		components[comp.Alias] = model
	}
	return components, result
}

// ValidateComposite checks whether a draft composite model can be published.
// Errors block publish; warnings describe risky but valid configurations.
func ValidateComposite(ctx context.Context, composite *CompositeModel, provider ComponentProvider) (*SemanticModel, PublishValidationResult) {
	result := PublishValidationResult{Valid: true}
	addError := func(format string, args ...any) {
		result.Errors = append(result.Errors, fmt.Sprintf(format, args...))
		result.Valid = false
	}
	addWarning := func(format string, args ...any) {
		result.Warnings = append(result.Warnings, fmt.Sprintf(format, args...))
	}

	if strings.TrimSpace(composite.Name) == "" {
		addError("composite name is required")
	}
	if len(composite.Components) < 2 {
		addError("composite model requires at least two component models")
	}

	primaryCount := 0
	aliases := make(map[string]bool, len(composite.Components))
	for _, comp := range composite.Components {
		if aliases[comp.Alias] {
			addError("duplicate component alias %q", comp.Alias)
		}
		aliases[comp.Alias] = true
		if comp.Role == ComponentRolePrimary {
			primaryCount++
		}
	}
	if primaryCount == 0 {
		addError("composite model requires exactly one primary component; none found")
	}
	if primaryCount > 1 {
		addError("composite model requires exactly one primary component; found %d", primaryCount)
	}

	components, compResult := resolveComponents(ctx, composite, provider)
	result.Errors = append(result.Errors, compResult.Errors...)
	if !compResult.Valid {
		result.Valid = false
	}

	for _, j := range composite.CrossModelJoins {
		if !j.IsActive {
			continue
		}
		if !aliases[j.FromModel] {
			addError("cross join %q references unknown alias %q", j.Name, j.FromModel)
		}
		if !aliases[j.ToModel] {
			addError("cross join %q references unknown alias %q", j.Name, j.ToModel)
		}
		if from, ok := components[j.FromModel]; ok && !dimensionExists(from, j.FromDimension) {
			addError("cross join %q references unknown dimension %q on %q", j.Name, j.FromDimension, j.FromModel)
		}
		if to, ok := components[j.ToModel]; ok && !dimensionExists(to, j.ToDimension) {
			addError("cross join %q references unknown dimension %q on %q", j.Name, j.ToDimension, j.ToModel)
		}
	}

	if composite.CanonicalDate != nil {
		ref := composite.CanonicalDate
		if !aliases[ref.ModelAlias] {
			addError("canonical date references unknown alias %q", ref.ModelAlias)
		} else if model, ok := components[ref.ModelAlias]; ok && !dimensionExists(model, ref.DimensionName) {
			addError("canonical date references unknown dimension %q on %q", ref.DimensionName, ref.ModelAlias)
		}
	} else {
		addWarning("no canonical date defined; cross-domain time filtering may be ambiguous")
	}

	if !result.Valid {
		return nil, result
	}

	resolver := NewCompositeResolver()
	resolved, err := resolver.Resolve(composite, components)
	if err != nil {
		addError("resolve composite: %s", err)
		return nil, result
	}

	graph := BuildMetricGraph(composite, resolved)
	if err := DetectCircularDependencies(graph); err != nil {
		addError("metric dependency: %s", err)
	}

	for _, j := range activeCrossJoins(composite.CrossModelJoins) {
		switch j.Relationship {
		case RelationshipManyToMany:
			addWarning("cross join %q is many_to_many; aggregated metrics may fan out and double-count", j.Name)
		case RelationshipOneToMany:
			addWarning("cross join %q is one_to_many; verify metric grain to avoid fanout", j.Name)
		}
	}

	result.EstimatedPromptSize = estimatePromptSize(*resolved)
	return resolved, result
}

func dimensionExists(model *SemanticModel, name string) bool {
	for i := range model.Dimensions {
		if model.Dimensions[i].Name == name {
			return true
		}
	}
	return false
}

func activeCrossJoins(joins []CrossModelJoin) []CrossModelJoin {
	out := make([]CrossModelJoin, 0, len(joins))
	for _, j := range joins {
		if j.IsActive {
			out = append(out, j)
		}
	}
	return out
}

// ValidateComposite loads the full composite then validates it.
func (r *CompositeRepository) ValidateComposite(ctx context.Context, id string, provider ComponentProvider) (*SemanticModel, PublishValidationResult, error) {
	composite, err := r.GetFullComposite(ctx, id)
	if err != nil {
		return nil, PublishValidationResult{}, fmt.Errorf("validate composite: %w", err)
	}
	resolved, validation := ValidateComposite(ctx, composite, provider)
	if errs := r.enforceLimits(composite, resolved); len(errs) > 0 {
		validation.Errors = append(validation.Errors, errs...)
		validation.Valid = false
	}
	return resolved, validation, nil
}

// PublishComposite validates and publishes a composite model, snapshotting the
// resolved semantic context for query runtime.
func (r *CompositeRepository) PublishComposite(ctx context.Context, id, publishedBy string, provider ComponentProvider) (*CompositePublishResult, error) {
	composite, err := r.GetFullComposite(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("publish composite: %w", err)
	}
	resolved, validation := ValidateComposite(ctx, composite, provider)
	if errs := r.enforceLimits(composite, resolved); len(errs) > 0 {
		validation.Errors = append(validation.Errors, errs...)
		validation.Valid = false
	}
	if !validation.Valid {
		return &CompositePublishResult{Composite: composite, Validation: validation, Version: composite.Version}, nil
	}

	nextVersion := composite.Version + 1
	if nextVersion <= 0 {
		nextVersion = 1
	}

	snapshot := struct {
		Composite *CompositeModel `json:"composite"`
		Resolved  *SemanticModel  `json:"resolved"`
	}{Composite: composite, Resolved: resolved}
	payload, err := json.Marshal(snapshot)
	if err != nil {
		return nil, fmt.Errorf("publish composite marshal context: %w", err)
	}
	validationPayload, err := json.Marshal(validation)
	if err != nil {
		return nil, fmt.Errorf("publish composite marshal validation: %w", err)
	}

	if err := platformdb.RunInTx(ctx, r.db, func(tx *sql.Tx) error {
		return r.writeCompositeVersionTx(ctx, tx, id, nextVersion, payload, validationPayload, publishedBy)
	}); err != nil {
		return nil, fmt.Errorf("publish composite: %w", err)
	}

	published, err := r.GetFullComposite(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("publish composite reload: %w", err)
	}
	if r.cache != nil {
		r.cache.Invalidate(ctx, id)
	}
	return &CompositePublishResult{Composite: published, Resolved: resolved, Validation: validation, Version: nextVersion}, nil
}

// RollbackComposite restores a previously published composite version.
func (r *CompositeRepository) RollbackComposite(ctx context.Context, id string, targetVersion int, publishedBy string) (*CompositePublishResult, error) {
	current, err := r.GetComposite(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("rollback composite: %w", err)
	}
	if targetVersion <= 0 {
		targetVersion = current.Version - 1
	}
	if targetVersion <= 0 {
		return nil, errors.New("no previous published context to roll back to")
	}
	payload, err := r.compositeSnapshotByVersion(ctx, id, targetVersion)
	if err != nil {
		return nil, fmt.Errorf("rollback composite snapshot: %w", err)
	}
	nextVersion := current.Version + 1
	validation := PublishValidationResult{Valid: true, Warnings: []string{fmt.Sprintf("rolled back from version %d to version %d", current.Version, targetVersion)}}
	validationPayload, err := json.Marshal(validation)
	if err != nil {
		return nil, fmt.Errorf("rollback composite marshal validation: %w", err)
	}
	if err := platformdb.RunInTx(ctx, r.db, func(tx *sql.Tx) error {
		return r.writeCompositeVersionTx(ctx, tx, id, nextVersion, payload, validationPayload, publishedBy)
	}); err != nil {
		return nil, fmt.Errorf("rollback composite: %w", err)
	}
	published, err := r.GetFullComposite(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("rollback composite reload: %w", err)
	}
	if r.cache != nil {
		r.cache.Invalidate(ctx, id)
	}
	return &CompositePublishResult{Composite: published, Validation: validation, Version: nextVersion}, nil
}

func (r *CompositeRepository) writeCompositeVersionTx(
	ctx context.Context,
	tx *sql.Tx,
	compositeID string,
	version int,
	contextPayload, validationPayload []byte,
	publishedBy string,
) error {
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO composite_context_snapshots (composite_id, version, context, validation_result, created_by)
		VALUES ($1, $2, $3::jsonb, $4::jsonb, NULLIF($5, ''))
	`, compositeID, version, contextPayload, validationPayload, publishedBy); err != nil {
		return fmt.Errorf("insert composite snapshot: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE composite_models
		SET status = 'published',
		    version = $2,
		    published_at = now(),
		    published_by = NULLIF($3, ''),
		    updated_at = now()
		WHERE id = $1::uuid
	`, compositeID, version, publishedBy); err != nil {
		return fmt.Errorf("update composite: %w", err)
	}
	return nil
}

func (r *CompositeRepository) compositeSnapshotByVersion(ctx context.Context, compositeID string, version int) ([]byte, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT context
		FROM composite_context_snapshots
		WHERE composite_id = $1::uuid AND version = $2
	`, compositeID, version)
	var raw []byte
	if err := row.Scan(&raw); err != nil {
		return nil, fmt.Errorf("composite snapshot by version scan: %w", err)
	}
	return raw, nil
}

// GetPublishedResolvedComposite returns the merged SemanticModel captured in the
// latest published snapshot of a composite. Used by the query runtime to compile
// queries against a composite model without re-resolving components on every run.
func (r *CompositeRepository) GetPublishedResolvedComposite(ctx context.Context, compositeID string) (*SemanticModel, error) {
	if r.cache != nil {
		if cached, ok := r.cache.Get(ctx, compositeID); ok {
			return cached, nil
		}
	}
	row := r.db.QueryRowContext(ctx, `
		SELECT context, version
		FROM composite_context_snapshots
		WHERE composite_id = $1::uuid
		ORDER BY version DESC
		LIMIT 1
	`, compositeID)
	var raw []byte
	var version int
	if err := row.Scan(&raw, &version); err != nil {
		return nil, fmt.Errorf("get published resolved composite: %w", err)
	}
	var snapshot struct {
		Resolved *SemanticModel `json:"resolved"`
	}
	if err := json.Unmarshal(raw, &snapshot); err != nil {
		return nil, fmt.Errorf("decode composite snapshot: %w", err)
	}
	if snapshot.Resolved == nil {
		return nil, fmt.Errorf("composite %s snapshot has no resolved model", compositeID)
	}
	if r.cache != nil {
		r.cache.Set(ctx, compositeID, version, snapshot.Resolved)
	}
	return snapshot.Resolved, nil
}

// PrefetchResolvedComposite warms the resolved-composite cache for the given
// composite, typically called for frequently-used composites at startup.
func (r *CompositeRepository) PrefetchResolvedComposite(ctx context.Context, compositeID string) error {
	_, err := r.GetPublishedResolvedComposite(ctx, compositeID)
	return err
}
