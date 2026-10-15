package app

import (
	"context"
	"errors"
	"strconv"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	payload "github.com/VanSail-Opensource/VanSail-Carpool/internal/outbox"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/database"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/outbox"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
)

type detectorReleaseRepository interface {
	RetiredDetectorReleaseUser(context.Context, string) (model.User, bool, error)
	ActiveAdminTemporaryBan(context.Context, string) (*database.AdminTemporaryBan, error)
	CompleteRetiredDetectorRelease(context.Context, string, time.Time) error
}
type detectorReleaseProvider interface {
	releaseRetiredDetector(context.Context, model.User) error
}

// This one-shot compatibility lane only releases proven pre-upgrade restrictions.
// It cannot ingest events, classify traffic, issue punishments or send messages.
func registerRetiredDetectorRelease(worker *outbox.Worker, store *database.Store, provider proxyProvider) error {
	return worker.Register("proxy_restriction_release", outbox.HandlerFunc(func(ctx context.Context, job model.OutboxJob) error {
		if provider.master == nil {
			return xnode.ErrUnavailable
		}
		return provider.master.DeferOperation(job, func(ctx context.Context) error {
			child, cancel := context.WithTimeout(ctx, 30*time.Second)
			defer cancel()
			return handleRetiredDetectorRelease(child, job, store, provider)
		})
	}))
}

func handleRetiredDetectorRelease(ctx context.Context, job model.OutboxJob, repo detectorReleaseRepository, provider detectorReleaseProvider) error {
	id, err := payload.TargetID(job, "userId")
	if err != nil {
		return err
	}
	user, pending, err := repo.RetiredDetectorReleaseUser(ctx, id)
	if err != nil || !pending {
		return err
	}
	manual, err := repo.ActiveAdminTemporaryBan(ctx, id)
	if err != nil {
		return err
	}
	if manual == nil {
		if err = provider.releaseRetiredDetector(ctx, user); err != nil {
			return err
		}
	}
	return repo.CompleteRetiredDetectorRelease(ctx, id, time.Now().UTC())
}

func (p proxyProvider) releaseRetiredDetector(ctx context.Context, user model.User) error {
	unlock := p.lockBridgeRead()
	defer unlock()
	identity, err := p.native.IdentityForUser(ctx, user.ID)
	if err != nil && !errors.Is(err, xnode.ErrNotFound) {
		return err
	}
	legacyID := ""
	if user.RemnaUserID != nil {
		legacyID = *user.RemnaUserID
	}
	if err == nil {
		legacyID = identity.LegacyUserID
		if err = p.store.SetXNodeAccessDisabled(ctx, identity.ProxyUserID, false); err != nil {
			return err
		}
		purchase, e := p.store.DesiredEntitlement(ctx, user.ID, time.Now().UTC())
		if e != nil {
			return e
		}
		id := strconv.FormatInt(identity.ProxyUserID, 10)
		if purchase == nil {
			err = p.native.CloseForReplacement(ctx, id)
		} else {
			err = p.native.ApplyPurchase(ctx, id, purchase.TrafficLimitBytes, purchase.ResetStrategy, purchase.ValidUntil)
		}
		if err != nil {
			return err
		}
	}
	bridge, err := p.legacy(ctx)
	if err != nil || !bridge || legacyID == "" {
		return err
	}
	var desired func(context.Context) (*model.Purchase, error)
	if identity.ProxyUserID > 0 {
		if err = p.reconcileBridgeUsage(ctx, identity); err != nil {
			return err
		}
		desired = func(callCtx context.Context) (*model.Purchase, error) {
			usage, err := p.store.XNodeLegacyUsage(callCtx, identity.ProxyUserID)
			if err != nil {
				return nil, err
			}
			return p.bridgePurchase(callCtx, identity, usage.UsedBytes)
		}
	}
	return releaseRetiredDetectorLegacy(ctx, p.remnaAdapter, p.store, user.ID, legacyID, desired)
}
