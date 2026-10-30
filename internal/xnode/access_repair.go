package xnode

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/outbox"
)

const AccessRepairJob = "xnode_access_repair"

var ErrAccessOperationPending = errors.New("XNODE_ACCESS_OPERATION_PENDING")
var ErrAccessAccountingPending = errors.New("XNODE_ACCESS_ACCOUNTING_PENDING")
var ErrAccessAmbiguous = errors.New("XNODE_ACCESS_ALLOWANCE_AMBIGUOUS")

type accessRepairRepository interface {
	ComboControlConflict(context.Context, string) (bool, error)
	RepairXNodeAccessPeriod(context.Context, Identity, model.Purchase, Period, time.Time) error
}

// RepairAccess runs only in the shared durable outbox, never in compilation or
// bearer handlers. Every onboarded member receives subscription credentials;
// existing periods retain their counters and settlement state.
func (s *Service) RepairAccess(ctx context.Context, job model.OutboxJob) error {
	var input struct {
		UserID string `json:"userId"`
	}
	if json.Unmarshal([]byte(job.Payload), &input) != nil || input.UserID == "" {
		return ErrInvalid
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for range 3 {
		err := s.repairAccess(ctx, input.UserID)
		if !errors.Is(err, ErrConflict) {
			if errors.Is(err, ErrAccessOperationPending) {
				return &accessRepairDeferred{reason: err}
			}
			return err
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}
	}
	return fmt.Errorf("XNODE_ACCESS_STATE_CHANGED: %w", outbox.ErrBusy)
}

type accessRepairDeferred struct{ reason error }

func (e *accessRepairDeferred) Error() string             { return e.reason.Error() }
func (e *accessRepairDeferred) Unwrap() error             { return outbox.ErrBusy }
func (e *accessRepairDeferred) RetryDelay() time.Duration { return time.Minute }

func (s *Service) repairAccess(ctx context.Context, userID string) error {
	user, err := s.repo.UserByID(ctx, userID)
	if err != nil || user.OnboardingState != "complete" {
		return err
	}
	identity, err := s.ensureIdentity(ctx, user)
	if err != nil {
		return err
	}
	purchase, err := s.EffectivePurchase(ctx, user.ID)
	if err != nil || purchase == nil {
		return err
	}
	restricted, err := s.repo.XNodeUserRestricted(ctx, user.ID)
	if err != nil || restricted || identity.AccessDisabled {
		return err
	}
	r, ok := s.repo.(accessRepairRepository)
	if !ok {
		return ErrUnavailable
	}
	blocked, err := r.ComboControlConflict(ctx, user.ID)
	if err != nil {
		return err
	}
	if blocked {
		return ErrAccessOperationPending
	}
	periods, err := s.repo.XNodePeriods(ctx, identity.ProxyUserID)
	if err != nil {
		return err
	}
	var open *Period
	var closing *Period
	found := false
	for _, period := range periods {
		if period.PurchaseID != purchase.ID {
			continue
		}
		found = true
		if period.Status == "open" {
			if open != nil {
				return ErrAccessAmbiguous
			}
			copy := period
			open = &copy
		}
		if period.Status == "closing" && period.ID == identity.CurrentPeriodID {
			copy := period
			closing = &copy
		}
	}
	if open != nil {
		// Earlier resets can already be settled. Restore only the existing open
		// period pointer and retain its counters and all historical evidence.
		return r.RepairXNodeAccessPeriod(ctx, identity, *purchase, *open, s.now())
	}
	if closing != nil {
		return r.RepairXNodeAccessPeriod(ctx, identity, *purchase, *closing, s.now())
	}
	if found {
		return ErrAccessAccountingPending
	}
	return s.applyPurchase(ctx, strconv.FormatInt(identity.ProxyUserID, 10), purchase.TrafficLimitBytes, purchase.ResetStrategy, purchase.ValidUntil)
}
