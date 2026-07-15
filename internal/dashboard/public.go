package dashboard

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/biqly/biqly/pkg/logicalquery"
	"github.com/bytedance/sonic"
)

// ErrShareNotFound is the single error every anonymous-path failure collapses
// to, so handlers can return one uniform 404 without leaking which check failed.
var ErrShareNotFound = errors.New("public share not found")

// PublicDashboardView is the anonymous read model: dashboard with widgets
// already sanitized (no logical_query / saved_query_id).
type PublicDashboardView struct {
	Dashboard *Dashboard
	Share     *PublicShare
}

// PublicWidgetQuery is the server-side execution input for one widget.
type PublicWidgetQuery struct {
	WorkspaceID  string
	LogicalQuery *logicalquery.LogicalQuery
}

// SanitizeWidgets strips query internals from the widget config so the
// anonymous client only ever sees render configuration.
func SanitizeWidgets(raw json.RawMessage) (json.RawMessage, error) {
	if len(raw) == 0 {
		return json.RawMessage(`[]`), nil
	}
	var widgets []map[string]any
	if err := sonic.Unmarshal(raw, &widgets); err != nil {
		return nil, fmt.Errorf("parse widgets: %w", err)
	}
	for _, w := range widgets {
		delete(w, "logical_query")
		delete(w, "saved_query_id")
	}
	out, err := sonic.Marshal(widgets)
	if err != nil {
		return nil, fmt.Errorf("marshal sanitized widgets: %w", err)
	}
	return out, nil
}

// PublicResolver resolves share tokens to dashboards/widget queries. Both the
// catalog service (metadata) and the query service (execution) use it against
// their own bi_metadata pools.
type PublicResolver struct {
	shares *ShareRepository
	dashes *Repository
}

// NewPublicResolver builds a resolver over a bi_metadata connection.
func NewPublicResolver(db *sql.DB) *PublicResolver {
	return &PublicResolver{shares: NewShareRepository(db), dashes: NewRepository(db)}
}

func (r *PublicResolver) resolve(ctx context.Context, plainToken string) (*Dashboard, *PublicShare, error) {
	share, err := r.shares.FindActiveByTokenHash(ctx, HashShareToken(plainToken))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil, ErrShareNotFound
	} else if err != nil {
		return nil, nil, err
	}
	d, err := r.dashes.Get(ctx, share.DashboardID, share.WorkspaceID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil, ErrShareNotFound
		}
		return nil, nil, err
	}
	return d, share, nil
}

// ResolveDashboard returns the sanitized dashboard for a share token.
func (r *PublicResolver) ResolveDashboard(ctx context.Context, plainToken string) (*PublicDashboardView, error) {
	d, share, err := r.resolve(ctx, plainToken)
	if err != nil {
		return nil, err
	}
	sanitized, err := SanitizeWidgets(d.Widgets)
	if err != nil {
		return nil, err
	}
	d.Widgets = sanitized
	return &PublicDashboardView{Dashboard: d, Share: share}, nil
}

// ResolveWidgetQuery returns the stored logical query for one widget of a
// shared dashboard. Widgets without a stored query (text) are not found.
func (r *PublicResolver) ResolveWidgetQuery(ctx context.Context, plainToken, widgetID string) (*PublicWidgetQuery, error) {
	d, share, err := r.resolve(ctx, plainToken)
	if err != nil {
		return nil, err
	}
	var widgets []struct {
		ID           string                     `json:"id"`
		LogicalQuery *logicalquery.LogicalQuery `json:"logical_query"`
	}
	if err := sonic.Unmarshal(d.Widgets, &widgets); err != nil {
		return nil, fmt.Errorf("parse widgets: %w", err)
	}
	for _, w := range widgets {
		if w.ID == widgetID && w.LogicalQuery != nil {
			return &PublicWidgetQuery{WorkspaceID: share.WorkspaceID, LogicalQuery: w.LogicalQuery}, nil
		}
	}
	return nil, ErrShareNotFound
}
