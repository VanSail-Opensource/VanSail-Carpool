package hourlybilling

import (
	"context"
	"sync"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/database"
)

// One bounded inspection window keeps an offline or SQL-blocked squad from
// expiring every earlier proof. It grants nothing; locked billing repeats the
// exact compile revision and four-second freshness checks before payment.
func (s *Service) deliveryGuards(ctx context.Context, enrollments []model.HourlyEnrollment) []database.HourlyDeliveryReady {
	child, cancel := context.WithTimeout(ctx, 1500*time.Millisecond)
	defer cancel()
	due := []model.HourlyEnrollment{}
	for _, e := range enrollments {
		if e.Status == model.HourlyActive && e.PaidThrough != nil && !e.PaidThrough.After(s.now()) {
			due = append(due, e)
		}
	}
	jobs := make(chan model.HourlyEnrollment, len(due))
	if len(due) > 0 {
		offset := s.deliveryOffset(due[0].OwnerID, false, len(due))
		for i := range due {
			jobs <- due[(i+offset)%len(due)]
		}
	}
	close(jobs)
	var group sync.WaitGroup
	var mu sync.Mutex
	result := []database.HourlyDeliveryReady{}
	for range min(8, len(due)) {
		group.Go(func() {
			for e := range jobs {
				if child.Err() != nil {
					return
				}
				observed := s.now().UTC()
				ready, err := s.native.HourlySquadReadiness(child, e.GroupID, e.OwnerID)
				if err != nil || !ready.Ready {
					continue
				}
				until := observed.Add(4 * time.Second)
				if ready.ReadyUntil.Before(until) {
					until = ready.ReadyUntil
				}
				mu.Lock()
				result = append(result, database.HourlyDeliveryReady{EnrollmentID: e.ID, ObservedAt: observed,
					ReadyUntil: until, CompileRevision: ready.CompileRevision})
				mu.Unlock()
			}
		})
	}
	group.Wait()
	return result
}
