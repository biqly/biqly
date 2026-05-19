// Package queryclient is the typed Go client for the /internal/query/*
// surface exposed by the Query Engine (the Biqly monolith in Phase 1, a
// standalone binary from Phase 3 onwards — see
// docs/microservice-decomposition.md).
//
// Usage:
//
//	c := queryclient.New("http://query-engine:8888",
//	    queryclient.WithHTTPClient(&http.Client{Timeout: 30 * time.Second}),
//	    queryclient.WithAuthToken(os.Getenv("BI_INTERNAL_API_TOKEN")),
//	)
//	out, err := c.Run(ctx, lq)
//	if errors.Is(err, queryclient.ErrInvalidRequest) { ... }
//
// The transport plumbing here intentionally mirrors pkg/catalogclient — both
// will share a private helper package once a second non-monolith binary is
// extracted (Phase 2/3). Until then, the duplication keeps Phase 1 changes
// surgical.
package queryclient
