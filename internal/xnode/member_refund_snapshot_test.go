package xnode_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
)

type refundSnapshotRepository struct {
	xnode.Repository
	identityReads int
	afterIdentity func(context.Context) error
	afterPeriods  func(context.Context) error
}

func (r *refundSnapshotRepository) XNodeIdentity(ctx context.Context, id int64) (xnode.Identity, error) {
	identity, err := r.Repository.XNodeIdentity(ctx, id)
	r.identityReads++
	if err == nil && r.identityReads == 2 && r.afterIdentity != nil {
		err = r.afterIdentity(ctx)
	}
	return identity, err
}

func (r *refundSnapshotRepository) XNodePeriods(ctx context.Context, id int64) ([]xnode.Period, error) {
	periods, err := r.Repository.XNodePeriods(ctx, id)
	if err == nil && r.afterPeriods != nil {
		err = r.afterPeriods(ctx)
	}
	return periods, err
}

func closedRefundFixture(t *testing.T) (fixture, xnode.Identity, xnode.Period) {
	t.Helper()
	f := controlFixture(t, true)
	ctx := context.Background()
	identity, err := f.store.XNodeIdentityByUser(ctx, f.user.ID)
	if err != nil {
		t.Fatal(err)
	}
	period, err := f.store.XNodePeriod(ctx, identity.CurrentPeriodID)
	if err != nil {
		t.Fatal(err)
	}
	period, err = f.store.BeginXNodePeriodClosure(ctx, period, time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	if err = f.store.FinalizeXNodePeriod(ctx, period); err != nil {
		t.Fatal(err)
	}
	period.Status = "closed"
	return f, identity, period
}

func TestClosedOperationSnapshotRechecksReopenedAdmission(t *testing.T) {
	f, identity, period := closedRefundFixture(t)
	repo := &refundSnapshotRepository{Repository: f.store, afterIdentity: func(ctx context.Context) error {
		return f.store.ReopenXNodePeriod(ctx, period.ID)
	}}
	service := xnode.NewService(repo, nil, nil, nil, nil)
	state, err := service.PurchaseOperationState(context.Background(), stringID(identity), period.PurchaseID)
	if !errors.Is(err, xnode.ErrConflict) || state.Quiesced {
		t.Fatal("closed snapshot bypassed the final admission check", state, err)
	}
}

func TestRefundAggregationRechecksOriginalClosureAfterReopen(t *testing.T) {
	f, identity, period := closedRefundFixture(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	entered, release := make(chan struct{}), make(chan struct{})
	repo := &refundSnapshotRepository{Repository: f.store, afterPeriods: func(ctx context.Context) error {
		close(entered)
		select {
		case <-release:
			return nil
		case <-ctx.Done():
			return ctx.Err()
		}
	}}
	service := xnode.NewService(repo, nil, nil, nil, nil)
	done := make(chan error, 1)
	go func() {
		state, err := service.MemberRefundState(ctx, stringID(identity), period.PurchaseID)
		if state.Quiesced {
			done <- errors.New("stale refund aggregate retained quiesced access")
			return
		}
		done <- err
	}()
	select {
	case <-entered:
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
	if err := f.store.ReopenXNodePeriod(ctx, period.ID); err != nil {
		t.Fatal(err)
	}
	close(release)
	select {
	case err := <-done:
		if !errors.Is(err, xnode.ErrConflict) {
			t.Fatal("refund used an obsolete closed aggregate", err)
		}
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
}
