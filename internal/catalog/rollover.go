package catalog

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/database"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/rollover"
)

var (
	// ErrRolloverNotActive means the requested term is not currently usable.
	ErrRolloverNotActive = errors.New("rollover purchase is not active")
	// ErrRolloverUnavailable means the live upstream projection cannot be built.
	ErrRolloverUnavailable = errors.New("rollover projection is unavailable")
)

type rolloverPurchaseReader interface {
	PurchaseByID(context.Context, string) (model.Purchase, error)
}

type rolloverSnapshotReader interface {
	UsageSnapshotForRollover(context.Context, string, time.Time, time.Time) (rollover.UsageSnapshot, error)
}

// RolloverProjection returns a fresh aggregate-only projection for the active
// member term. Raw Remnawave usage data remains inside the queued adapter.
func (s *Service) RolloverProjection(ctx context.Context, user model.User, purchaseID string) (model.RolloverProjection, error) {
	if user.OnboardingState != "complete" || strings.TrimSpace(purchaseID) == "" {
		return model.RolloverProjection{}, ErrRolloverUnavailable
	}
	reader, ok := s.repository.(rolloverPurchaseReader)
	if !ok {
		return model.RolloverProjection{}, ErrRolloverUnavailable
	}
	purchase, err := reader.PurchaseByID(ctx, strings.TrimSpace(purchaseID))
	if err != nil {
		return model.RolloverProjection{}, err
	}
	if purchase.UserID != user.ID {
		return model.RolloverProjection{}, database.ErrNotFound
	}
	now := s.now().UTC()
	if (purchase.Status != "active" && purchase.Status != "activating") || purchase.ValidUntil.Before(now) || purchase.ValidFrom.After(now) {
		return model.RolloverProjection{}, ErrRolloverNotActive
	}
	if !purchase.AutoRenewEnabled {
		warning := "AUTO_RENEWAL_DISABLED"
		return model.RolloverProjection{PurchaseID: purchase.ID, Paid: purchase.Price, AutoRenewalEnabled: false,
			WarningCode: &warning, MinimumRemainingBPS: purchase.RolloverMinRemainingBPS, FetchedAt: now}, nil
	}
	if user.RemnaUserID == nil {
		return model.RolloverProjection{}, ErrRolloverUnavailable
	}
	remote, ok := s.remnawave.(rolloverSnapshotReader)
	if !ok {
		return model.RolloverProjection{}, ErrRolloverUnavailable
	}
	end := now
	if end.After(purchase.ValidUntil) {
		end = purchase.ValidUntil
	}
	snapshot, err := remote.UsageSnapshotForRollover(ctx, *user.RemnaUserID, purchase.ValidFrom, end)
	if errors.Is(err, rollover.ErrRemoteUserMissing) || errors.Is(err, rollover.ErrPerNodeUsageUnavailable) {
		return model.RolloverProjection{}, ErrRolloverUnavailable
	}
	if err != nil {
		return model.RolloverProjection{}, fmt.Errorf("fetch rollover projection: %w", err)
	}
	if !snapshot.NodeSeriesAvailable {
		return model.RolloverProjection{}, ErrRolloverUnavailable
	}
	projection := rollover.ProjectUsage(purchase, purchase.RolloverMinRemainingBPS, snapshot, now)
	projection.FetchedAt = now
	return projection, nil
}
