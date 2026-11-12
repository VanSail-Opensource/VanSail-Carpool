package iplookup

import "context"

type runLeaseKey struct{}

// WithRunLease binds a background execution to its durable report lease. Every
// repository mutation and derived provider context must retain this value.
func WithRunLease(ctx context.Context, owner string) context.Context {
	return context.WithValue(ctx, runLeaseKey{}, owner)
}

// RunLeaseOwner returns the execution fence. An empty value may only mutate a
// report that has never been leased, as in synchronous worker integrations.
func RunLeaseOwner(ctx context.Context) string {
	owner, _ := ctx.Value(runLeaseKey{}).(string)
	return owner
}
