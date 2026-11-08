package xnode

import (
	"context"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
)

type usageBreakdownRepository interface {
	XNodeUsageBreakdown(context.Context, string, time.Time, time.Time) (model.UsageBreakdown, error)
}

// OwnedUsageBreakdown reads the authenticated owner's consistent projection.
// It never derives charges from current node prices or fabricates empty history
// when the repository cannot supply the required accounting projection.
func (s *Service) OwnedUsageBreakdown(ctx context.Context, owner string) (model.UsageBreakdown, error) {
	if owner == "" {
		return model.UsageBreakdown{}, ErrInvalid
	}
	repo, ok := s.repo.(usageBreakdownRepository)
	if !ok {
		return model.UsageBreakdown{}, ErrUnavailable
	}
	through := s.now().UTC()
	from := through.Add(-PAYGWindow)
	return repo.XNodeUsageBreakdown(ctx, owner, from, through)
}
