package xnode

import (
	"context"
	"encoding/json"
	"strconv"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
)

const AccessRepairJob = "xnode_access_repair"

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
	user, err := s.repo.UserByID(ctx, input.UserID)
	if err != nil || user.OnboardingState != "complete" {
		return err
	}
	identity, err := s.repo.XNodeIdentityByUser(ctx, user.ID)
	if isMissing(err) {
		identity, err = s.EnsureIdentity(ctx, user)
	}
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
	periods, err := s.repo.XNodePeriods(ctx, identity.ProxyUserID)
	if err != nil {
		return err
	}
	var open *Period
	found := false
	for _, period := range periods {
		if period.PurchaseID != purchase.ID {
			continue
		}
		found = true
		if period.Status == "open" {
			if open != nil {
				return ErrConflict // Ambiguous allowances require review.
			}
			copy := period
			open = &copy
		}
	}
	if open != nil {
		// Earlier resets can already be settled. Restore only the existing open
		// period pointer and retain its counters and all historical evidence.
		return s.repo.SaveXNodePeriod(ctx, *open)
	}
	if found {
		return ErrConflict
	}
	return s.ApplyPurchase(ctx, strconv.FormatInt(identity.ProxyUserID, 10), purchase.TrafficLimitBytes, purchase.ResetStrategy, purchase.ValidUntil)
}
