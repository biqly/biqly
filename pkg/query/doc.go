// Package query defines the public-facing result, compiled-SQL and history
// types that the Query Engine and Catalog services exchange with peer
// services and SDKs.
//
// This package is data-only. It depends on pkg/logicalquery for the wire
// shape of a query but does not import any internal/ package, so any binary
// (AI Service, Frontend BFF, third-party SDK) can pull it in without
// dragging in the monolith's compiler, executor or repository code.
//
// The legacy import path "github.com/biqly/biqly/internal/query" re-exports
// every type and constant here via Go type aliases for backward compatibility.
package query
