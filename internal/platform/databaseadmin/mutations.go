package databaseadmin

import (
	"context"
	"fmt"
	"regexp"
)

var decimalPattern = regexp.MustCompile(`^[+-]?(?:[0-9]+(?:\.[0-9]+)?|\.[0-9]+)$`)

// PostgreSQL administration is read-only. Typed domain commands own all writes.
func (s *Service) ReviewMutation(context.Context, string, MutationRequest) (MutationReview, error) {
	return MutationReview{}, fmt.Errorf("%w: PostgreSQL administration is read-only", ErrInvalidValue)
}
func (s *Service) ApplyMutation(context.Context, string, MutationRequest) (MutationResult, error) {
	return MutationResult{}, fmt.Errorf("%w: PostgreSQL administration is read-only", ErrInvalidValue)
}
