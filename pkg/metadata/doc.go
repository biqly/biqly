// Package metadata exposes the public-facing datasource and schema-cache
// DTOs that any service uses to describe what databases, tables, columns and
// foreign-key relationships exist in the catalog.
//
// This package is data-only. Behaviours that require connections,
// encryption or business logic — RuntimeDSN composition, repositories,
// embedding maintenance — live in internal/metadata where they belong to
// the monolith / catalog service.
//
// The legacy import path "github.com/biqly/biqly/internal/metadata"
// re-exports every type and constant here via Go type aliases so existing
// callers continue to compile unchanged.
package metadata
