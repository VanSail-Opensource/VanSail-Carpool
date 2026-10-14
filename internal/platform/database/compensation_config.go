package database

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/compensation"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/ids"
)

// CompensationConfig returns the atomic singleton configuration.
func (s *Store) CompensationConfig(ctx context.Context) (compensation.Config, error) {
	return scanCompensationConfig(s.db.QueryRowContext(ctx, `SELECT enabled,threshold_minutes,multiplier_bps,revision,updated_at
		FROM node_compensation_config WHERE id=1`))
}

func scanCompensationConfig(row rowScanner) (compensation.Config, error) {
	var result compensation.Config
	var enabled int
	var threshold, multiplier sql.NullInt64
	var updated string
	if err := row.Scan(&enabled, &threshold, &multiplier, &result.Revision, &updated); err != nil {
		return result, err
	}
	result.Enabled = enabled == 1
	if threshold.Valid {
		value := int(threshold.Int64)
		result.ThresholdMinutes = &value
	}
	if multiplier.Valid {
		value := int(multiplier.Int64)
		result.MultiplierBPS = &value
	}
	parsed, err := parseStamp(updated)
	result.UpdatedAt = parsed
	return result, err
}

// UpdateCompensationConfig applies revision-based optimistic concurrency and audits the actor.
func (s *Store) UpdateCompensationConfig(ctx context.Context, actorID string, input compensation.ConfigUpdate, now time.Time) (compensation.Config, error) {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return compensation.Config{}, err
	}
	defer func() { _ = tx.Rollback() }()
	result, err := tx.ExecContext(ctx, `UPDATE node_compensation_config SET enabled=?,threshold_minutes=?,multiplier_bps=?,
		revision=revision+1,updated_at=? WHERE id=1 AND revision=?`, boolInt(input.Enabled), nullableInt(input.ThresholdMinutes),
		nullableInt(input.MultiplierBPS), stamp(now), input.Revision)
	if err != nil {
		return compensation.Config{}, fmt.Errorf("update compensation config: %w", err)
	}
	changed, _ := result.RowsAffected()
	if changed != 1 {
		return compensation.Config{}, compensation.ErrConflict
	}
	detail, _ := json.Marshal(map[string]any{"enabled": input.Enabled, "thresholdMinutes": input.ThresholdMinutes,
		"multiplierBps": input.MultiplierBPS, "revision": input.Revision + 1})
	auditID, err := ids.New()
	if err != nil {
		return compensation.Config{}, err
	}
	if err := insertAuditTx(ctx, tx, auditID, &actorID, "node_compensation.config_update", "node_compensation_config", "1", string(detail), now); err != nil {
		return compensation.Config{}, err
	}
	config, err := scanCompensationConfig(tx.QueryRowContext(ctx, `SELECT enabled,threshold_minutes,multiplier_bps,revision,updated_at
		FROM node_compensation_config WHERE id=1`))
	if err != nil {
		return compensation.Config{}, err
	}
	if err := tx.Commit(); err != nil {
		return compensation.Config{}, err
	}
	return config, nil
}

func nullableInt(value *int) any {
	if value == nil {
		return nil
	}
	return *value
}
