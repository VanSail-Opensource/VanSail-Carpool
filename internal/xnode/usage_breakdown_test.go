package xnode

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
)

type breakdownRepositoryFixture struct {
	Repository
	owner         string
	from, through time.Time
	ctx           context.Context
	result        model.UsageBreakdown
	err           error
}

func (r *breakdownRepositoryFixture) XNodeUsageBreakdown(ctx context.Context, owner string, from, through time.Time) (model.UsageBreakdown, error) {
	r.ctx, r.owner, r.from, r.through = ctx, owner, from, through
	return r.result, r.err
}

func TestOwnedUsageBreakdownPreservesOwnerUTCWindowAndExactProjection(t *testing.T) {
	now := time.Date(2026, 11, 9, 7, 31, 45, 987654321, time.FixedZone("fixture", 8*60*60))
	repo := &breakdownRepositoryFixture{result: model.UsageBreakdown{Cost: model.UsageBreakdownCost{
		ActualMinor: "9007199254740993", KnownTotalMinor: model.RationalMinor{Numerator: "27021597764222980", Denominator: "3"},
	}}}
	s := NewService(repo, nil, nil, nil, nil)
	s.now = func() time.Time { return now }
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	view, err := s.OwnedUsageBreakdown(ctx, "current-owner")
	if err != nil || repo.owner != "current-owner" || repo.ctx != ctx || !repo.through.Equal(now) || repo.through.Location() != time.UTC || !repo.from.Equal(now.UTC().Add(-PAYGWindow)) {
		t.Fatal("owner, cancellation or UTC window changed", repo.owner, repo.from, repo.through, err)
	}
	if view.Cost.ActualMinor != repo.result.Cost.ActualMinor || view.Cost.KnownTotalMinor != repo.result.Cost.KnownTotalMinor {
		t.Fatal("presentation rounded exact monetary facts", view.Cost)
	}
	repo.err = context.Canceled
	if _, err = s.OwnedUsageBreakdown(ctx, "current-owner"); !errors.Is(err, context.Canceled) {
		t.Fatal("storage failure was represented as empty history", err)
	}
}

func TestOwnedUsageBreakdownRefusesUnknownOwnerOrUnsupportedRepository(t *testing.T) {
	s := NewService(nil, nil, nil, nil, nil)
	if _, err := s.OwnedUsageBreakdown(context.Background(), ""); !errors.Is(err, ErrInvalid) {
		t.Fatal(err)
	}
	if _, err := s.OwnedUsageBreakdown(context.Background(), "owner"); !errors.Is(err, ErrUnavailable) {
		t.Fatal("missing projection fabricated complete empty history", err)
	}
}
