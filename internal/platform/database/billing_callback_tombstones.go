package database

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// PaymentCallbackReplay reports whether a verified provider callback was
// already applied, including after its terminal payment detail was compacted.
func (s *Store) PaymentCallbackReplay(ctx context.Context, provider, dedupeKey, orderID string) (bool, error) {
	var existingOrderID string
	err := s.db.QueryRowContext(ctx, `SELECT order_id FROM payment_callback_tombstones
		WHERE provider=? AND dedupe_key=?`, provider, dedupeKey).Scan(&existingOrderID)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("load payment callback tombstone: %w", err)
	}
	if existingOrderID != orderID {
		return false, ErrConflict
	}
	return true, nil
}

// compactPaymentCallbackTx replaces a transient claim with its replay evidence.
func compactPaymentCallbackTx(ctx context.Context, tx *sql.Tx, provider, dedupeKey, orderID, payloadHash, eventID string, now time.Time) error {
	if _, err := tx.ExecContext(ctx, `INSERT INTO payment_callback_tombstones(provider,dedupe_key,order_id,
		payload_hash,received_at,processed_at) VALUES(?,?,?,?,?,?)`, provider, dedupeKey, orderID,
		payloadHash, stamp(now), stamp(now)); err != nil {
		return fmt.Errorf("preserve payment callback tombstone: %w", err)
	}
	_, err := tx.ExecContext(ctx, `DELETE FROM webhook_events WHERE id=?`, eventID)
	return err
}
