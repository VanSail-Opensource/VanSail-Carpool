package hourlybilling

import (
	"context"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/database"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/providerops"
)

func (s *Service) disableOperation(ctx context.Context, op providerops.Operation, item providerops.Item, target operationTarget) error {
	e, err := s.store.HourlyEnrollment(ctx, target.EnrollmentID)
	if err != nil {
		return wait("HOURLY_DISABLE_TARGET_PENDING", err)
	}
	if e.OwnerID != op.OwnerUserID || e.AccountGeneration != target.Generation || e.GroupID != target.GroupID {
		return wait("HOURLY_DISABLE_TARGET_PENDING", database.ErrConflict)
	}
	// Replaying a previously committed stop must not depend on mutable policy.
	if e.Status != model.HourlyDisabled && e.Status != model.HourlyClosed {
		snapshot, err := s.policy.HourlyPolicy(ctx)
		if err != nil {
			return wait("HOURLY_POLICY_PENDING", err)
		}
		if snapshot.Version != target.PolicyVersion {
			return s.finish(ctx, op, item, providerops.StatusFailed, "HOURLY_TERMS_CHANGED")
		}
		e, err = s.store.DisableHourlyEnrollment(ctx, database.HourlyDisableInput{OwnerID: op.OwnerUserID,
			AccountGeneration: target.Generation, EnrollmentID: target.EnrollmentID, Voluntary: true}, snapshot, s.now())
		if err != nil {
			if businessFailure(err) {
				return s.finish(ctx, op, item, providerops.StatusFailed, failureCode(err))
			}
			return wait("HOURLY_DISABLE_PENDING", err)
		}
	}
	if s.native == nil {
		return wait("HOURLY_NATIVE_PENDING", ErrUnavailable)
	}
	if err = s.native.SyncHourlyEnrollment(ctx, e.ID); err != nil {
		return wait("HOURLY_APPLICATION_PENDING", err)
	}
	// This durable target guard closes only terminal predecessor periods; a new
	// paid/preparing benefit prevents owner-wide retirement under its row lock.
	if err = s.native.CloseHourlyOwnerIfIdle(ctx, e.OwnerID, e.AccountGeneration, e.ID); err != nil {
		return wait("HOURLY_ACCOUNTING_PENDING", err)
	}
	return s.finish(ctx, op, item, providerops.StatusSucceeded, "")
}
