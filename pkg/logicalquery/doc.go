// Package logicalquery defines the database-independent LogicalQuery types
// produced by the AI layer and consumed by the Query Engine compiler.
//
// This package contains only data types and pure helpers — no I/O, no
// validation that requires a semantic model, no compilation. It is the
// "stable wire" form of a query: serialize a LogicalQuery to JSON in one
// service, deserialize and compile it in another.
//
// Peer services and SDKs import this package directly to construct, inspect
// or transform LogicalQuery payloads. The legacy import path
// "github.com/biqly/biqly/internal/query" continues to expose every type and
// constant here via Go type aliases, so existing callers do not need to
// migrate immediately.
package logicalquery
