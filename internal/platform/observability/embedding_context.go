package observability

import "context"

type embeddingOperationKey struct{}

// ContextWithEmbeddingOperation tags ctx with a bounded embedding operation label.
func ContextWithEmbeddingOperation(ctx context.Context, operation string) context.Context {
	return context.WithValue(ctx, embeddingOperationKey{}, operation)
}

// EmbeddingOperationFromContext reads the embedding operation label from ctx.
func EmbeddingOperationFromContext(ctx context.Context) string {
	if v, ok := ctx.Value(embeddingOperationKey{}).(string); ok {
		return v
	}
	return "other"
}
