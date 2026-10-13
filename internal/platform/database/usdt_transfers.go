package database

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"time"

	"github.com/txyyddss/Remna-User-Panel/internal/usdt"
)

// SaveUSDTBatch durably stores verified receiving events before advancing progress.
func (s *Store) SaveUSDTBatch(ctx context.Context, network string, b usdt.Batch) error {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	spec, ok := usdt.Network(network)
	if !ok {
		return usdt.ErrProof
	}
	for _, t := range b.Transfers {
		if !t.Finalized || !t.Verified || t.Network != network || t.Token != spec.Token || len(t.TxHash) == 0 || len(t.TxHash) > 128 || len(t.EventID) == 0 || len(t.EventID) > 128 || t.OccurredAt.IsZero() {
			return usdt.ErrProof
		}
		address, err := usdt.NormalizeAddress(spec, t.Recipient)
		if err != nil || address != t.Recipient {
			return usdt.ErrProof
		}
		amount, err := usdt.TransferAmount(t.AtomicAmount, spec.Decimals)
		if err != nil || amount != t.Amount {
			return usdt.ErrProof
		}
		digest := sha256.Sum256([]byte(network + "\x00" + t.TxHash + "\x00" + t.EventID))
		t.ID = hex.EncodeToString(digest[:])
		old, err := scanUSDTTransfer(tx.QueryRowContext(ctx, usdtTransferSelect+" WHERE id=?", t.ID))
		if err == nil {
			if old.Token != t.Token || old.Recipient != t.Recipient || old.AtomicAmount != t.AtomicAmount || !old.OccurredAt.Equal(t.OccurredAt) {
				return usdt.ErrConflict
			}
			continue
		}
		if !errors.Is(err, ErrNotFound) {
			return err
		}
		t.Status, t.Reason = "review", "USDT_UNMATCHED"
		q, err := scanUSDTQuote(tx.QueryRowContext(ctx, usdtQuoteSelect+" WHERE network=? AND token=? AND recipient=? AND atomic_amount=?", network, t.Token, t.Recipient, t.AtomicAmount))
		if err == nil {
			t.OrderID = q.OrderID
			order, loadErr := paymentOrderTx(ctx, tx, q.OrderID)
			if loadErr == nil {
				t.Reason = "USDT_LATE_PAYMENT"
				if order.PaidAt != nil {
					t.Reason = "USDT_ALREADY_CREDITED"
				} else if !t.OccurredAt.Before(order.CreatedAt) && t.OccurredAt.Before(q.ExpiresAt) && (order.CancelledAt == nil || t.OccurredAt.Before(*order.CancelledAt)) {
					t.Status, t.Reason = "ready", ""
				}
			} else if !errors.Is(loadErr, ErrNotFound) {
				return loadErr
			}
		} else if !errors.Is(err, ErrNotFound) {
			return err
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO usdt_transfers(id,network,token,recipient,atomic_amount,amount,tx_hash,event_id,occurred_at,order_id,status,reason,received_at) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?)`, t.ID, t.Network, t.Token, t.Recipient, t.AtomicAmount, t.Amount, t.TxHash, t.EventID, stamp(t.OccurredAt), t.OrderID, t.Status, t.Reason, stamp(time.Now().UTC())); err != nil {
			return err
		}
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO usdt_scan_cursors(network,value,hash,updated_at) VALUES(?,?,?,?) ON CONFLICT(network) DO UPDATE SET value=excluded.value,hash=excluded.hash,updated_at=excluded.updated_at`, network, b.Cursor.Value, b.Cursor.Hash, stamp(time.Now().UTC())); err != nil {
		return err
	}
	return tx.Commit()
}
