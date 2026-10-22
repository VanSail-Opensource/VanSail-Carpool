// Package entitlements applies durable purchase state to Remnawave.
package entitlements

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/accounts"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/outbox"
)

// Repository is the durable outbox and entitlement query surface.
type Repository interface {
	ClaimOutboxJob(context.Context, time.Time) (*model.OutboxJob, error)
	CompleteOutboxJob(context.Context, string, int, error, time.Time) error
	PurchaseByID(context.Context, string) (model.Purchase, error)
	UserForPurchase(context.Context, string) (model.User, error)
	UserByID(context.Context, string) (model.User, error)
	DesiredEntitlement(context.Context, string, time.Time) (*model.Purchase, error)
	ContinuityEntitlement(context.Context, string, time.Time) (*model.Purchase, error)
	PurchaseTrafficResetPhase(context.Context, string) (string, error)
	AdvancePurchaseTrafficReset(context.Context, string, string, string, time.Time) error
	MarkPurchaseSyncResult(context.Context, string, bool, time.Time) error
	ExpirePurchase(context.Context, string, time.Time) error
}

type userSyncNotificationReleaser interface {
	ReleaseUserSyncNotifications(context.Context, string, time.Time) error
}

// RemnawaveClient atomically replaces the complete entitlement view upstream.
type RemnawaveClient interface {
	ApplyEntitlement(ctx context.Context, remoteUserID string, trafficLimitBytes int64, resetStrategy string, squadUUIDs []string, expiresAt time.Time) error
	ResetTraffic(context.Context, string) error
	RemoveEntitlement(ctx context.Context, remoteUserID string) error
}

// Worker processes retryable external synchronization one job at a time.
type Worker struct {
	repository Repository
	remnawave  RemnawaveClient
	identity   IdentityClient
	now        func() time.Time
}

// NewWorker creates an entitlement worker.
func NewWorker(repository Repository, remnawave RemnawaveClient, identity ...IdentityClient) *Worker {
	worker := &Worker{repository: repository, remnawave: remnawave, now: time.Now}
	if len(identity) > 0 {
		worker.identity = identity[0]
	}
	return worker
}

// Drain processes up to limit ready jobs and returns the first infrastructure error.
func (w *Worker) Drain(ctx context.Context, limit int) error {
	if limit <= 0 {
		limit = 20
	}
	for range limit {
		if err := ctx.Err(); err != nil {
			return err
		}
		job, err := w.repository.ClaimOutboxJob(ctx, w.now().UTC())
		if err != nil {
			return err
		}
		if job == nil {
			return nil
		}
		jobErr := w.HandleOutbox(ctx, *job)
		if err := w.repository.CompleteOutboxJob(ctx, job.ID, job.Attempts, jobErr, w.now().UTC()); err != nil {
			return err
		}
	}
	return nil
}

// HandleOutbox implements the shared kind-specific dispatcher contract.
func (w *Worker) HandleOutbox(ctx context.Context, job model.OutboxJob) error {
	jobErr := w.process(ctx, job)
	if jobErr != nil && job.Attempts >= 10 && job.Kind == "remna_apply_entitlement" {
		purchaseID, targetErr := outbox.TargetID(job, "purchaseId")
		if targetErr != nil {
			return errors.Join(jobErr, targetErr)
		}
		if markErr := w.repository.MarkPurchaseSyncResult(ctx, purchaseID, false, w.now().UTC()); markErr != nil {
			return errors.Join(jobErr, fmt.Errorf("mark terminal entitlement failure: %w", markErr))
		}
	}
	return jobErr
}

