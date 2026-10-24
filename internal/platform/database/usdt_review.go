package database

import (
	"context"
	"encoding/json"
	"math/big"
	"strings"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/ids"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/usdt"
)

// DismissUSDTTransfer preserves verified evidence and records a permanent decision.
// The same write lock/transaction boundary excludes scanner and settlement races.
func (s *Store) retryAttempt_DismissUSDTTransfer(ctx context.Context, id, actor, reason string) (bool, error) {
	reason = strings.TrimSpace(reason)
	if actor == "" || len(reason) < 3 || len(reason) > 500 {
		return false, usdt.ErrConfiguration
	}
	if !IsPostgres(s.db) {
		s.writeMu.Lock()
		defer s.writeMu.Unlock()
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return false, err
	}
	defer func() { _ = tx.Rollback() }()
	var role string
	if err := tx.QueryRowContext(ctx, "SELECT role FROM users WHERE id=?", actor).Scan(&role); err != nil {
		return false, err
	}
	if role != "admin" {
		return false, usdt.ErrConflict
	}
	t, err := scanUSDTTransfer(tx.QueryRowContext(ctx, usdtTransferSelect+" WHERE id=?", id))
	if err != nil {
		return false, err
	}
	if t.Status == "dismissed" {
		return false, nil
	}
	if t.Status != "review" {
		return false, usdt.ErrConflict
	}
	now := time.Now().UTC()
	if _, err := tx.ExecContext(ctx, "UPDATE usdt_transfers SET status='dismissed',dismissed_by=?,dismissed_reason=?,dismissed_at=? WHERE id=? AND status='review'", actor, reason, stamp(now), id); err != nil {
		return false, err
	}
	detail, err := json.Marshal(map[string]string{"transferId": id, "network": t.Network, "token": t.Token, "txHash": t.TxHash, "eventId": t.EventID, "reason": reason})
	if err != nil {
		return false, err
	}
	auditID, err := ids.New()
	if err != nil {
		return false, err
	}
	if err := insertAuditTx(ctx, tx, auditID, &actor, "usdt.transfer.dismiss", "usdt_transfer", id, string(detail), now); err != nil {
		return false, err
	}
	if err := tx.Commit(); err != nil {
		return false, err
	}
	return true, nil
}

// USDTMatchingEvidence returns only unique, same-token receiving evidence for the
// selected quote. Settlement rechecks the match under its own write transaction.
func (s *Store) retryAttempt_USDTMatchingEvidence(ctx context.Context, orderID string) (usdt.ReviewPage, error) {
	result := usdt.ReviewPage{Items: []usdt.Transfer{}}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return result, err
	}
	defer func() { _ = tx.Rollback() }()
	q, err := scanUSDTQuote(tx.QueryRowContext(ctx, usdtQuoteSelect+" WHERE order_id=?", orderID))
	if err != nil {
		return result, err
	}
	spec, ok := usdt.TokenNetwork(q.Network, q.Token)
	if !ok {
		return result, usdt.ErrProof
	}
	order, err := paymentOrderTx(ctx, tx, orderID)
	if err != nil {
		return result, err
	}
	credited, err := courtesyCreditExistsTx(ctx, tx, orderID)
	if err != nil {
		return result, err
	}
	if order.PaidAt != nil || credited || order.Status == "refunded" {
		return result, nil
	}
	minimum, ok := new(big.Int).SetString(q.AtomicAmount, 10)
	if !ok {
		return result, usdt.ErrProof
	}
	minimum.Sub(minimum, usdt.AmountShortfallTolerance(spec.Decimals))
	if minimum.Sign() < 0 {
		minimum.SetInt64(0)
	}
	lower := minimum.String()
	rows, err := tx.QueryContext(ctx, usdtTransferSelect+` WHERE network=? AND token=? AND recipient=? AND status IN ('review','ready')
	 AND occurred_at>=?
	 AND (length(atomic_amount)>length(?) OR (length(atomic_amount)=length(?) AND atomic_amount>=?))
	 AND (length(atomic_amount)<length(?) OR (length(atomic_amount)=length(?) AND atomic_amount<=?))
	 ORDER BY occurred_at DESC,id LIMIT 100`, q.Network, q.Token, q.Recipient, stamp(order.CreatedAt), lower, lower, lower, q.AtomicAmount, q.AtomicAmount, q.AtomicAmount)
	if err != nil {
		return result, err
	}
	var candidates []usdt.Transfer
	for rows.Next() {
		t, err := scanUSDTTransfer(rows)
		if err != nil {
			_ = rows.Close()
			return result, err
		}
		if t.AtomicAmount == q.AtomicAmount || usdt.IsAmountShortfallWithinTolerance(t.AtomicAmount, q.AtomicAmount, spec.Decimals) {
			candidates = append(candidates, t)
		}
	}
	rowErr, closeErr := rows.Err(), rows.Close()
	if rowErr != nil {
		return result, rowErr
	}
	if closeErr != nil {
		return result, closeErr
	}
	for _, t := range candidates {
		matched, found, err := matchUSDTQuote(ctx, tx, t)
		if err != nil {
			return result, err
		}
		if found && matched.OrderID == orderID {
			result.Items = append(result.Items, t)
		}
	}
	return result, nil
}
