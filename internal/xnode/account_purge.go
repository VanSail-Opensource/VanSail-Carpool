package xnode

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	wire "github.com/VanSail-Opensource/X-Node/pkg/protocol"
)

type accountNativePurgeRepository interface {
	AccountNativePurgePage(context.Context, model.AccountPurge, string, func([]PurgeActivityScope) error) ([]Period, bool, error)
	AdvanceAccountNativePurge(context.Context, model.AccountPurge, string, Period) (Period, bool, error)
	AccountCleanupAdmissionValid(context.Context, string, int64, time.Time) (bool, error)
	FinalizeAccountNativePurgePeriod(context.Context, model.AccountPurge, string, Period) (bool, error)
	SealAccountNativePurgePeriodNode(context.Context, model.AccountPurge, string, Period, string, wire.TrafficBarrier) (bool, error)
	ReleaseAccountPurgeExemptions(context.Context, model.AccountPurge, string) (bool, error)
}

// CloseAccountPeriods rotates one durable cursor across every retained native
// period. Only the admitted current purge can fence its historical generations.
func (s *Service) CloseAccountPeriods(ctx context.Context, target model.AccountPurge, jobID string) error {
	repo, ok := s.repo.(accountNativePurgeRepository)
	if !ok {
		return ErrUnavailable
	}
	periods, active, err := repo.AccountNativePurgePage(ctx, target, jobID, func(scopes []PurgeActivityScope) error {
		if s.master != nil {
			if err := s.master.fenceAccountActivity(scopes); err != nil {
				return err
			}
			s.master.retireHourlyCreditThrough(target.UserID, target.ThroughGeneration)
		}
		return nil
	})
	if err != nil || !active {
		return err
	}
	var pending error
	released, e := repo.ReleaseAccountPurgeExemptions(ctx, target, jobID)
	if e != nil {
		pending = e
	} else if !released {
		pending = &PolicyError{"ACCOUNT_PURGE_FIREWALL_RELEASE_PENDING", ErrAccessAccountingPending}
	}
	ipCtx, cancelIP := context.WithTimeout(ctx, 2*time.Second)
	pending = errors.Join(pending, s.closeAccountPurgeIPControls(ipCtx, repo, target, jobID))
	cancelIP()
	for _, period := range periods {
		if ctx.Err() != nil {
			return errors.Join(pending, ctx.Err())
		}
		// Advance before each attempt, so context exhaustion and restarts do not
		// keep selecting an offline page's first period.
		current, allowed, e := repo.AdvanceAccountNativePurge(ctx, target, jobID, period)
		if e == nil && !allowed {
			return nil
		}
		if e == nil {
			e = s.settleAccountPurgePeriod(ctx, repo, target, jobID, current)
		}
		if e != nil {
			pending = errors.Join(pending, fmt.Errorf("owner %s period %s: %w", target.UserID, period.ID, e))
		}
	}
	if err = s.accountPurgeStillAdmitted(ctx, repo, target); err != nil {
		return err
	}
	if s.master != nil {
		pending = errors.Join(pending, s.master.inspectHourlyCreditThrough(ctx, target.UserID, target.ThroughGeneration))
	}
	remaining, active, err := repo.AccountNativePurgePage(ctx, target, jobID, nil)
	if err != nil {
		return errors.Join(pending, err)
	}
	if !active {
		return nil
	}
	if len(remaining) != 0 {
		pending = errors.Join(pending, ErrAccessAccountingPending)
	}
	if pending != nil {
		return pending
	}
	return nil
}

func (s *Service) accountPurgeStillAdmitted(ctx context.Context, repo accountNativePurgeRepository, target model.AccountPurge) error {
	valid, err := repo.AccountCleanupAdmissionValid(ctx, target.UserID, target.Generation, s.now().UTC())
	if err == nil && !valid {
		err = &PolicyError{"ACCOUNT_PURGE_SUPERSEDED", ErrConflict}
	}
	return err
}
