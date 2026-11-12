package admin

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
)

type courtesyCreditRepository interface {
	CourtesyCreditPayment(context.Context, string, string, string, time.Time) (model.CourtesyCredit, error)
}

// AdjustBalanceWithReference appends an audited adjustment with a caller-owned idempotency reference.
func (s *Service) AdjustBalanceWithReference(ctx context.Context, actorID, userID string, delta int64, referenceID, reason string) (model.LedgerEntry, error) {
	reason = strings.TrimSpace(reason)
	if delta == 0 || delta < -1_000_000_000_000 || delta > 1_000_000_000_000 || reason == "" || len(reason) > 500 || strings.TrimSpace(referenceID) == "" {
		return model.LedgerEntry{}, errors.New("non-zero delta and reason are required")
	}
	return s.repository.AdjustAdminBalance(ctx, actorID, userID, delta, referenceID, reason, s.now().UTC())
}

// DeductBalanceWithReference appends an audited no-debt deduction with a stable idempotency reference.
func (s *Service) DeductBalanceWithReference(ctx context.Context, actorID, userID string, amount int64, referenceID, reason string) (model.LedgerEntry, error) {
	reason = strings.TrimSpace(reason)
	if amount <= 0 || amount > 1_000_000_000_000 || reason == "" || len(reason) > 500 || strings.TrimSpace(referenceID) == "" {
		return model.LedgerEntry{}, errors.New("positive amount and reason are required")
	}
	return s.repository.DeductAdminBalance(ctx, actorID, userID, amount, referenceID, reason, s.now().UTC())
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
	return s.repository.CancelAdminPurchase(ctx, actorID, purchaseID, reason, s.now().UTC())
}

// RunBackup executes a verified online backup and audits the request.
func (s *Service) RunBackup(ctx context.Context, actorID string) (model.BackupRun, error) {
	if s.backups == nil {
		return model.BackupRun{}, errors.New("backup creation is unavailable")
	}
	return s.backups.RunForActor(ctx, actorID)
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
