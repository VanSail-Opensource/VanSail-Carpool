package hourlybilling

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
)

// Bound native application separately from financial settlement. Four callers
// use the existing per-node executor; one unavailable node cannot occupy the
// shared outbox slot for the sum of every enrollment's individual timeout.
func (s *Service) applySettlements(ctx context.Context, enrollments []model.HourlyEnrollment) error {
	child, cancel := context.WithTimeout(ctx, 4*time.Second)
	defer cancel()
	jobs := make(chan model.HourlyEnrollment, len(enrollments))
	if len(enrollments) > 0 {
		offset := s.deliveryOffset(enrollments[0].OwnerID, true, len(enrollments))
		for i := range enrollments {
			jobs <- enrollments[(i+offset)%len(enrollments)]
		}
	}
	close(jobs)
	var group sync.WaitGroup
	var mu sync.Mutex
	var pending error
	completed := 0
	for range min(4, len(enrollments)) {
		group.Go(func() {
			for e := range jobs {
				if child.Err() != nil {
					return
				}
				err := s.applySettlement(child, e)
				mu.Lock()
				completed++
				pending = errors.Join(pending, err)
				mu.Unlock()
			}
		})
	}
	group.Wait()
	if completed < len(enrollments) {
		pending = errors.Join(pending, child.Err())
	}
	return pending
}

func (s *Service) applySettlement(ctx context.Context, e model.HourlyEnrollment) error {
	if e.Status == model.HourlyActive && e.PaidThrough != nil && e.PaidThrough.After(s.now()) {
		if err := s.native.ResumeHourlyOwner(ctx, e.OwnerID, e.AccountGeneration); err != nil {
			return err
		}
	}
	if err := s.native.SyncHourlyEnrollment(ctx, e.ID); err != nil {
		return err
	}
	if e.Status == model.HourlyFundingSuspended || e.Status == model.HourlyDisabled || e.Status == model.HourlyClosed {
		return s.native.CloseHourlyOwnerIfIdle(ctx, e.OwnerID, e.AccountGeneration, e.ID)
	}
	return nil
}
