// Package internalapi defines the wire-format contracts shared between Biqly
// microservices for the /internal/* HTTP API. These are intra-cluster only:
// the Cilium Gateway never matches /internal/*, so the surface is reachable
// solely from peer pods inside the biqly namespace.
//
// Phase 1 of the microservice decomposition (docs/microservice-decomposition.md)
// adds these endpoints to the monolith without disturbing the public /api/*
// surface. Later phases move the handlers into separate binaries (catalog,
// query, ai), but the wire format defined here stays stable.
//
// Conventions:
//   - All endpoints respond with application/json; UTF-8.
//   - Error responses use the Error type with a stable {error, message?} shape.
//   - Lists are returned as JSON arrays, not envelopes ({data:[...]} is avoided
//     so the receiver can stream-decode without an outer object).
//   - Identifiers (datasource_id, model_id, ...) are always lowercase snake_case
//     to match the rest of the API surface.
package internalapi
