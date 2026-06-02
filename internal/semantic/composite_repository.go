package semantic

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/google/uuid"

	platformdb "github.com/biqly/biqly/internal/platform/db"
)

// CompositeRepository handles persistence for composite semantic models.
type CompositeRepository struct {
	db     *sql.DB
	cache  ResolvedCompositeCache
	limits CompositeLimits
}

// NewCompositeRepository creates a new composite repository.
func NewCompositeRepository(db *sql.DB) *CompositeRepository {
	return &CompositeRepository{db: db}
}

// WithResolvedCache attaches a cache for published resolved composites. A nil
// cache leaves caching disabled, so callers may wire it unconditionally.
func (r *CompositeRepository) WithResolvedCache(cache ResolvedCompositeCache) *CompositeRepository {
	r.cache = cache
	return r
}

// CreateComposite inserts a composite model header.
func (r *CompositeRepository) CreateComposite(ctx context.Context, c *CompositeModel) error {
	if c.ID == "" {
		c.ID = uuid.NewString()
	}
	if c.Status == "" {
		c.Status = ModelStatusDraft
	}
	canonical, err := marshalCanonicalDate(c.CanonicalDate)
	if err != nil {
		return err
	}
	query := `
		INSERT INTO composite_models (id, datasource_id, name, label, description, canonical_date, is_active, status, version, created_by)
		VALUES ($1, $2, $3, $4, $5, $6::jsonb, $7, $8, 0, $9)
	`
	if err := r.db.QueryRowContext(ctx, query, c.ID, c.DatasourceID, c.Name, c.Label, c.Description, canonical, c.IsActive, c.Status, c.CreatedBy).Err(); err != nil {
		return fmt.Errorf("create composite: %w", err)
	}
	return nil
}

// GetComposite retrieves a composite header by ID.
func (r *CompositeRepository) GetComposite(ctx context.Context, id string) (*CompositeModel, error) {
	query := compositeSelectSQL() + ` WHERE id = $1::uuid` //nolint:gosec // nosemgrep: go.lang.security.audit.database.string-formatted-query.string-formatted-query
	row := r.db.QueryRowContext(ctx, query, id)
	return scanComposite(row)
}

// ListComposites returns composite models, optionally filtered by datasource.
func (r *CompositeRepository) ListComposites(ctx context.Context, datasourceID string) ([]CompositeModel, error) {
	query := compositeSelectSQL()
	var args []any
	if datasourceID != "" {
		query += " WHERE datasource_id = $1::uuid"
		args = append(args, datasourceID)
	}
	query += " ORDER BY created_at DESC"
	ptrs, err := platformdb.QuerySliceErr(ctx, r.db, "list composites", query, args, scanComposite)
	if err != nil {
		return nil, err
	}
	out := make([]CompositeModel, len(ptrs))
	for i, p := range ptrs {
		out[i] = *p
	}
	return out, nil
}

// UpdateComposite updates a composite header and marks it draft.
func (r *CompositeRepository) UpdateComposite(ctx context.Context, c *CompositeModel) error {
	canonical, err := marshalCanonicalDate(c.CanonicalDate)
	if err != nil {
		return err
	}
	query := `
		UPDATE composite_models
		SET name = $2, label = $3, description = $4, canonical_date = $5::jsonb, is_active = $6,
		    status = 'draft', draft_updated_at = now(), updated_at = now()
		WHERE id = $1::uuid
	`
	if _, err := r.db.ExecContext(ctx, query, c.ID, c.Name, c.Label, c.Description, canonical, c.IsActive); err != nil {
		return fmt.Errorf("update composite: %w", err)
	}
	return nil
}

// DeleteComposite removes a composite model (cascades to children).
func (r *CompositeRepository) DeleteComposite(ctx context.Context, id string) error {
	if _, err := r.db.ExecContext(ctx, `DELETE FROM composite_models WHERE id = $1::uuid`, id); err != nil {
		return fmt.Errorf("delete composite: %w", err)
	}
	return nil
}

// GetFullComposite loads a composite header together with its components,
// cross-model joins and dimension conflict resolutions.
func (r *CompositeRepository) GetFullComposite(ctx context.Context, id string) (*CompositeModel, error) {
	composite, err := r.GetComposite(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("get full composite: %w", err)
	}

	var (
		components  []ComponentModelRef
		crossJoins  []CrossModelJoin
		resolutions []DimensionConflictResolution
		compErr     error
		joinErr     error
		resErr      error
	)
	var wg sync.WaitGroup
	wg.Add(3)
	go func() {
		defer wg.Done()
		components, compErr = r.GetComponents(ctx, id)
	}()
	go func() {
		defer wg.Done()
		crossJoins, joinErr = r.GetCrossModelJoins(ctx, id)
	}()
	go func() {
		defer wg.Done()
		resolutions, resErr = r.GetDimensionResolutions(ctx, id)
	}()
	wg.Wait()

	switch {
	case compErr != nil:
		return nil, fmt.Errorf("get components: %w", compErr)
	case joinErr != nil:
		return nil, fmt.Errorf("get cross joins: %w", joinErr)
	case resErr != nil:
		return nil, fmt.Errorf("get dimension resolutions: %w", resErr)
	}

	composite.Components = components
	composite.CrossModelJoins = crossJoins
	composite.ConflictResolutions = resolutions
	return composite, nil
}

