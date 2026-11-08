package hourlybilling

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
)

func TestReadinessWaitCannotBlockHealthySquadProof(t *testing.T) {
	started := make(chan struct{})
	healthy := make(chan struct{})
	native := &fixtureNative{readyHook: func(ctx context.Context, group string) (xnode.HourlyReadiness, error) {
		if group == "blocked" {
			close(started)
			<-ctx.Done()
			return xnode.HourlyReadiness{}, ctx.Err()
		}
		close(healthy)
		return xnode.HourlyReadiness{Ready: true, CompileRevision: 7, ReadyUntil: time.Now().Add(4 * time.Second)}, nil
	}}
	s := New(nil, nil, nil, nil)
	s.native = native
	before := time.Now().Add(-time.Hour)
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	done := make(chan []string, 1)
	go func() {
		guards := s.deliveryGuards(ctx, []model.HourlyEnrollment{
			{ID: "blocked", OwnerID: "owner", GroupID: "blocked", Status: model.HourlyActive, PaidThrough: &before},
			{ID: "healthy", OwnerID: "owner", GroupID: "healthy", Status: model.HourlyActive, PaidThrough: &before},
		})
		ids := []string{}
		for _, g := range guards {
			ids = append(ids, g.EnrollmentID)
		}
		done <- ids
	}()
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("blocked readiness never began")
	}
	select {
	case <-healthy:
	case <-time.After(time.Second):
		t.Fatal("healthy readiness serialized behind blocked squad")
	}
	select {
	case ids := <-done:
		if len(ids) != 1 || ids[0] != "healthy" {
			t.Fatal("healthy readiness proof was lost")
		}
	case <-time.After(time.Second):
		t.Fatal("readiness deadline was not honored")
	}
}

func TestDependencyWaitDoesNotBecomeTerminalAtQueueRetryLimit(t *testing.T) {
	s, user, policy, native := commerceFixture(t)
	op, job := queuedActivation(t, s, user, policy)
	native.readyHook = func(context.Context, string) (xnode.HourlyReadiness, error) {
		return xnode.HourlyReadiness{}, errors.New("offline participant")
	}
	job.Attempts = 100
	if err := s.HandleProviderOperation(context.Background(), op, job); err == nil {
		t.Fatal("offline readiness was accepted")
	}
	op, _ = reloadOperation(t, s, op.Receipt.ID)
	if op.Receipt.Status != "processing" || balanceFixture(t, s, user) != 100000 {
		t.Fatal("offline dependency exhausted the receipt or charged unavailable access")
	}
}
