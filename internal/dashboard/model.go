package dashboard

import (
	"encoding/json"
	"time"
)

// Dashboard represents a user custom dashboard populated with widgets.
type Dashboard struct {
	ID          string          `json:"id"`
	WorkspaceID *string         `json:"workspace_id,omitempty"`
	Name        string          `json:"name"`
	Description *string         `json:"description,omitempty"`
	Widgets     json.RawMessage `json:"widgets"` // JSON array of widgets configuration
	CreatedAt   time.Time       `json:"created_at"`
	UpdatedAt   time.Time       `json:"updated_at"`
}
