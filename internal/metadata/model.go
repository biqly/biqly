// Package metadata defines types for datasource metadata storage.
package metadata

//revive:disable:exported // alias shim — canonical docs live in pkg/metadata

import pkgmetadata "github.com/biqly/biqly/pkg/metadata"

// Datasource and the sibling aliases below re-export pkg/metadata data
// structures so existing callers continue to import the legacy
// "internal/metadata" path. Behavioural helpers (RuntimeDSN, repositories)
// stay in this package; methods that previously hung off Datasource are
// exposed as free functions because aliases cannot declare new methods.
type (
	Datasource             = pkgmetadata.Datasource
	Schema                 = pkgmetadata.Schema
	Table                  = pkgmetadata.Table
	Column                 = pkgmetadata.Column
	ColumnEmbedding        = pkgmetadata.ColumnEmbedding
	Relation               = pkgmetadata.Relation
	AIQueryHistoryEntry    = pkgmetadata.AIQueryHistoryEntry
	PermissionPolicyRecord = pkgmetadata.PermissionPolicyRecord
	PermissionRowFilter    = pkgmetadata.PermissionRowFilter
)

// Re-exported DSN mode identifiers.
const (
	DSNModeRaw        = pkgmetadata.DSNModeRaw
	DSNModeStructured = pkgmetadata.DSNModeStructured
)
