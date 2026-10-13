package database

import (
	"context"
	"database/sql"
	"errors"
	"github.com/txyyddss/Remna-User-Panel/internal/usdt"
)

const usdtTransferSelect = `SELECT id,network,token,recipient,atomic_amount,amount,tx_hash,event_id,occurred_at,order_id,status,reason FROM usdt_transfers`

func scanUSDTTransfer(row rowScanner) (usdt.Transfer, error) {
	var t usdt.Transfer
	var occurred string
	err := row.Scan(&t.ID, &t.Network, &t.Token, &t.Recipient, &t.AtomicAmount, &t.Amount, &t.TxHash, &t.EventID, &occurred, &t.OrderID, &t.Status, &t.Reason)
	if errors.Is(err, sql.ErrNoRows) {
		return t, ErrNotFound
	}
	if err != nil {
		return t, err
	}
	t.OccurredAt, err = parseStamp(occurred)
	t.Verified, t.Finalized = true, true
	return t, err
}

// USDTCursor returns only fully processed progress, including canonical EVM hash.
func (s *Store) USDTCursor(ctx context.Context, network string) (usdt.Cursor, error) {
	var c usdt.Cursor
	err := s.db.QueryRowContext(ctx, "SELECT value,hash FROM usdt_scan_cursors WHERE network=?", network).Scan(&c.Value, &c.Hash)
	if errors.Is(err, sql.ErrNoRows) {
		return c, nil
	}
	return c, err
}

// USDTReady recovers transfers committed immediately before process interruption.
func (s *Store) USDTReady(ctx context.Context, limit int) ([]usdt.Transfer, error) {
	return s.usdtTransferList(ctx, " WHERE status='ready' ORDER BY received_at,id LIMIT ?", limit)
}

func (s *Store) usdtTransferList(ctx context.Context, where string, args ...any) ([]usdt.Transfer, error) {
	rows, err := s.db.QueryContext(ctx, usdtTransferSelect+where, args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	result := []usdt.Transfer{}
	for rows.Next() {
		t, err := scanUSDTTransfer(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, t)
	}
	return result, rows.Err()
}

// USDTReview uses stable receiving IDs for bounded keyset pagination.
func (s *Store) USDTReview(ctx context.Context, cursor string, limit int) (usdt.ReviewPage, error) {
	if limit < 1 || limit > 100 {
		limit = 50
	}
	items, err := s.usdtTransferList(ctx, " WHERE status='review' AND id>? ORDER BY id LIMIT ?", cursor, limit+1)
	page := usdt.ReviewPage{Items: items}
	if err != nil {
		return page, err
	}
	if len(items) > limit {
		page.Items = items[:limit]
		page.NextCursor = page.Items[limit-1].ID
	}
	return page, nil
}
