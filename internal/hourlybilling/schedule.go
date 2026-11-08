package hourlybilling

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/database"
)

type settlementTarget struct {
	OwnerID    string `json:"userId"`
	Generation int64  `json:"accountGeneration,string"`
}

// ScheduleDue is a bounded keyset scan for the existing scheduler, not a new
// executor. Payload coalescing retains one resumable job per owner generation.
func (s *Service) ScheduleDue(ctx context.Context, at time.Time) error {
	s.mu.Lock()
	after := s.dueAfter
	s.mu.Unlock()
	due, err := s.store.DueHourlyOwnersPage(ctx, at.UTC(), after, 200)
	if err != nil {
		return err
	}
	for _, owner := range due {
		raw, err := json.Marshal(settlementTarget{owner.OwnerID, owner.AccountGeneration})
		if err != nil {
			return err
		}
		if err = s.store.EnqueueOutbox(ctx, SettlementJob, string(raw), at.UTC()); err != nil {
			return err
		}
		s.mu.Lock()
		s.dueAfter = owner.OwnerID
		s.mu.Unlock()
	}
	s.mu.Lock()
	if len(due) < 200 {
		s.dueAfter = ""
	}
	s.mu.Unlock()
	return nil
}

func (s *Service) HandleOutbox(ctx context.Context, job model.OutboxJob) error {
	var target settlementTarget
	if job.Kind != SettlementJob || json.Unmarshal([]byte(job.Payload), &target) != nil || target.OwnerID == "" || target.Generation < 0 {
		return errors.New("invalid hourly settlement target")
	}
	user, err := s.store.UserByID(ctx, target.OwnerID)
	if errors.Is(err, database.ErrNotFound) || err == nil && (user.AccountGeneration != target.Generation || user.AccountStatus != "active") {
		return nil // Superseded account generations cannot be charged or resumed.
	}
	if err != nil {
		return wait("HOURLY_OWNER_PENDING", err)
	}
	return s.settleOwner(ctx, target)
}
