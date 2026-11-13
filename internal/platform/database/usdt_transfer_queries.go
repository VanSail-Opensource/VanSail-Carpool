package database

import (
	"context"
	"database/sql"
	"errors"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/usdt"
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
	spec, ok := usdt.TokenNetwork(t.Network, t.Token)
	if !ok {
		return t, usdt.ErrProof
	}
	t.Currency = spec.Currency()
	t.OccurredAt, err = parseStamp(occurred)
	t.Verified, t.Finalized = true, true
	return t, err
}

// USDTCursor returns only fully processed progress, including canonical EVM hash.
func (s *Store) USDTCursor(ctx context.Context, network string) (usdt.Cursor, error) {
	var c usdt.Cursor
	spec, ok := usdt.Network(network)
	if !ok {
		return c, usdt.ErrConfiguration
	}
	// Bound materialization too: a retained legacy/corrupt cursor must not
	// allocate an unbounded string before validation can refuse it.
	var oversized bool
	err := s.db.QueryRowContext(ctx, `SELECT
 CASE WHEN octet_length(value)<=$3 THEN value ELSE '' END,
 CASE WHEN octet_length(hash)<=$4 THEN hash ELSE '' END,
 octet_length(value)>$3 OR octet_length(hash)>$4
 FROM usdt_scan_cursors WHERE network=$1 AND token=$2`, spec.Network, spec.Token, usdt.MaxCursorBytes, usdt.MaxCursorHashBytes).Scan(&c.Value, &c.Hash, &oversized)
	if errors.Is(err, sql.ErrNoRows) {
		return c, nil
	}
	if err != nil {
		return usdt.Cursor{}, err
	}
	if oversized {
		return usdt.Cursor{}, usdt.ErrCapacity
	}
	if err := usdt.ValidateCursor(c); err != nil {
		return usdt.Cursor{}, err
	}
	return c, nil
}

// USDTReady recovers transfers committed immediately before process interruption.
func (s *Store) USDTReady(ctx context.Context, limit int) ([]usdt.Transfer, error) {
	return s.usdtTransferList(ctx, " WHERE status='ready' ORDER BY received_at,id LIMIT $1", limit)
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
	items, err := s.usdtTransferList(ctx, " WHERE status='review' AND id>$1 ORDER BY id LIMIT $2", cursor, limit+1)
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
