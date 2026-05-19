// Package security exports the public-facing access-control DTOs that any
// service needs to describe a user's data permissions: which models they may
// query, which fields are denied, and which row-level filters must be injected
// into every generated query.
//
// This package contains data only — no encryption helpers, no permission
// manager, no read-only SQL checker. Those behaviours stay in
// internal/security where they belong to the monolith.
//
// The legacy import path "github.com/biqly/biqly/internal/security" re-exports
// every type here via Go type aliases so existing callers continue to compile.
package security
