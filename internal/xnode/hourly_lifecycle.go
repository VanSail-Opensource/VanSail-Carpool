package xnode

import (
	"context"
	"errors"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	wire "github.com/VanSail-Opensource/X-Node/pkg/protocol"
)

// Purge requires the exact current purging generation; ThroughGeneration only
// bounds historical scopes. Ordinary idle closure names its terminal enrollment
// and refuses an owner RAM stop after a newer paid/preparing benefit appears.
type HourlyLifecycleIntent struct {
	OwnerID, EnrollmentID                string
	CurrentGeneration, ThroughGeneration int64
	Purge, Resume                        bool
}

type hourlyLifecycleRepository interface {
	WithHourlyOwnerLifecycle(context.Context, HourlyLifecycleIntent, func() error) ([]HourlyPrincipal, bool, error)
}

func (s *Service) CloseHourlyOwnerIfIdle(ctx context.Context, owner string, generation int64, enrollment string) error {
	return s.closeHourlyOwner(ctx, HourlyLifecycleIntent{OwnerID: owner, CurrentGeneration: generation, ThroughGeneration: generation, EnrollmentID: enrollment})
}

func (s *Service) CloseHourlyOwnerForPurge(ctx context.Context, owner string, currentGeneration, throughGeneration int64) error {
	return s.closeHourlyOwner(ctx, HourlyLifecycleIntent{OwnerID: owner, CurrentGeneration: currentGeneration, ThroughGeneration: throughGeneration, Purge: true})
}

func (s *Service) closeHourlyOwner(ctx context.Context, intent HourlyLifecycleIntent) error {
	repo, ok := s.repo.(hourlyLifecycleRepository)
	if !ok {
		return ErrUnavailable
	}
	principals, marked, err := repo.WithHourlyOwnerLifecycle(ctx, intent, func() error {
		if s.master != nil {
			s.master.retireHourlyCreditThrough(intent.OwnerID, intent.ThroughGeneration)
		}
		return nil
	})
	if err != nil {
		return err
	}
	var pending error
	for _, principal := range principals {
		if principal.PeriodID == "" {
			continue
		}
		period, e := s.repo.XNodePeriod(ctx, principal.PeriodID)
		if e == nil && period.ProxyUserID != principal.ProxyUserID {
			e = ErrConflict
		}
		if e == nil {
			e = s.closePeriod(ctx, period)
		}
		pending = errors.Join(pending, e)
	}
	if marked && s.master != nil {
		pending = errors.Join(pending, s.master.inspectHourlyCreditThrough(ctx, intent.OwnerID, intent.ThroughGeneration))
	}
	if intent.Purge && len(principals) == 128 {
		pending = errors.Join(pending, ErrAccessAccountingPending)
	}
	return pending
}

func (s *Service) ResumeHourlyOwner(ctx context.Context, owner string, generation int64) error {
	repo, ok := s.repo.(hourlyLifecycleRepository)
	if !ok {
		return ErrUnavailable
	}
	_, resumed, err := repo.WithHourlyOwnerLifecycle(ctx, HourlyLifecycleIntent{OwnerID: owner, CurrentGeneration: generation, ThroughGeneration: generation, Resume: true}, func() error {
		if s.master == nil {
			return ErrUnavailable
		}
		return s.master.ResumeHourlyCreditAdmission(owner, generation)
	})
	if err == nil && !resumed {
		return ErrConflict
	}
	return err
}

type hourlyOutstandingThroughRepository interface {
	OutstandingHourlyCreditsThrough(context.Context, string, int64, int) ([]model.HourlyCreditReservation, error)
}

func (m *Master) retireHourlyCreditThrough(owner string, through int64) {
	c := m.hourlyCredits
	if c == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	for _, state := range c.peers {
		markHourlyCreditRetired(state, wire.HourlyOwner{ID: owner, Generation: uint64(through)})
		for subject := range state.hints {
			if subject.ID == owner && subject.Generation <= uint64(through) {
				markHourlyCreditRetired(state, subject)
			}
		}
		for subject := range state.intents {
			if subject.ID == owner && subject.Generation <= uint64(through) {
				markHourlyCreditRetired(state, subject)
			}
		}
		state.work = true
	}
	c.signal()
}

// Inspection never re-marks per-row RAM state after releasing the users lock.
func (m *Master) inspectHourlyCreditThrough(ctx context.Context, owner string, through int64) error {
	repo, ok := m.service.repo.(hourlyOutstandingThroughRepository)
	if !ok {
		return ErrUnavailable
	}
	rows, err := repo.OutstandingHourlyCreditsThrough(ctx, owner, through, 128)
	if err != nil {
		return err
	}
	if len(rows) != 0 {
		for _, row := range rows {
			if row.FinalProofDigest != "" && row.UnknownBytes > 0 {
				return &PolicyError{"XNODE_HOURLY_CREDIT_RECOVERY_UNKNOWN", ErrAccessAccountingPending}
			}
			m.mu.RLock()
			peer := m.peers[row.NodeID]
			m.mu.RUnlock()
			if !m.currentHourlyCreditPeer(peer) || peer.agentID != row.AgentID {
				return &PolicyError{"XNODE_HOURLY_CREDIT_OFFLINE", ErrAccessAccountingPending}
			}
		}
		return &PolicyError{"XNODE_HOURLY_CREDIT_PROOF_PENDING", ErrAccessAccountingPending}
	}
	return nil
}