// GetComponents returns the component model references for a composite.
func (r *CompositeRepository) GetComponents(ctx context.Context, compositeID string) ([]ComponentModelRef, error) {
	query := `SELECT id::text, composite_id::text, model_id::text, alias, role, created_at FROM composite_model_components WHERE composite_id = $1::uuid ORDER BY role DESC, alias`
	return platformdb.QuerySliceErr(ctx, r.db, "get composite components", query, []any{compositeID}, scanComponent)
}

// AddComponent attaches a component model to a composite.
func (r *CompositeRepository) AddComponent(ctx context.Context, compositeID string, ref ComponentModelRef) error {
	if ref.Role == "" {
		ref.Role = ComponentRoleSecondary
	}
	query := `INSERT INTO composite_model_components (id, composite_id, model_id, alias, role) VALUES ($1, $2, $3, $4, $5)`
	if err := r.db.QueryRowContext(ctx, query, uuid.NewString(), compositeID, ref.ModelID, ref.Alias, ref.Role).Err(); err != nil {
		return fmt.Errorf("add component: %w", err)
	}
	return r.markDraft(ctx, compositeID)
}

// RemoveComponent detaches a component model from a composite.
func (r *CompositeRepository) RemoveComponent(ctx context.Context, compositeID, modelID string) error {
	if _, err := r.db.ExecContext(ctx, `DELETE FROM composite_model_components WHERE composite_id = $1::uuid AND model_id = $2::uuid`, compositeID, modelID); err != nil {
		return fmt.Errorf("remove component: %w", err)
	}
	return r.markDraft(ctx, compositeID)
}

// GetCrossModelJoins returns the active cross-model joins for a composite.
func (r *CompositeRepository) GetCrossModelJoins(ctx context.Context, compositeID string) ([]CrossModelJoin, error) {
	query := `SELECT id::text, composite_id::text, name, from_alias, from_dimension, to_alias, to_dimension, join_type, relationship, is_active, created_at FROM composite_cross_model_joins WHERE composite_id = $1::uuid ORDER BY name`
	return platformdb.QuerySliceErr(ctx, r.db, "get cross model joins", query, []any{compositeID}, scanCrossModelJoin)
}

// AddCrossModelJoin inserts a cross-model join.
func (r *CompositeRepository) AddCrossModelJoin(ctx context.Context, compositeID string, j CrossModelJoin) error {
	if j.JoinType == "" {
		j.JoinType = DefaultJoinType
	}
	if j.Relationship == "" {
		j.Relationship = DefaultRelationshipType
	}
	query := `INSERT INTO composite_cross_model_joins (id, composite_id, name, from_alias, from_dimension, to_alias, to_dimension, join_type, relationship, is_active)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)`
	if err := r.db.QueryRowContext(ctx, query, uuid.NewString(), compositeID, j.Name, j.FromModel, j.FromDimension, j.ToModel, j.ToDimension, j.JoinType, j.Relationship, j.IsActive).Err(); err != nil {
		return fmt.Errorf("add cross model join: %w", err)
	}
	return r.markDraft(ctx, compositeID)
}

// UpdateCrossModelJoin updates an existing cross-model join.
func (r *CompositeRepository) UpdateCrossModelJoin(ctx context.Context, compositeID string, j CrossModelJoin) error {
	query := `UPDATE composite_cross_model_joins
		SET name = $3, from_alias = $4, from_dimension = $5, to_alias = $6, to_dimension = $7, join_type = $8, relationship = $9, is_active = $10
		WHERE id = $1::uuid AND composite_id = $2::uuid`
	if _, err := r.db.ExecContext(ctx, query, j.ID, compositeID, j.Name, j.FromModel, j.FromDimension, j.ToModel, j.ToDimension, j.JoinType, j.Relationship, j.IsActive); err != nil {
		return fmt.Errorf("update cross model join: %w", err)
	}
	return r.markDraft(ctx, compositeID)
}

// RemoveCrossModelJoin deletes a cross-model join.
func (r *CompositeRepository) RemoveCrossModelJoin(ctx context.Context, compositeID, joinID string) error {
	if _, err := r.db.ExecContext(ctx, `DELETE FROM composite_cross_model_joins WHERE id = $1::uuid AND composite_id = $2::uuid`, joinID, compositeID); err != nil {
		return fmt.Errorf("remove cross model join: %w", err)
	}
	return r.markDraft(ctx, compositeID)
}

