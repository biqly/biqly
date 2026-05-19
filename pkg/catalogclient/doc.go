// Package catalogclient is the typed Go client for the /internal/* surface
// exposed by the Catalog Service (the Biqly monolith in Phase 1, a standalone
// binary from Phase 2 onwards — see docs/microservice-decomposition.md).
//
// The client is intentionally thin: it owns transport (HTTP + JSON), error
// translation (internalapi.Error → typed APIError) and a small request
// vocabulary. It deliberately does NOT cache, retry forever, or hide HTTP
// semantics; callers that need an in-memory cache layer compose one above
// the client.
//
// Usage:
//
//	c := catalogclient.New("http://catalog:8888",
//	    catalogclient.WithHTTPClient(&http.Client{Timeout: 5 * time.Second}),
//	    catalogclient.WithAuthToken(os.Getenv("BI_INTERNAL_API_TOKEN")),
//	)
//	ds, err := c.GetDatasource(ctx, "ds_42")
//	if errors.Is(err, catalogclient.ErrNotFound) { ... }
//
// All read methods are safe to call concurrently. Write methods (history)
// are safe too; the server guarantees append-only semantics.
//
// The wire format lives in pkg/internalapi; bumping that package is a
// breaking change for every caller of this client.
package catalogclient
