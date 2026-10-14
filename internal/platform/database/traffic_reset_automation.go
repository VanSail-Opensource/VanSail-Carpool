package database

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
)

// TrafficResetAutomation returns the member's account-wide preference.
func (s *Store) TrafficResetAutomation(ctx context.Context, userID string) (model.TrafficResetAutomation, error) {
	var enabled int
	var updated string
	err := s.db.QueryRowContext(ctx, `SELECT auto_traffic_reset_enabled,updated_at FROM users WHERE id=?`, userID).Scan(&enabled, &updated)
	if errors.Is(err, sql.ErrNoRows) {
		return model.TrafficResetAutomation{}, ErrNotFound
	}
	if err != nil {
		return model.TrafficResetAutomation{}, fmt.Errorf("load traffic reset automation: %w", err)
	}
	parsed, err := parseStamp(updated)
	if err != nil {
		return model.TrafficResetAutomation{}, err
	}
	return model.TrafficResetAutomation{Enabled: enabled == 1, UpdatedAt: parsed}, nil
}

// SetTrafficResetAutomation immediately saves the member's preference.
func (s *Store) SetTrafficResetAutomation(ctx context.Context, userID string, enabled bool, now time.Time) (model.TrafficResetAutomation, error) {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	result, err := s.db.ExecContext(ctx, `UPDATE users SET auto_traffic_reset_enabled=?,updated_at=? WHERE id=?`, boolInt(enabled), stamp(now), userID)
	if err != nil {
		return model.TrafficResetAutomation{}, fmt.Errorf("save traffic reset automation: %w", err)
	}
	if affected, rowsErr := result.RowsAffected(); rowsErr != nil || affected != 1 {
		if rowsErr != nil {
			return model.TrafficResetAutomation{}, rowsErr
		}
		return model.TrafficResetAutomation{}, ErrNotFound
	}
	return model.TrafficResetAutomation{Enabled: enabled, UpdatedAt: now.UTC()}, nil
}
