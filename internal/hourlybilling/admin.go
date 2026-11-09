package hourlybilling

import (
	"context"
	"strings"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/database"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/providerops"
)

// DisableForAdmin queues the existing xnode_hourly_disable operation on behalf
// of an administrator. It never weakens the member authorization path: the
// actor and owner are distinct, the sealed target is marked administrative, and
// the target must still belong to the owner's current account generation.
func (s *Service) DisableForAdmin(ctx context.Context, actorID, ownerID, enrollmentID, version, key string) (model.OperationReceipt, error) {
	actorID, ownerID, enrollmentID = strings.TrimSpace(actorID), strings.TrimSpace(ownerID), strings.TrimSpace(enrollmentID)
	key = strings.TrimSpace(key)
	if actorID == "" || ownerID == "" || enrollmentID == "" || len(version) != 64 || key == "" || len(key) > 96 || s.store == nil {
		return model.OperationReceipt{}, database.ErrConflict
	}
	owner, err := s.store.UserByID(ctx, ownerID)
	if err != nil {
		return model.OperationReceipt{}, err
	}
	if owner.AccountStatus != "active" {
		return model.OperationReceipt{}, database.ErrAccountUnavailable
	}
	enrollment, err := s.store.HourlyEnrollment(ctx, enrollmentID)
	if err != nil {
		return model.OperationReceipt{}, err
	}
	if enrollment.OwnerID != owner.ID || enrollment.GroupID == "" {
		return model.OperationReceipt{}, database.ErrNotFound
	}
	if enrollment.AccountGeneration != owner.AccountGeneration {
		return model.OperationReceipt{}, database.ErrConflict
	}
	target := operationTarget{Generation: owner.AccountGeneration, GroupID: enrollment.GroupID,
		EnrollmentID: enrollment.ID, PolicyVersion: version, Admin: true}
	// A replay returns the original receipt even after the enrollment stopped.
	if receipt, found, replayErr := s.store.ProviderOperationForActorKey(ctx, actorID, DisableOperation,
		operationActionKey(target.Generation, key), targetFingerprint(DisableOperation, owner.ID, target)); found || replayErr != nil {
		return receipt, replayErr
	}
	if enrollment.Status == model.HourlyDisabled || enrollment.Status == model.HourlyClosed {
		return model.OperationReceipt{}, database.ErrConflict
	}
	if s.policy == nil {
		return model.OperationReceipt{}, ErrUnavailable
	}
	snapshot, err := s.policy.HourlyPolicy(ctx)
	if err != nil {
		return model.OperationReceipt{}, err
	}
	if snapshot.Version != version {
		return model.OperationReceipt{}, database.ErrConflict
	}
	return s.queue(ctx, actorID, owner.ID, DisableOperation, key, target)
}

// adminDisableOperation executes a shared xnode_hourly_disable receipt with a
// distinct administrator actor. The paid hour is retained and no member
// cooldown is created; every native phase matches the member stop.
func (s *Service) adminDisableOperation(ctx context.Context, op providerops.Operation, item providerops.Item, target operationTarget) error {
	if !target.Admin || op.Receipt.Kind != DisableOperation {
		return s.finish(ctx, op, item, providerops.StatusFailed, "HOURLY_OPERATION_INVALID")
	}
	e, err := s.store.HourlyEnrollment(ctx, target.EnrollmentID)
	if err != nil {
		return wait("HOURLY_DISABLE_TARGET_PENDING", err)
	}
	if e.OwnerID != op.OwnerUserID || e.AccountGeneration != target.Generation || e.GroupID != target.GroupID {
		return wait("HOURLY_DISABLE_TARGET_PENDING", database.ErrConflict)
	}
	if e.Status != model.HourlyDisabled && e.Status != model.HourlyClosed {
		snapshot, err := s.policy.HourlyPolicy(ctx)
		if err != nil {
			return wait("HOURLY_POLICY_PENDING", err)
		}
		if snapshot.Version != target.PolicyVersion {
			return s.finish(ctx, op, item, providerops.StatusFailed, "HOURLY_TERMS_CHANGED")
		}
		e, err = s.store.DisableAdminHourlyEnrollment(ctx, database.AdminHourlyDisableInput{OwnerID: op.OwnerUserID,
			AccountGeneration: target.Generation, EnrollmentID: target.EnrollmentID}, snapshot, s.now())
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
	// The durable target guard closes only terminal predecessor periods; a new
	// paid/preparing benefit prevents owner-wide retirement under its row lock.
	if err = s.native.CloseHourlyOwnerIfIdle(ctx, e.OwnerID, e.AccountGeneration, e.ID); err != nil {
		return wait("HOURLY_ACCOUNTING_PENDING", err)
	}
	return s.finish(ctx, op, item, providerops.StatusSucceeded, "")
}
