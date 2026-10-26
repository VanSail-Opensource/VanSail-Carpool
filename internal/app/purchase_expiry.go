package app

import (
	"context"
	"errors"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	jobpayload "github.com/VanSail-Opensource/VanSail-Carpool/internal/outbox"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/database"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
)

// purchaseExpiry closes only predecessor periods. Their durable reconciliation
// continues independently, including when a paid successor is already serving.
func purchaseExpiry(store *database.Store, native *xnode.Service) func(context.Context, model.OutboxJob) error {
	return func(ctx context.Context, job model.OutboxJob) error {
		id, err := jobpayload.TargetID(job, "purchaseId")
		if err != nil {
			return err
		}
		p, err := store.PurchaseByID(ctx, id)
		if err != nil || p.ValidUntil.After(time.Now()) || p.Status == "cancelled" {
			return err
		}
		user, err := store.UserForPurchase(ctx, id)
		if err != nil {
			return err
		}
		closeErr := native.ClosePurchasePeriods(ctx, user, p)
		if errors.Is(closeErr, xnode.ErrNotFound) {
			closeErr = nil
		}
		return errors.Join(closeErr, store.ExpireDuePurchase(ctx, id, time.Now().UTC()))
	}
}
