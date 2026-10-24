package database

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"math/big"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/usdt"
)

// SaveUSDTBatch durably stores verified receiving events before advancing progress.
func (s *Store) retryAttempt_SaveUSDTBatch(ctx context.Context, network string, b usdt.Batch) error {
	if !IsPostgres(s.db) {
		s.writeMu.Lock()
		defer s.writeMu.Unlock()
	}
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
		if !t.Finalized || !t.Verified || t.Network != spec.Network || t.Token != spec.Token || len(t.TxHash) == 0 || len(t.TxHash) > 128 || len(t.EventID) == 0 || len(t.EventID) > 128 || t.OccurredAt.IsZero() {
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
		digest := sha256.Sum256([]byte(spec.Network + "\x00" + t.TxHash + "\x00" + t.EventID))
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
		q, matched, err := matchUSDTQuote(ctx, tx, t)
		if err != nil {
			return err
		}
		if matched {
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
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO usdt_transfers(id,network,token,recipient,atomic_amount,amount,tx_hash,event_id,occurred_at,order_id,status,reason,received_at) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?)`, t.ID, t.Network, t.Token, t.Recipient, t.AtomicAmount, t.Amount, t.TxHash, t.EventID, stamp(t.OccurredAt), t.OrderID, t.Status, t.Reason, stamp(time.Now().UTC())); err != nil {
			return err
		}
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO usdt_scan_cursors(network,token,value,hash,updated_at) VALUES(?,?,?,?,?) ON CONFLICT(network,token) DO UPDATE SET value=excluded.value,hash=excluded.hash,updated_at=excluded.updated_at`, spec.Network, spec.Token, b.Cursor.Value, b.Cursor.Hash, stamp(time.Now().UTC())); err != nil {
		return err
	}
	return tx.Commit()
}

func matchUSDTQuote(ctx context.Context, tx *sql.Tx, transfer usdt.Transfer) (usdt.Quote, bool, error) {
	exact, err := scanUSDTQuote(tx.QueryRowContext(ctx, usdtQuoteSelect+" WHERE network=? AND token=? AND recipient=? AND atomic_amount=?", transfer.Network, transfer.Token, transfer.Recipient, transfer.AtomicAmount))
	if err == nil {
		return exact, true, nil
	}
	if !errors.Is(err, ErrNotFound) {
		return usdt.Quote{}, false, err
	}
	spec, ok := usdt.TokenNetwork(transfer.Network, transfer.Token)
	if !ok || usdt.AmountShortfallTolerance(spec.Decimals).Sign() == 0 {
		return usdt.Quote{}, false, nil
	}
	amount, ok := new(big.Int).SetString(transfer.AtomicAmount, 10)
	if !ok || amount.Sign() <= 0 {
		return usdt.Quote{}, false, usdt.ErrProof
	}
	tolerance := usdt.AmountShortfallTolerance(spec.Decimals)
	maximum := new(big.Int).Add(new(big.Int).Set(amount), tolerance).String()
	minimum := amount.String()
	rows, err := tx.QueryContext(ctx, usdtQuoteSelect+` WHERE network=? AND token=? AND recipient=?
		AND (length(atomic_amount)>length(?) OR (length(atomic_amount)=length(?) AND atomic_amount>?))
		AND (length(atomic_amount)<length(?) OR (length(atomic_amount)=length(?) AND atomic_amount<=?))
		ORDER BY length(atomic_amount),atomic_amount LIMIT 2`,
		transfer.Network, transfer.Token, transfer.Recipient,
		minimum, minimum, minimum, maximum, maximum, maximum)
	if err != nil {
		return usdt.Quote{}, false, err
	}
	defer func() { _ = rows.Close() }()
	var candidate usdt.Quote
	count := 0
	for rows.Next() {
		candidate, err = scanUSDTQuote(rows)
		if err != nil {
			return usdt.Quote{}, false, err
		}
		count++
	}
	if err := rows.Err(); err != nil {
		return usdt.Quote{}, false, err
	}
	if count != 1 {
		return usdt.Quote{}, false, nil
	}
	return candidate, true, nil
}
