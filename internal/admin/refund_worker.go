package admin

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	jobpayload "github.com/VanSail-Opensource/VanSail-Carpool/internal/outbox"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/providerops"
)

type paymentRefundReconciler interface {
	PaymentRefunder
	ProviderRefunded(context.Context, model.PaymentOrder) (bool, error)
	DefinitiveRefundFailure(error) bool
}

func (w *MutationWorker) handlePaymentRefund(ctx context.Context, operation providerops.Operation, job model.OutboxJob) error {
	operation, item, fresh, err := w.start(ctx, operation, "payment_order")
	if err != nil {
		return err
	}
	reason, err := refundReason(job)
	if err != nil {
		return err
	}
	order, err := w.repository.PaymentOrderByID(ctx, item.TargetID)
	if err != nil {
		return w.complete(ctx, operation, item, providerops.StatusFailed, "PAYMENT_NOT_FOUND")
	}
	if order.Status == "refunded" {
		return w.complete(ctx, operation, item, providerops.StatusSucceeded, "")
	}
	if order.Status != "paid" {
		return w.complete(ctx, operation, item, providerops.StatusFailed, "PAYMENT_NOT_REFUNDABLE")
	}
	reconciler, providerAware := w.service.refunder.(paymentRefundReconciler)
	if order.Provider == "stars" && !providerAware {
		return w.complete(ctx, operation, item, providerops.StatusPendingReview, "REFUND_RECONCILIATION_UNAVAILABLE")
	}
	if order.Provider == "stars" && !fresh {
		return w.reconcileRefund(ctx, operation, item, order, reason, reconciler)
	}
	if order.Provider == "stars" {
		if callErr := reconciler.RefundProvider(ctx, order); callErr != nil {
			refunded, reconcileErr := reconciler.ProviderRefunded(ctx, order)
			if reconcileErr == nil && refunded {
				return w.applyLocalRefund(ctx, operation, item, order, reason)
			}
			if reconciler.DefinitiveRefundFailure(callErr) {
				return w.complete(ctx, operation, item, providerops.StatusFailed, "REFUND_REJECTED")
			}
			return w.complete(ctx, operation, item, providerops.StatusPendingReview, "REFUND_OUTCOME_AMBIGUOUS")
		}
	}
	return w.applyLocalRefund(ctx, operation, item, order, reason)
}

func (w *MutationWorker) reconcileRefund(ctx context.Context, operation providerops.Operation, item providerops.Item,
	order model.PaymentOrder, reason string, reconciler paymentRefundReconciler) error {
	refunded, err := reconciler.ProviderRefunded(ctx, order)
	if err == nil && refunded {
		return w.applyLocalRefund(ctx, operation, item, order, reason)
	}
	return w.complete(ctx, operation, item, providerops.StatusPendingReview, "REFUND_OUTCOME_AMBIGUOUS")
}

func (w *MutationWorker) applyLocalRefund(ctx context.Context, operation providerops.Operation, item providerops.Item,
	order model.PaymentOrder, reason string) error {
	actor := operation.ActorUserID
	if _, err := w.repository.RefundPayment(ctx, &actor, order.ID, reason, w.now().UTC()); err != nil {
		return err
	}
	return w.complete(ctx, operation, item, providerops.StatusSucceeded, "")
}

func refundReason(job model.OutboxJob) (string, error) {
	payload, err := jobpayload.TargetID(job, "sealedTarget")
	if err != nil {
		return "", err
	}
	var target refundTarget
	if json.Unmarshal([]byte(payload), &target) != nil || target.Reason == "" {
		return "", errors.New("payment refund target is invalid")
	}
	return target.Reason, nil
}