// GetDimensionResolutions returns the dimension conflict resolutions for a composite.
func (r *CompositeRepository) GetDimensionResolutions(ctx context.Context, compositeID string) ([]DimensionConflictResolution, error) {
	query := `SELECT id::text, composite_id::text, dimension_name, resolution, source_alias, target_alias FROM composite_dimension_resolutions WHERE composite_id = $1::uuid ORDER BY dimension_name`
	return platformdb.QuerySliceErr(ctx, r.db, "get dimension resolutions", query, []any{compositeID}, scanDimensionResolution)
}

// SetDimensionResolution upserts a dimension conflict resolution.
func (r *CompositeRepository) SetDimensionResolution(ctx context.Context, compositeID string, res DimensionConflictResolution) error {
	query := `
		INSERT INTO composite_dimension_resolutions (id, composite_id, dimension_name, resolution, source_alias, target_alias)
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (composite_id, dimension_name, source_alias)
		DO UPDATE SET resolution = EXCLUDED.resolution, target_alias = EXCLUDED.target_alias`
	if _, err := r.db.ExecContext(ctx, query, uuid.NewString(), compositeID, res.DimensionName, res.Resolution, res.SourceAlias, res.TargetAlias); err != nil {
		return fmt.Errorf("set dimension resolution: %w", err)
	}
	return r.markDraft(ctx, compositeID)
}

// SetCanonicalDate updates the canonical date reference for a composite.
func (r *CompositeRepository) SetCanonicalDate(ctx context.Context, compositeID string, ref *CanonicalDateRef) error {
	canonical, err := marshalCanonicalDate(ref)
	if err != nil {
		return err
	}
	if _, err := r.db.ExecContext(ctx, `UPDATE composite_models SET canonical_date = $2::jsonb, status = 'draft', draft_updated_at = now(), updated_at = now() WHERE id = $1::uuid`, compositeID, canonical); err != nil {
		return fmt.Errorf("set canonical date: %w", err)
	}
	return nil
}

func (r *CompositeRepository) markDraft(ctx context.Context, id string) error {
	if _, err := r.db.ExecContext(ctx, `UPDATE composite_models SET status = 'draft', draft_updated_at = now(), updated_at = now() WHERE id = $1::uuid`, id); err != nil {
		return fmt.Errorf("mark composite draft: %w", err)
	}
	return nil
}

func marshalCanonicalDate(ref *CanonicalDateRef) (any, error) {
	if ref == nil {
		return nil, nil
	}
	raw, err := json.Marshal(ref)
	if err != nil {
		return nil, fmt.Errorf("marshal canonical date: %w", err)
	}
	return string(raw), nil
}

func compositeSelectSQL() string {
	return `SELECT id::text, datasource_id::text, name, label, description, canonical_date, is_active,
		status, version, published_at, published_by, draft_updated_at, created_by, created_at, updated_at
		FROM composite_models`
}

func scanComposite(s platformdb.Scanner) (*CompositeModel, error) {
	var c CompositeModel
	var canonical []byte
	if err := s.Scan(&c.ID, &c.DatasourceID, &c.Name, &c.Label, &c.Description, &canonical, &c.IsActive,
		&c.Status, &c.Version, &c.PublishedAt, &c.PublishedBy, &c.DraftUpdatedAt, &c.CreatedBy, &c.CreatedAt, &c.UpdatedAt); err != nil {
		return nil, fmt.Errorf("scan composite: %w", err)
	}
	if len(canonical) > 0 {
		var ref CanonicalDateRef
		if err := json.Unmarshal(canonical, &ref); err == nil && ref.DimensionName != "" {
			c.CanonicalDate = &ref
		}
	}
	return &c, nil
}

func scanComponent(s platformdb.Scanner) (ComponentModelRef, error) {
	var c ComponentModelRef
	if err := s.Scan(&c.ID, &c.CompositeID, &c.ModelID, &c.Alias, &c.Role, &c.CreatedAt); err != nil {
		return c, fmt.Errorf("scan component: %w", err)
	}
	return c, nil
}

func scanCrossModelJoin(s platformdb.Scanner) (CrossModelJoin, error) {
	var j CrossModelJoin
	if err := s.Scan(&j.ID, &j.CompositeID, &j.Name, &j.FromModel, &j.FromDimension, &j.ToModel, &j.ToDimension, &j.JoinType, &j.Relationship, &j.IsActive, &j.CreatedAt); err != nil {
		return j, fmt.Errorf("scan cross model join: %w", err)
	}
	return j, nil
}

func scanDimensionResolution(s platformdb.Scanner) (DimensionConflictResolution, error) {
	var d DimensionConflictResolution
	var source, target sql.NullString
	if err := s.Scan(&d.ID, &d.CompositeID, &d.DimensionName, &d.Resolution, &source, &target); err != nil {
		return d, fmt.Errorf("scan dimension resolution: %w", err)
	}
	d.SourceAlias = source.String
	d.TargetAlias = target.String
	return d, nil
}
