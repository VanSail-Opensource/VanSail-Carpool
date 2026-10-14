package database

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/ids"
)

const (
	// DefaultAddTXBMinimumMinor is 1.00 TXB.
	DefaultAddTXBMinimumMinor int64 = 100
	// DefaultAddTXBMaximumMinor is 100,000,000.00 TXB.
	DefaultAddTXBMaximumMinor int64 = 10_000_000_000
)

// AddTXBBounds returns the inclusive server-authoritative top-up range.
func (s *Store) AddTXBBounds(ctx context.Context) (model.AddTXBBounds, error) {
	return scanAddTXBBounds(s.db.QueryRowContext(ctx, `SELECT minimum_txb_minor,maximum_txb_minor,updated_at
		FROM txb_limits WHERE singleton=1`))
}

// UpdateAddTXBBounds atomically changes the singleton range and audits the actor.
func (s *Store) UpdateAddTXBBounds(ctx context.Context, minimumMinor, maximumMinor int64, actorID string, now time.Time) (model.AddTXBBounds, error) {
	if minimumMinor <= 0 || maximumMinor < minimumMinor {
		return model.AddTXBBounds{}, ErrConflict
	}
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return model.AddTXBBounds{}, fmt.Errorf("begin Add TXB bounds update: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	result, err := tx.ExecContext(ctx, `UPDATE txb_limits SET minimum_txb_minor=?,maximum_txb_minor=?,
		updated_by=?,updated_at=? WHERE singleton=1`, minimumMinor, maximumMinor, nullIfEmpty(actorID), stamp(now))
	if err != nil {
		return model.AddTXBBounds{}, fmt.Errorf("update Add TXB bounds: %w", err)
	}
	if affected, rowsErr := result.RowsAffected(); rowsErr != nil || affected != 1 {
		return model.AddTXBBounds{}, ErrConflict
	}
	auditID, err := ids.New()
	if err != nil {
		return model.AddTXBBounds{}, err
	}
	detail, err := json.Marshal(map[string]int64{"minimumTxbMinor": minimumMinor, "maximumTxbMinor": maximumMinor})
	if err != nil {
		return model.AddTXBBounds{}, fmt.Errorf("encode Add TXB bounds audit: %w", err)
	}
	actor := actorID
	var actorPointer *string
	if actor != "" {
		actorPointer = &actor
	}
	if err := insertAuditTx(ctx, tx, auditID, actorPointer, "billing.amount_bounds.update", "txb_limits", "1", string(detail), now); err != nil {
		return model.AddTXBBounds{}, fmt.Errorf("audit Add TXB bounds update: %w", err)
	}
	bounds, err := scanAddTXBBounds(tx.QueryRowContext(ctx, `SELECT minimum_txb_minor,maximum_txb_minor,updated_at
		FROM txb_limits WHERE singleton=1`))
	if err != nil {
		return model.AddTXBBounds{}, err
	}
	if err := tx.Commit(); err != nil {
		return model.AddTXBBounds{}, fmt.Errorf("commit Add TXB bounds update: %w", err)
	}
	return bounds, nil
}

func scanAddTXBBounds(row rowScanner) (model.AddTXBBounds, error) {
	var bounds model.AddTXBBounds
	var updated string
	if err := row.Scan(&bounds.MinimumTXBMinor, &bounds.MaximumTXBMinor, &updated); err != nil {
		return model.AddTXBBounds{}, fmt.Errorf("scan Add TXB bounds: %w", err)
	}
	bounds.Minimum = model.TXBMoney(bounds.MinimumTXBMinor)
	bounds.Maximum = model.TXBMoney(bounds.MaximumTXBMinor)
	parsed, err := parseStamp(updated)
	if err != nil {
		return model.AddTXBBounds{}, fmt.Errorf("parse Add TXB bounds update: %w", err)
	}
	bounds.UpdatedAt = parsed
	return bounds, nil
}
