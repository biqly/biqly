// Package errors defines the canonical Go-side ServiceError type used to
// carry an HTTP status, a stable machine-readable code and a human-readable
// message across the service boundary.
//
// Wire format: every internal-API response that has HTTP status >= 400
// returns a pkg/internalapi.Error JSON envelope. A ServiceError is the Go
// counterpart of that envelope plus the originating HTTP status — useful
// inside handlers (returning typed errors that round-trip cleanly to JSON)
// and inside clients (mapping an HTTP response to a structured Go error
// before exposing a sentinel via errors.Is).
//
// Why not extend pkg/internalapi/errors.go directly? Because that package
// must stay JSON-only — no HTTP behaviour, no error interface — so it can
// be vendored by language-agnostic clients. This package layers Go-specific
// helpers on top.
package errors
