package database

import (
	"context"
	"database/sql"
	"errors"
	"strconv"
	"strings"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/notifications"
	jobpayload "github.com/VanSail-Opensource/VanSail-Carpool/internal/outbox"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/purchaseops"
)

type automaticResetCandidate struct {
	purchaseID string
	userID     string
	combo      string
	validFrom  time.Time
}

// ProcessAutomaticTrafficResetObservation revalidates and records one reset period atomically.
func (s *Store) ProcessAutomaticTrafficResetObservation(ctx context.Context, remoteID string, used, limit int64,
	remoteReset string, lastReset *time.Time, now time.Time) (notifications.AutomaticResetResult, error) {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return notifications.AutomaticResetResult{}, err
	}
	defer func() { _ = tx.Rollback() }()
	candidate, err := automaticResetCandidateTx(ctx, tx, remoteID, now)
	if errors.Is(err, sql.ErrNoRows) {
		return notifications.AutomaticResetResult{}, nil
	}
	if err != nil {
		return notifications.AutomaticResetResult{}, err
	}
	facts, err := memberPurchaseFactsTx(ctx, tx, candidate.purchaseID, candidate.userID)
	if err != nil || !memberOperationEligible(facts, true, now) {
		return notifications.AutomaticResetResult{}, err
	}
	anchor := candidate.validFrom.UTC()
	if lastReset != nil {
		anchor = lastReset.UTC()
	}
	period := anchor.Format(time.RFC3339Nano)
	input := purchaseops.AutomaticTrafficResetCommand(candidate.userID, candidate.purchaseID, period)
	if existing, found, replayErr := memberOperationReplayTx(ctx, tx, input, now); found || replayErr != nil {
		_ = existing
		if replayErr == nil {
			replayErr = tx.Commit()
		}
		return notifications.AutomaticResetResult{Handled: true}, replayErr
	}
	if duplicate, err := resetOperationSinceTx(ctx, tx, candidate.purchaseID, anchor); err != nil || duplicate {
		if err == nil {
			err = tx.Commit()
		}
		return notifications.AutomaticResetResult{Handled: true}, err
	}
	price, valid := purchaseops.ResetPriceMinor(facts.CoreGrossMinor, facts.Purchase.ResetStrategy)
	if !valid {
		return notifications.AutomaticResetResult{}, nil
	}
	balance, err := balanceTx(ctx, tx, candidate.userID)
	if err != nil {
		return notifications.AutomaticResetResult{}, err
	}
	reset := notificationReset(remoteReset, facts.Purchase.ResetStrategy)
	if balance < price {
		return s.disableAutomaticResetTx(ctx, tx, candidate, period, used, limit, reset, price, balance, now)
	}
	operationID, err := insertMemberOperationTx(ctx, tx, input, now)
	if err != nil {
		return notifications.AutomaticResetResult{}, err
	}
	balance, err = changeBalanceTx(ctx, tx, candidate.userID, -price, now)
	if err != nil {
		return notifications.AutomaticResetResult{}, err
	}
	if _, err := insertLedgerTx(ctx, tx, candidate.userID, -price, balance, "traffic_reset_debit", operationID, "automatic traffic reset", now); err != nil {
		return notifications.AutomaticResetResult{}, err
	}
	created, err := s.insertUserNotificationTx(ctx, tx, "automatic-reset:"+candidate.purchaseID+":"+period, candidate.userID,
		jobpayload.UserEventAutomaticReset, providerItemGate(operationID, "purchase"), automaticResetFacts(candidate.combo, used, limit, reset, price, balance, now), now)
	if err != nil {
		return notifications.AutomaticResetResult{}, err
	}
	if err := tx.Commit(); err != nil {
		return notifications.AutomaticResetResult{}, err
	}
	return notifications.AutomaticResetResult{Handled: true, EventCreated: created}, nil
}

func automaticResetCandidateTx(ctx context.Context, tx *sql.Tx, remoteID string, now time.Time) (automaticResetCandidate, error) {
	var result automaticResetCandidate
	var validFrom string
	err := tx.QueryRowContext(ctx, `SELECT purchases.id,purchases.user_id,combos.name,purchases.valid_from
		FROM users JOIN purchases ON purchases.user_id=users.id JOIN combos ON combos.id=purchases.combo_id
		WHERE users.remna_user_id=? AND users.auto_traffic_reset_enabled=1 AND purchases.status='active'
		AND purchases.valid_from<=? AND purchases.valid_until>? ORDER BY purchases.valid_from DESC LIMIT 1`,
		strings.TrimSpace(remoteID), stamp(now), stamp(now)).Scan(&result.purchaseID, &result.userID, &result.combo, &validFrom)
	if err == nil {
		result.validFrom, err = parseStamp(validFrom)
	}
	return result, err
}

func resetOperationSinceTx(ctx context.Context, tx *sql.Tx, purchaseID string, anchor time.Time) (bool, error) {
	var count int
	err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM provider_operations operation
		JOIN provider_operation_items item ON item.operation_id=operation.id
		WHERE operation.kind=? AND item.target_type='purchase' AND item.target_id=?
		AND operation.status NOT IN ('failed','compensated') AND operation.created_at>=?`,
		purchaseops.OperationResetKind, purchaseID, stamp(anchor)).Scan(&count)
	return count > 0, err
}

func (s *Store) disableAutomaticResetTx(ctx context.Context, tx *sql.Tx, candidate automaticResetCandidate, period string,
	used, limit int64, reset string, price, balance int64, now time.Time) (notifications.AutomaticResetResult, error) {
	if _, err := tx.ExecContext(ctx, `UPDATE users SET auto_traffic_reset_enabled=0,updated_at=?
		WHERE id=? AND auto_traffic_reset_enabled=1`, stamp(now), candidate.userID); err != nil {
		return notifications.AutomaticResetResult{}, err
	}
	created, err := s.insertUserNotificationTx(ctx, tx, "automatic-reset-insufficient:"+candidate.purchaseID+":"+period,
		candidate.userID, jobpayload.UserEventAutomaticResetInsufficient, "", automaticResetFacts(candidate.combo, used, limit, reset, price, balance, now), now)
	if err != nil {
		return notifications.AutomaticResetResult{}, err
	}
	if err := tx.Commit(); err != nil {
		return notifications.AutomaticResetResult{}, err
	}
	return notifications.AutomaticResetResult{Handled: true, EventCreated: created}, nil
}

func automaticResetFacts(combo string, used, limit int64, reset string, charge, balance int64, now time.Time) map[string]string {
	return map[string]string{notifications.FactCombo: combo, notifications.FactUsed: strconv.FormatInt(used, 10),
		notifications.FactTrafficLimit: strconv.FormatInt(limit, 10), notifications.FactReset: reset,
		notifications.FactCharge: strconv.FormatInt(charge, 10), notifications.FactBalance: strconv.FormatInt(balance, 10),
		notifications.FactAutomationState: "disabled", notifications.FactTime: now.UTC().Format(time.RFC3339Nano)}
}

func notificationReset(remote, local string) string {
	switch remote {
	case "DAY", "WEEK", "MONTH_ROLLING", "NO_RESET":
		return remote
	default:
		return local
	}
}
