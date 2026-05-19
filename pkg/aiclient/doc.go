// Package aiclient is the typed Go client for the public /api/ai/* surface
// exposed by the AI Service (the Biqly monolith in Phase 1, a standalone
// binary from Phase 2 onwards — see docs/microservice-decomposition.md).
//
// The client is intentionally thin: it owns transport (HTTP + JSON), error
// translation (legacy {"error"} and internalapi.Error envelopes → typed
// APIError), and clarification detection (HTTP 200 with needs_clarification
// → ErrNeedsClarification). It does NOT cache, retry, or hide HTTP semantics.
//
// Usage:
//
//	c := aiclient.New("http://ai:8888",
//	    aiclient.WithHTTPClient(&http.Client{Timeout: 120 * time.Second}),
//	    aiclient.WithAuthToken(os.Getenv("BI_API_TOKEN")),
//	    aiclient.WithCaller("query"),
//	)
//	resp, err := c.Query(ctx, aiclient.QueryRequest{
//	    DatasourceID: "ds_1", Question: "total revenue last month",
//	})
//	if errors.Is(err, aiclient.ErrNeedsClarification) { ... }
//
// v0 covers query, preview, run, describe, embed, and settings. Examples,
// feedback, glossary, usage, and eval endpoints are listed in client.go.
package aiclient
