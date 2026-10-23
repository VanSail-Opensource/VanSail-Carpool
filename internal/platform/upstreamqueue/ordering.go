package upstreamqueue

import "context"

type orderingContextKey struct{}

// WithKey serializes calls for one resource within a provider queue. Unrelated
// keys may advance past a waiting call while preserving global provider pacing.
func WithKey(ctx context.Context, key string) context.Context {
	return context.WithValue(ctx, orderingContextKey{}, key)
}

func orderingKey(ctx context.Context) string {
	key, _ := ctx.Value(orderingContextKey{}).(string)
	return key
}
