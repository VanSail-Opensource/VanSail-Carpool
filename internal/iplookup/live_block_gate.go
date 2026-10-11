package iplookup

import "context"

type deferBlockKey struct{}
type qualityScoresKey struct{}

func withQualityScores(ctx context.Context, scores *NetworkScores) context.Context {
	return context.WithValue(ctx, qualityScoresKey{}, scores)
}

func withoutBlock(ctx context.Context) context.Context {
	return context.WithValue(ctx, deferBlockKey{}, true)
}
