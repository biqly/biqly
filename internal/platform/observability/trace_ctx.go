package observability

import "context"

type (
	queryFingerprintKey struct{}
	dbSystemKey         struct{}
)

// WithDBSystem attaches a datasource driver type (postgres, mysql, …) for downstream spans.
func WithDBSystem(ctx context.Context, system string) context.Context {
	if system == "" {
		return ctx
	}
	return context.WithValue(ctx, dbSystemKey{}, system)
}

// DBSystem returns the driver type stored by WithDBSystem.
func DBSystem(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	sys, _ := ctx.Value(dbSystemKey{}).(string)
	return sys
}

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
