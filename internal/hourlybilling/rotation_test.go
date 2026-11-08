package hourlybilling

import (
	"context"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
)

func TestBlockedReadinessHeadCannotStarveHealthyTailAcrossTurns(t *testing.T) {
	s := New(nil, nil, nil, nil)
	var healthy atomic.Int32
	s.native = &fixtureNative{readyHook: func(ctx context.Context, group string) (xnode.HourlyReadiness, error) {
		if group != "healthy" {
			<-ctx.Done()
			return xnode.HourlyReadiness{}, ctx.Err()
		}
		healthy.Add(1)
		return xnode.HourlyReadiness{Ready: true, CompileRevision: 1, ReadyUntil: time.Now().Add(time.Second)}, nil
	}}
	paid := time.Now().Add(-time.Hour)
	enrollments := []model.HourlyEnrollment{}
	for i := range 9 {
		group := "blocked"
		if i == 8 {
			group = "healthy"
		}
		enrollments = append(enrollments, model.HourlyEnrollment{ID: fmt.Sprint(i), OwnerID: "owner", GroupID: group, Status: model.HourlyActive, PaidThrough: &paid})
	}
	for range 2 {
		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		s.deliveryGuards(ctx, enrollments)
		cancel()
	}
	if healthy.Load() == 0 {
		t.Fatal("rotating readiness left the healthy ninth squad behind the blocked first eight")
	}
}

func TestBlockedApplicationHeadCannotStarveHealthyTailAcrossTurns(t *testing.T) {
	s := New(nil, nil, nil, nil)
	var healthy atomic.Int32
	s.native = &fixtureNative{syncHook: func(ctx context.Context, id string) error {
		if id == "healthy" {
			healthy.Add(1)
			return nil
		}
		<-ctx.Done()
		return ctx.Err()
	}}
	enrollments := []model.HourlyEnrollment{}
	for i := range 5 {
		id := fmt.Sprint(i)
		if i == 4 {
			id = "healthy"
		}
		enrollments = append(enrollments, model.HourlyEnrollment{ID: id, OwnerID: "owner", Status: model.HourlyActive})
	}
	for range 2 {
		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		_ = s.applySettlements(ctx, enrollments)
		cancel()
	}
	if healthy.Load() == 0 {
		t.Fatal("rotating application left healthy fifth squad behind blocked first four")
	}
}
