// Package rollover settles unused-traffic credits before automatic renewal.
package rollover

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/outbox"
)

var ErrRemoteUserMissing = errors.New("Remnawave user is missing")
var ErrPerNodeUsageUnavailable = errors.New("per-node rollover usage is unavailable")

type Repository interface {
	RolloverByPurchase(context.Context, string) (model.PurchaseRollover, error)
	UserForPurchase(context.Context, string) (model.User, error)
	PurchaseByID(context.Context, string) (model.Purchase, error)
	RolloverEligible(context.Context, string) (bool, error)
	MarkRolloverProcessing(context.Context, string, time.Time) error
	RecordRolloverCalculation(context.Context, string, model.RolloverUsageSummary, time.Time) (model.PurchaseRollover, error)
	FinalizeRollover(context.Context, string, int64, int64, string, time.Time) (model.PurchaseRollover, error)
}

type Remote interface {
	QuiesceForRollover(context.Context, string) error
	UsageSnapshotForRollover(context.Context, string, time.Time, time.Time) (UsageSnapshot, error)
}

// PeriodRemote lets providers settle an older purchase while its successor is
// already serving. Legacy providers keep the original account-wide operation.
type PeriodRemote interface {
	QuiesceRolloverPeriod(context.Context, string, time.Time, time.Time) error
}

type DailyUsage struct {
	Date  time.Time
	Bytes int64
}

type UsageSnapshot struct {
	LimitBytes          int64
	Strategy            string
	LastResetAt         *time.Time
	CurrentUsedBytes    *int64
	Daily               []DailyUsage
	WeightedUsedBytes   int64
	NodeSeriesAvailable bool
}

type Service struct {
	repository Repository
	remote     Remote
	now        func() time.Time
}

func NewService(repository Repository, remote Remote) *Service {
	return &Service{repository: repository, remote: remote, now: time.Now}
}

func (s *Service) HandleOutbox(ctx context.Context, job model.OutboxJob) error {
	if job.Kind != "rollover_finalize" {
		return fmt.Errorf("unsupported rollover job %q", job.Kind)
	}
	purchaseID, err := outbox.TargetID(job, "purchaseId")
	if err != nil {
		return err
	}
	rollover, err := s.repository.RolloverByPurchase(ctx, purchaseID)
	if err != nil {
		return err
	}
	if rolloverTerminal(rollover.Status) || rollover.Status == "calculated" {
		return nil
	}
	purchase, err := s.repository.PurchaseByID(ctx, purchaseID)
	if err != nil {
		return err
	}
	if purchase.Status == "cancelled" {
		return nil
	}
	eligible, err := s.repository.RolloverEligible(ctx, purchaseID)
	if err != nil {
		return err
	}
	if !eligible {
		return s.finalizeWithoutCalculation(ctx, purchaseID, rollover, "")
	}
	user, err := s.repository.UserForPurchase(ctx, purchaseID)
	if err != nil {
		return err
	}
	if user.RemnaUserID == nil {
		return s.finalizeWithoutCalculation(ctx, purchaseID, rollover, "local_identity_missing")
	}
	if rollover.Status == "pending" {
		var err error
		if remote, ok := s.remote.(PeriodRemote); ok {
			err = remote.QuiesceRolloverPeriod(ctx, *user.RemnaUserID, purchase.ValidFrom, purchase.ValidUntil)
		} else {
			err = s.remote.QuiesceForRollover(ctx, *user.RemnaUserID)
		}
		if err != nil {
			if errors.Is(err, ErrRemoteUserMissing) {
				return s.finalizeWithoutCalculation(ctx, purchaseID, rollover, "remnawave_user_missing")
			}
			return fmt.Errorf("quiesce rollover: %w", err)
		}
		if err := s.repository.MarkRolloverProcessing(ctx, purchaseID, s.now().UTC()); err != nil {
			return err
		}
	}
	snapshot, err := s.remote.UsageSnapshotForRollover(ctx, *user.RemnaUserID, purchase.ValidFrom, purchase.ValidUntil)
	if errors.Is(err, ErrRemoteUserMissing) {
		return s.finalizeWithoutCalculation(ctx, purchaseID, rollover, "remnawave_user_missing")
	}
	if err != nil {
		return fmt.Errorf("fetch per-node rollover traffic: %w", err)
	}
	if !snapshot.NodeSeriesAvailable {
		return ErrPerNodeUsageUnavailable
	}
	_, err = s.repository.RecordRolloverCalculation(ctx, purchaseID, CalculateUsage(purchase, rollover.MinimumRemainingBPS, snapshot), s.now().UTC())
	return err
}

func (s *Service) finalizeWithoutCalculation(ctx context.Context, purchaseID string, rollover model.PurchaseRollover, exception string) error {
	if rollover.Status == "pending" {
		if err := s.repository.MarkRolloverProcessing(ctx, purchaseID, s.now().UTC()); err != nil {
			return err
		}
	}
	_, err := s.repository.FinalizeRollover(ctx, purchaseID, 0, 0, exception, s.now().UTC())
	return err
}

func rolloverTerminal(status string) bool {
	return status == "credited" || status == "zero" || status == "exception"
}

const UsageAlgorithmVersion = "cadence-v4"

// CalculateUsage derives cadence allowances from weighted per-node daily usage.
