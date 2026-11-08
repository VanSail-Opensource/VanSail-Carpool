package hourlybilling

import (
	"context"
	"errors"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/database"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/providerops"
)

func (s *Service) activateOperation(ctx context.Context, op providerops.Operation, item providerops.Item, target operationTarget) error {
	input := database.HourlyActivationInput{OwnerID: op.OwnerUserID, AccountGeneration: target.Generation,
		GroupID: target.GroupID, ActivationKey: op.Receipt.ID, ExpectedPolicyVersion: target.PolicyVersion,
		ExpectedOfferRevision: target.OfferRevision, ActivationCode: target.ActivationCode}
	existing, err := s.store.HourlyEnrollmentForActivation(ctx, input.OwnerID, input.AccountGeneration, input.ActivationKey)
	if err != nil && !errors.Is(err, database.ErrNotFound) {
		return wait("HOURLY_PAYMENT_REPLAY_PENDING", err)
	}
	if err == nil && existing.Status != model.HourlyPreparing {
		// Revalidate the immutable fingerprint under the owner's wallet lock.
		existing, err = s.store.PrepareHourlyActivation(ctx, input, model.HourlyPolicySnapshot{}, s.now())
		if err != nil {
			return wait("HOURLY_PAYMENT_REPLAY_PENDING", err)
		}
		return s.applyActivation(ctx, op, item, existing)
	}
	snapshot, err := s.policy.HourlyPolicy(ctx)
	if err != nil {
		return wait("HOURLY_POLICY_PENDING", err)
	}
	enrollment, err := s.store.PrepareHourlyActivation(ctx, input, snapshot, s.now())
	if err != nil {
		if businessFailure(err) {
			return s.cancelActivation(ctx, op, item, existing.ID, target.Generation, err)
		}
		return wait("HOURLY_PREPARATION_PENDING", err)
	}
	if enrollment.Status != model.HourlyPreparing {
		return s.applyActivation(ctx, op, item, enrollment)
	}
	if s.native == nil {
		return wait("HOURLY_NATIVE_PENDING", ErrUnavailable)
	}
	principal, err := s.native.PrepareHourlyPrincipal(ctx, enrollment.ID)
	if err != nil {
		return wait("HOURLY_PRINCIPAL_PENDING", err)
	}
	observed := s.now().UTC()
	ready, err := s.native.PreflightHourlyEnrollment(ctx, enrollment.ID)
	if err == nil && !ready.Ready && ready.Code == "XNODE_HOURLY_CAPACITY" {
		return s.cancelActivation(ctx, op, item, enrollment.ID, target.Generation, database.ErrHourlyCapacity)
	}
	if err != nil || !ready.Ready {
		return wait("HOURLY_DELIVERY_PENDING", errors.Join(ErrUnavailable, err))
	}
	until := observed.Add(4 * time.Second)
	if ready.ReadyUntil.Before(until) {
		until = ready.ReadyUntil
	}
	delivery := database.HourlyDeliveryReady{EnrollmentID: enrollment.ID, ObservedAt: observed, ReadyUntil: until, CompileRevision: ready.CompileRevision}
	commitCtx, cancelCommit := context.WithTimeout(ctx, 4*time.Second)
	result, err := s.store.CommitHourlyActivation(commitCtx, database.HourlyCommitInput{HourlyActivationInput: input,
		EnrollmentID: enrollment.ID, ProxyUserID: principal.ProxyUserID, PeriodID: principal.PeriodID,
		ExpectedCompileRevision: ready.CompileRevision, Delivery: delivery}, snapshot, s.now())
	cancelCommit()
	if err != nil {
		if errors.Is(err, database.ErrHourlyDeliveryPending) {
			return deliveryWait("HOURLY_DELIVERY_PENDING", err, []database.HourlyDeliveryReady{delivery}, []string{enrollment.ID}, s.now())
		}
		if businessFailure(err) {
			return s.cancelActivation(ctx, op, item, enrollment.ID, target.Generation, err)
		}
		return wait("HOURLY_PAYMENT_PENDING", err)
	}
	return s.applyActivation(ctx, op, item, result.Enrollment)
}

func (s *Service) applyActivation(ctx context.Context, op providerops.Operation, item providerops.Item, e model.HourlyEnrollment) error {
	if s.native == nil {
		return wait("HOURLY_NATIVE_PENDING", ErrUnavailable)
	}
	// The native lifecycle repeats current generation/paid eligibility under the
	// same owner lock. A historical paid receipt cannot resume a disabled benefit.
	if e.Status == model.HourlyActive && e.PaidThrough != nil && e.PaidThrough.After(s.now()) {
		if err := s.native.ResumeHourlyOwner(ctx, e.OwnerID, e.AccountGeneration); err != nil {
			return wait("HOURLY_RESUME_PENDING", err)
		}
	}
	if err := s.native.SyncHourlyEnrollment(ctx, e.ID); err != nil {
		return wait("HOURLY_APPLICATION_PENDING", err)
	}
	if e.Status == model.HourlyDisabled || e.Status == model.HourlyClosed || e.Status == model.HourlyFundingSuspended {
		if err := s.native.CloseHourlyOwnerIfIdle(ctx, e.OwnerID, e.AccountGeneration, e.ID); err != nil {
			return wait("HOURLY_ACCOUNTING_PENDING", err)
		}
	}
	if e.ActivatedAt == nil {
		return s.finish(ctx, op, item, providerops.StatusFailed, "HOURLY_ACTIVATION_CANCELLED")
	}
	return s.finish(ctx, op, item, providerops.StatusSucceeded, "")
}

func (s *Service) cancelActivation(ctx context.Context, op providerops.Operation, item providerops.Item, id string, generation int64, cause error) error {
	if id != "" {
		e, err := s.store.CancelHourlyPreparation(ctx, op.OwnerUserID, generation, id, s.now())
		if err != nil { // A concurrent uncertain commit may already have paid it.
			return wait("HOURLY_PAYMENT_REPLAY_PENDING", err)
		}
		if s.native == nil {
			return wait("HOURLY_CLEANUP_PENDING", ErrUnavailable)
		}
		if err = s.native.SyncHourlyEnrollment(ctx, e.ID); err != nil {
			return wait("HOURLY_CLEANUP_PENDING", err)
		}
		if err = s.native.CloseHourlyOwnerIfIdle(ctx, e.OwnerID, e.AccountGeneration, e.ID); err != nil {
			return wait("HOURLY_ACCOUNTING_PENDING", err)
		}
	}
	return s.finish(ctx, op, item, providerops.StatusFailed, failureCode(cause))
}

func failureCode(err error) string {
	switch {
	case errors.Is(err, database.ErrInsufficientBalance):
		return "INSUFFICIENT_BALANCE"
	case errors.Is(err, database.ErrActivationCodeRequired):
		return "SQUAD_ACTIVATION_REQUIRED"
	case errors.Is(err, database.ErrActivationCodeInvalid):
		return "SQUAD_ACTIVATION_INVALID"
	case errors.Is(err, database.ErrStockUnavailable):
		return "SQUAD_STOCK_UNAVAILABLE"
	case errors.Is(err, database.ErrHourlyCooldown):
		return "HOURLY_COOLDOWN"
	case errors.Is(err, database.ErrHourlyLegacyConflict):
		return "HOURLY_LEGACY_ACTIVE"
	case errors.Is(err, database.ErrHourlyCapacity):
		return "HOURLY_CAPACITY"
	default:
		return "HOURLY_TERMS_CHANGED"
	}
}
