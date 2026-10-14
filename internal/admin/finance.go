package admin

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/ids"
)

type courtesyCreditRepository interface {
	CourtesyCreditPayment(context.Context, string, string, string, time.Time) (model.CourtesyCredit, error)
}

type atomicBalanceRepository interface {
	AdjustAdminBalance(context.Context, string, string, int64, string, string, time.Time) (model.LedgerEntry, error)
	DeductAdminBalance(context.Context, string, string, int64, string, string, time.Time) (model.LedgerEntry, error)
}

type atomicCancellationRepository interface {
	CancelAdminPurchase(context.Context, string, string, string, time.Time) (model.Purchase, error)
}

// AdjustBalance appends an audited immutable ledger entry.
func (s *Service) AdjustBalance(ctx context.Context, actorID, userID string, delta int64, reason string) (model.LedgerEntry, error) {
	referenceID, err := ids.New()
	if err != nil {
		return model.LedgerEntry{}, err
	}
	return s.AdjustBalanceWithReference(ctx, actorID, userID, delta, referenceID, reason)
}

// AdjustBalanceWithReference appends an audited adjustment with a caller-owned idempotency reference.
func (s *Service) AdjustBalanceWithReference(ctx context.Context, actorID, userID string, delta int64, referenceID, reason string) (model.LedgerEntry, error) {
	reason = strings.TrimSpace(reason)
	if delta == 0 || delta < -1_000_000_000_000 || delta > 1_000_000_000_000 || reason == "" || len(reason) > 500 || strings.TrimSpace(referenceID) == "" {
		return model.LedgerEntry{}, errors.New("non-zero delta and reason are required")
	}
	if repository, ok := s.repository.(atomicBalanceRepository); ok {
		return repository.AdjustAdminBalance(ctx, actorID, userID, delta, referenceID, reason, s.now().UTC())
	}
	entry, err := s.repository.AdjustBalance(ctx, userID, delta, referenceID, reason, s.now().UTC())
	if err != nil {
		return model.LedgerEntry{}, err
	}
	if err := s.audit(ctx, actorID, "balance.adjust", "user", userID, map[string]any{"deltaMinor": delta, "reason": reason, "ledgerEntryId": entry.ID}); err != nil {
		return model.LedgerEntry{}, err
	}
	return entry, nil
}

// DeductBalance appends an audited exact debit that cannot create debt.
func (s *Service) DeductBalance(ctx context.Context, actorID, userID string, amount int64, reason string) (model.LedgerEntry, error) {
	referenceID, err := ids.New()
	if err != nil {
		return model.LedgerEntry{}, err
	}
	return s.DeductBalanceWithReference(ctx, actorID, userID, amount, referenceID, reason)
}

// DeductBalanceWithReference appends an audited no-debt deduction with a stable idempotency reference.
func (s *Service) DeductBalanceWithReference(ctx context.Context, actorID, userID string, amount int64, referenceID, reason string) (model.LedgerEntry, error) {
	reason = strings.TrimSpace(reason)
	if amount <= 0 || amount > 1_000_000_000_000 || reason == "" || len(reason) > 500 || strings.TrimSpace(referenceID) == "" {
		return model.LedgerEntry{}, errors.New("positive amount and reason are required")
	}
	if repository, ok := s.repository.(atomicBalanceRepository); ok {
		return repository.DeductAdminBalance(ctx, actorID, userID, amount, referenceID, reason, s.now().UTC())
	}
	entry, err := s.repository.DeductBalance(ctx, userID, amount, referenceID, reason, s.now().UTC())
	if err != nil {
		return model.LedgerEntry{}, err
	}
	if err := s.audit(ctx, actorID, "telegram.balance_deduct", "user", userID, map[string]any{"amountMinor": amount, "reason": reason, "ledgerEntryId": entry.ID}); err != nil {
		return model.LedgerEntry{}, err
	}
	return entry, nil
}

