package iplookup

import "context"

type deferBlockKey struct{}
type qualityScoresKey struct{}

func withQualityScores(ctx context.Context, scores *NetworkScores) context.Context {
	return context.WithValue(ctx, qualityScoresKey{}, scores)
}
