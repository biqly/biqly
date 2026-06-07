package observability

import "context"

type queryFingerprintKey struct{}

// WithQueryFingerprint attaches a logical-query fingerprint to ctx for downstream
// spans (e.g. query.Execute) without threading it through every function param.
func WithQueryFingerprint(ctx context.Context, fingerprint string) context.Context {
	if fingerprint == "" {
		return ctx
	}
	return context.WithValue(ctx, queryFingerprintKey{}, fingerprint)
}

// QueryFingerprint returns the fingerprint stored by WithQueryFingerprint.
func QueryFingerprint(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	fp, _ := ctx.Value(queryFingerprintKey{}).(string)
	return fp
}