func (w *Worker) process(ctx context.Context, job model.OutboxJob) error {
	switch job.Kind {
	case outbox.ContinuityKind:
		return w.prepareContinuity(ctx, job)
	case "remna_apply_entitlement":
		purchaseID, err := outbox.TargetID(job, "purchaseId")
		if err != nil {
			return err
		}
		purchase, err := w.repository.PurchaseByID(ctx, purchaseID)
		if err != nil {
			return err
		}
		if purchase.Status == "cancelled" || purchase.Status == "expired" {
			return nil
		}
		if !w.now().UTC().Before(purchase.ValidUntil) {
			return w.repository.ExpirePurchase(ctx, purchase.ID, w.now().UTC())
		}
		user, err := w.repository.UserForPurchase(ctx, purchase.ID)
		if err != nil {
			return err
		}
		user, err = w.ensureIdentity(ctx, user)
		if errors.Is(err, accounts.ErrRemnawaveIdentityConflict) {
			return w.resolveIdentityConflict(ctx, user)
		}
		if err != nil {
			return err
		}
		if user.RemnaUserID == nil {
			return errors.New("user has no Remnawave identity")
		}
		phase, err := w.repository.PurchaseTrafficResetPhase(ctx, purchase.ID)
		if err != nil {
			return err
		}
		if phase == "pending" {
			// Removing squads before resetting makes ambiguous reset retries safe:
			// no traffic can accrue between two reset attempts.
			if err := w.remnawave.RemoveEntitlement(ctx, *user.RemnaUserID); err != nil {
				return w.repairMissingIdentity(ctx, user, fmt.Errorf("quiesce Remnawave entitlement: %w", err))
			}
			if err := w.repository.AdvancePurchaseTrafficReset(ctx, purchase.ID, "pending", "quiesced", w.now().UTC()); err != nil {
				return err
			}
			phase = "quiesced"
		}
		if phase == "quiesced" {
			if err := w.remnawave.ResetTraffic(ctx, *user.RemnaUserID); err != nil {
				return w.repairMissingIdentity(ctx, user, fmt.Errorf("reset Remnawave traffic: %w", err))
			}
			if err := w.repository.AdvancePurchaseTrafficReset(ctx, purchase.ID, "quiesced", "reset", w.now().UTC()); err != nil {
				return err
			}
			phase = "reset"
		}
		if phase != "reset" {
			return fmt.Errorf("unknown traffic reset phase %q", phase)
		}
		if err := w.remnawave.ApplyEntitlement(ctx, *user.RemnaUserID, purchase.TrafficLimitBytes, purchase.ResetStrategy, purchase.SquadUUIDs, purchase.ValidUntil); err != nil {
			return w.repairMissingIdentity(ctx, user, fmt.Errorf("apply Remnawave entitlement: %w", err))
		}
		return w.repository.MarkPurchaseSyncResult(ctx, purchase.ID, true, w.now().UTC())
	case "remna_sync_user":
		userID, err := outbox.TargetID(job, "userId")
		if err != nil {
			return err
		}
		user, err := w.repository.UserByID(ctx, userID)
		if err != nil {
			return err
		}
		if user.OnboardingState != "complete" {
			return nil
		}
		if repository, ok := w.repository.(provisioningRepository); ok {
			purchased, err := repository.HasProvisionablePurchase(ctx, userID, w.now().UTC())
			if err != nil {
				return err
			}
			if purchased {
				user, err = w.ensureIdentity(ctx, user)
				if errors.Is(err, accounts.ErrRemnawaveIdentityConflict) {
					return w.resolveIdentityConflict(ctx, user)
				}
				if err != nil {
					return err
				}
			}
		}
		if user.RemnaUserID == nil {
			return w.releaseUserSyncNotifications(ctx, userID)
		}
		desired, err := w.repository.DesiredEntitlement(ctx, user.ID, w.now().UTC())
		if err != nil {
			return err
		}
		if desired == nil {
			if err := w.remnawave.RemoveEntitlement(ctx, *user.RemnaUserID); err != nil {
				return w.repairMissingIdentity(ctx, user, err)
			}
			return w.releaseUserSyncNotifications(ctx, userID)
		}
		if err := w.remnawave.ApplyEntitlement(ctx, *user.RemnaUserID, desired.TrafficLimitBytes, desired.ResetStrategy, desired.SquadUUIDs, desired.ValidUntil); err != nil {
			return w.repairMissingIdentity(ctx, user, err)
		}
		return w.releaseUserSyncNotifications(ctx, userID)
	default:
		return fmt.Errorf("unknown outbox job kind %q", job.Kind)
	}
}

func (w *Worker) releaseUserSyncNotifications(ctx context.Context, userID string) error {
	repository, ok := w.repository.(userSyncNotificationReleaser)
	if !ok {
		return nil
	}
	return repository.ReleaseUserSyncNotifications(ctx, userID, w.now().UTC())
}