// Refund reverses one settled payment and applies the debt policy transactionally.
func (s *Service) Refund(ctx context.Context, actorID, orderID, reason string) (model.PaymentOrder, error) {
	reason = strings.TrimSpace(reason)
	if reason == "" || len(reason) > 500 {
		return model.PaymentOrder{}, errors.New("refund reason is required")
	}
	order, err := s.repository.PaymentOrderByID(ctx, orderID)
	if err != nil {
		return model.PaymentOrder{}, err
	}
	// Persist the administrator's intent before any irreversible provider call.
	// Telegram's refunded-payment webhook and reconciliation loop complete a
	// local reversal if the process stops after a successful Stars refund.
	if err := s.audit(ctx, actorID, "payment.refund", "payment", orderID, map[string]any{"reason": reason, "phase": "requested"}); err != nil {
		return model.PaymentOrder{}, err
	}
	if order.Status != "refunded" && s.refunder != nil {
		if err := s.refunder.RefundProvider(ctx, order); err != nil {
			return model.PaymentOrder{}, fmt.Errorf("refund provider payment: %w", err)
		}
	}
	order, err = s.repository.RefundPayment(ctx, &actorID, orderID, reason, s.now().UTC())
	if err != nil {
		return model.PaymentOrder{}, err
	}
	return order, nil
}

// CourtesyCredit credits a terminal failed or expired order locally without
// claiming that a provider payment succeeded.
func (s *Service) CourtesyCredit(ctx context.Context, actorID, orderID, reason string) (model.CourtesyCredit, error) {
	reason = strings.TrimSpace(reason)
	if strings.TrimSpace(actorID) == "" || len(reason) < 3 || len(reason) > 500 {
		return model.CourtesyCredit{}, errors.New("a courtesy-credit reason of 3 to 500 bytes is required")
	}
	creditor, ok := s.repository.(courtesyCreditRepository)
	if !ok {
		return model.CourtesyCredit{}, errors.New("courtesy credits are unavailable")
	}
	return creditor.CourtesyCreditPayment(ctx, actorID, orderID, reason, s.now().UTC())
}

// CancelEntitlement applies a compensating credit and revokes active upstream access.
func (s *Service) CancelEntitlement(ctx context.Context, actorID, purchaseID, reason string) (model.Purchase, error) {
	reason = strings.TrimSpace(reason)
	if reason == "" || len(reason) > 500 {
		return model.Purchase{}, errors.New("cancellation reason is required")
	}
	if repository, ok := s.repository.(atomicCancellationRepository); ok {
		return repository.CancelAdminPurchase(ctx, actorID, purchaseID, reason, s.now().UTC())
	}
	purchase, err := s.repository.CancelPurchase(ctx, purchaseID, reason, s.now().UTC())
	if err != nil {
		return model.Purchase{}, err
	}
	if err := s.audit(ctx, actorID, "entitlement.cancel", "purchase", purchaseID, map[string]any{"reason": reason}); err != nil {
		return model.Purchase{}, err
	}
	return purchase, nil
}

// RunBackup executes a verified online backup and audits the request.
func (s *Service) RunBackup(ctx context.Context, actorID string) (model.BackupRun, error) {
	run, err := s.backups.Run(ctx)
	if err != nil {
		return run, err
	}
	if err := s.audit(ctx, actorID, "backup.create", "backup", run.ID, nil); err != nil {
		return model.BackupRun{}, err
	}
	return run, nil
}

// RetryJob makes a failed synchronization eligible again.
func (s *Service) RetryJob(ctx context.Context, actorID, jobID string) error {
	if err := s.repository.RetryOutboxJob(ctx, jobID, s.now().UTC()); err != nil {
		return err
	}
	return s.audit(ctx, actorID, "job.retry", "job", jobID, nil)
}

func (s *Service) audit(ctx context.Context, actorID, action, targetType, targetID string, detail any) error {
	payload := "{}"
	if detail != nil {
		encoded, err := json.Marshal(detail)
		if err != nil {
			return fmt.Errorf("encode audit detail: %w", err)
		}
		payload = string(encoded)
	}
	return s.repository.AppendAudit(ctx, &actorID, action, targetType, targetID, payload, s.now().UTC())
}
