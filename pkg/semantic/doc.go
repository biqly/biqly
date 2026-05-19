// Package semantic exposes the public-facing semantic-layer DTOs that
// services exchange when describing business-friendly views over physical
// tables: models, dimensions, metrics and joins.
//
// This package is data-only. Behavioural helpers such as MetricRegistry,
// publish/rollback workflow and budget enforcement live in internal/semantic
// because they are monolith-scoped today.
//
// The legacy import path "github.com/biqly/biqly/internal/semantic"
// re-exports every type and constant here via Go type aliases so existing
// callers continue to compile unchanged.
package semantic
