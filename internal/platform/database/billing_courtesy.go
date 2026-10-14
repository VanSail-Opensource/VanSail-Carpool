package database

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/notifications"
	jobpayload "github.com/VanSail-Opensource/VanSail-Carpool/internal/outbox"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/ids"
)

const courtesyCreditSelect = `SELECT id,payment_order_id,actor_user_id,txb_minor,ledger_entry_id,reason,created_at FROM courtesy_credits`

// CourtesyCreditPayment appends one administrator-authorized local credit for
// a failed or expired payment order. It never changes the payment's provider
// settlement state, and its ledger and audit records commit atomically.
func (s *Store) CourtesyCreditPayment(ctx context.Context, actorID, orderID, reason string, now time.Time) (model.CourtesyCredit, error) {
	actorID = strings.TrimSpace(actorID)
	orderID = strings.TrimSpace(orderID)
	reason = strings.TrimSpace(reason)
	if actorID == "" || orderID == "" || len(reason) < 3 || len(reason) > 500 {
		return model.CourtesyCredit{}, errors.New("a courtesy-credit actor, payment order, and reason of 3 to 500 bytes are required")
	}
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return model.CourtesyCredit{}, fmt.Errorf("begin payment courtesy credit: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	if existing, loadErr := courtesyCreditByOrderTx(ctx, tx, orderID); loadErr == nil {
		existing.Replayed = true
		return existing, nil
	} else if !errors.Is(loadErr, ErrNotFound) {
		return model.CourtesyCredit{}, loadErr
	}
	order, err := paymentOrderTx(ctx, tx, orderID)
	if err != nil {
		return model.CourtesyCredit{}, err
	}
	if order.Status != "failed" && order.Status != "expired" {
		return model.CourtesyCredit{}, ErrConflict
	}
	now = now.UTC()
	balance, err := adjustBalanceTx(ctx, tx, order.UserID, order.TXBMinor, now)
	if err != nil {
		return model.CourtesyCredit{}, fmt.Errorf("credit terminal payment order: %w", err)
	}
	ledgerID, err := insertLedgerTx(ctx, tx, order.UserID, order.TXBMinor, balance, "payment_courtesy_credit", order.ID, reason, now)
	if err != nil {
		return model.CourtesyCredit{}, err
	}
	creditID, err := ids.New()
	if err != nil {
		return model.CourtesyCredit{}, err
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO courtesy_credits(id,payment_order_id,actor_user_id,txb_minor,ledger_entry_id,reason,created_at)
		VALUES(?,?,?,?,?,?,?)`, creditID, order.ID, actorID, order.TXBMinor, ledgerID, reason, stamp(now)); err != nil {
		return model.CourtesyCredit{}, fmt.Errorf("record payment courtesy credit: %w", err)
	}
	detail, err := json.Marshal(map[string]any{"creditId": creditID, "ledgerEntryId": ledgerID, "amountMinor": order.TXBMinor, "reason": reason})
	if err != nil {
		return model.CourtesyCredit{}, fmt.Errorf("encode payment courtesy audit: %w", err)
	}
	auditID, err := ids.New()
	if err != nil {
		return model.CourtesyCredit{}, err
	}
	if err := insertAuditTx(ctx, tx, auditID, &actorID, "payment.courtesy_credit", "payment", order.ID, string(detail), now); err != nil {
		return model.CourtesyCredit{}, fmt.Errorf("append payment courtesy audit: %w", err)
	}
	facts := adminFinanceFacts("courtesy_credit", 0, balance, reason, now)
	delete(facts, notifications.FactAmount)
	facts[notifications.FactCredited] = fmt.Sprintf("%d", order.TXBMinor)
	if _, err := s.insertUserNotificationTx(ctx, tx, "admin:"+creditID+":"+order.UserID, order.UserID,
		jobpayload.UserEventAdminUpdate, "", facts, now); err != nil {
		return model.CourtesyCredit{}, err
	}
	if err := tx.Commit(); err != nil {
		return model.CourtesyCredit{}, fmt.Errorf("commit payment courtesy credit: %w", err)
	}
	return model.CourtesyCredit{ID: creditID, PaymentOrderID: order.ID, ActorUserID: actorID, TXB: model.TXBMoney(order.TXBMinor),
		LedgerEntryID: ledgerID, Reason: reason, CreatedAt: now}, nil
}

func courtesyCreditByOrderTx(ctx context.Context, tx *sql.Tx, orderID string) (model.CourtesyCredit, error) {
	var credit model.CourtesyCredit
	var txbMinor int64
	var created string
	err := tx.QueryRowContext(ctx, courtesyCreditSelect+` WHERE payment_order_id=?`, orderID).Scan(
		&credit.ID, &credit.PaymentOrderID, &credit.ActorUserID, &txbMinor, &credit.LedgerEntryID, &credit.Reason, &created,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return model.CourtesyCredit{}, ErrNotFound
	}
	if err != nil {
		return model.CourtesyCredit{}, fmt.Errorf("scan payment courtesy credit: %w", err)
	}
	credit.TXB = model.TXBMoney(txbMinor)
	credit.CreatedAt, err = parseStamp(created)
	if err != nil {
		return model.CourtesyCredit{}, err
	}
	return credit, nil
}

func courtesyCreditExistsTx(ctx context.Context, tx *sql.Tx, orderID string) (bool, error) {
	var matched int
	err := tx.QueryRowContext(ctx, `SELECT 1 FROM courtesy_credits WHERE payment_order_id=?`, orderID).Scan(&matched)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("load payment courtesy credit: %w", err)
	}
	return true, nil
}
