package catalog

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/database"
)

var ErrAutoRenewalEnabled = database.ErrAutoRenewalEnabled
var ErrAutoRenewalIneligible = errors.New("automatic renewal is currently ineligible")

type automaticRenewalRepository interface {
	AutoRenewalPlan(context.Context, string, string, time.Time) (database.AutoRenewalPlan, error)
	SetAutoRenewal(context.Context, string, string, bool, time.Time) error
	DueAutoRenewals(context.Context, time.Time) ([]database.DueAutoRenewal, error)
	CommitAutoRenewal(context.Context, string, time.Time) (model.Purchase, error)
	MarkAutoRenewalFailed(context.Context, string, string, time.Time) error
	HasEnabledAutoRenewal(context.Context, string, time.Time) (bool, error)
}

type automaticRenewalFailureRepository interface {
	AutoRenewalFailure(context.Context, string) (*model.AutoRenewalFailure, error)
}

// AutomaticRenewal returns a member's authoritative current next-cycle quote.
func (s *Service) AutomaticRenewal(ctx context.Context, user model.User, purchaseID string) (model.AutoRenewal, error) {
	return s.automaticRenewalAt(ctx, user, purchaseID, s.now().UTC())
}

func (s *Service) automaticRenewalAt(ctx context.Context, user model.User, purchaseID string, now time.Time) (model.AutoRenewal, error) {
	result, _, err := s.automaticRenewalSelectionAt(ctx, user, purchaseID, now)
	return result, err
}

// SetAutomaticRenewal saves an eligible enablement choice or always permits disabling.
func (s *Service) SetAutomaticRenewal(ctx context.Context, user model.User, purchaseID string, enabled bool) (model.AutoRenewal, error) {
	repository, ok := s.repository.(automaticRenewalRepository)
	if !ok {
		return model.AutoRenewal{}, database.ErrNotFound
	}
	if !enabled {
		plan, err := repository.AutoRenewalPlan(ctx, user.ID, purchaseID, s.now().UTC())
		if err != nil {
			return model.AutoRenewal{}, err
		}
		if err := repository.SetAutoRenewal(ctx, user.ID, purchaseID, false, s.now().UTC()); err != nil {
			return model.AutoRenewal{}, err
		}
		status := autoRenewalFromPlan(plan)
		status.Enabled = false
		status.CanEnable = status.IneligibleReason == nil
		return status, nil
	}
	status, err := s.AutomaticRenewal(ctx, user, purchaseID)
	if err != nil {
		return model.AutoRenewal{}, err
	}
	if enabled && !status.CanEnable {
		return status, ErrAutoRenewalIneligible
	}
	if err := repository.SetAutoRenewal(ctx, user.ID, purchaseID, true, s.now().UTC()); err != nil {
		return model.AutoRenewal{}, err
	}
	status.Enabled = true
	return status, nil
}

func (s *Service) ProcessDueAutoRenewals(ctx context.Context, now time.Time) error {
	repository, ok := s.repository.(automaticRenewalRepository)
	if !ok {
		return nil
	}
	candidates, err := repository.DueAutoRenewals(ctx, now.UTC())
	if err != nil {
		return err
	}
	var processingErr error
	for _, candidate := range candidates {
		if err := ctx.Err(); err != nil {
			return err
		}
		status, unavailable, statusErr := s.automaticRenewalSelectionAt(ctx, model.User{ID: candidate.UserID}, candidate.PurchaseID, now.UTC())
		if statusErr == nil && status.CanEnable {
			if _, err := s.commitAutomaticRenewal(ctx, repository, candidate.PurchaseID, unavailable, now.UTC()); err == nil {
				continue
			} else {
				statusErr = err
			}
		}
		shouldDefer := database.IsPricingRenewalDeferral(statusErr) || errors.Is(statusErr, database.ErrStockUnavailable)
		if statusErr == nil && status.IneligibleReason != nil {
			reason := *status.IneligibleReason
			shouldDefer = reason == database.AutoRenewalReasonNoAccessibleNodes || reason == database.AutoRenewalReasonUnavailable || reason == database.AutoRenewalReasonPaidAddonUnavailable
		}
		if deferrals, ok := s.repository.(interface {
			DeferPricingRenewal(context.Context, string, time.Time) (bool, error)
		}); ok && shouldDefer {
			deferred, err := deferrals.DeferPricingRenewal(ctx, candidate.PurchaseID, now.UTC())
			if err != nil {
				return err
			}
			if deferred {
				continue
			}
		}
		// Only known business outcomes can terminate a renewal. A database,
		// transport or cancellation error is not evidence that it failed.
		if statusErr != nil && !errors.Is(statusErr, database.ErrInsufficientBalance) &&
			!database.IsPricingRenewalDeferral(statusErr) && !errors.Is(statusErr, database.ErrStockUnavailable) {
			processingErr = errors.Join(processingErr, fmt.Errorf("automatic renewal %s: %w", candidate.PurchaseID, statusErr))
			continue
		}
		reason := autoRenewalFailureReason(status, statusErr)
		if err := repository.MarkAutoRenewalFailed(ctx, candidate.PurchaseID, reason, now.UTC()); err != nil {
			return err
		}
	}
	return processingErr
}

func autoRenewalFromPlan(plan database.AutoRenewalPlan) model.AutoRenewal {
	result := model.AutoRenewal{PurchaseID: plan.Purchase.ID, Enabled: plan.Purchase.AutoRenewEnabled,
		GrossPrice: model.TXBMoney(plan.GrossMinor), Discount: model.TXBMoney(plan.DiscountMinor), NetPrice: model.TXBMoney(plan.NetMinor),
		ScheduledAt: plan.ScheduledAt, NextCycleEndsAt: plan.NextCycleEndsAt}
	if plan.IneligibleReason != "" {
		reason := plan.IneligibleReason
		result.IneligibleReason = &reason
	}
	return result
}

func (s *Service) autoRenewalLiveReason(ctx context.Context, plan database.AutoRenewalPlan, catalog model.Catalog) (*string, error) {
	addonIDs := make([]string, 0, len(plan.Addons))
	for _, addon := range plan.Addons {
		addonIDs = append(addonIDs, addon.GroupID)
	}
	nodes := quoteAccessibleNodes(catalog, plan.Combo.ID, addonIDs)
	if len(nodes) == 0 {
		return autoRenewalReason(database.AutoRenewalReasonNoAccessibleNodes), nil
	}
	return nil, nil
}

func autoRenewalFailureReason(status model.AutoRenewal, err error) string {
	if status.IneligibleReason != nil {
		return *status.IneligibleReason
	}
	if errors.Is(err, database.ErrInsufficientBalance) {
		return database.AutoRenewalReasonInsufficientBalance
	}
	return database.AutoRenewalReasonUnavailable
}

func autoRenewalReason(value string) *string { return &value }
