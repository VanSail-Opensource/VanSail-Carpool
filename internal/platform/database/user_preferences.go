package database

import (
	"context"
	"database/sql"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/preferences"
	"time"
)

const preferenceSelect = `SELECT COALESCE(p.notify_combos,1),COALESCE(p.notify_traffic,1),
 COALESCE(p.notify_money,1),COALESCE(p.notify_activity,1),COALESCE(p.notify_account,1),
 COALESCE(p.show_referral_username,1),COALESCE(p.show_around_tx,0),COALESCE(p.show_activity,0),
 COALESCE(p.include_node_prices,1) FROM users u LEFT JOIN user_preferences p ON p.user_id=u.id WHERE u.id=?`

func scanPreferences(row rowScanner) (preferences.Preferences, error) {
	var p preferences.Preferences
	err := row.Scan(&p.Notifications.Combos, &p.Notifications.Traffic, &p.Notifications.Money,
		&p.Notifications.Activity, &p.Notifications.Account, &p.ShowReferralUsername, &p.ShowAroundTX,
		&p.ShowActivity, &p.IncludeNodePrices)
	return p, err
}

// UserPreferences retrieves current choices without losing immutable event facts.
func (s *Store) UserPreferences(ctx context.Context, userID string) (preferences.Preferences, error) {
	return scanPreferences(s.db.QueryRowContext(ctx, preferenceSelect, userID))
}

// UserPreferenceSnapshot projects the strict active-combo gate.
func (s *Store) UserPreferenceSnapshot(ctx context.Context, userID string, now time.Time) (preferences.Snapshot, error) {
	p, err := s.UserPreferences(ctx, userID)
	if err != nil {
		return preferences.Snapshot{}, err
	}
	active, err := s.HasActiveCombo(ctx, userID, now)
	if !active {
		p.ShowAroundTX, p.ShowActivity = false, false
	}
	return preferences.Snapshot{Preferences: p, ActiveCombo: active}, err
}

// UpdateUserPreferences merges supplied flags atomically with live eligibility.
func (s *Store) retryAttempt_UpdateUserPreferences(ctx context.Context, userID string, patch preferences.Patch, now time.Time) (preferences.Snapshot, error) {
	if !IsPostgres(s.db) {
		s.writeMu.Lock()
		defer s.writeMu.Unlock()
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return preferences.Snapshot{}, err
	}
	defer func() { _ = tx.Rollback() }()
	p, err := scanPreferences(tx.QueryRowContext(ctx, preferenceSelect, userID))
	if err != nil {
		return preferences.Snapshot{}, err
	}
	var active bool
	err = tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM purchases WHERE user_id=? AND status='active'
 AND valid_from<=? AND valid_until>?)`, userID, stamp(now), stamp(now)).Scan(&active)
	if err != nil {
		return preferences.Snapshot{}, err
	}
	p.Apply(patch)
	if !active {
		p.ShowAroundTX, p.ShowActivity = false, false
	}
	err = savePreferences(ctx, tx, userID, p)
	if err != nil {
		return preferences.Snapshot{}, err
	}
	if err := tx.Commit(); err != nil {
		return preferences.Snapshot{}, err
	}
	return preferences.Snapshot{Preferences: p, ActiveCombo: active}, nil
}

func savePreferences(ctx context.Context, tx *sql.Tx, userID string, p preferences.Preferences) error {
	_, err := tx.ExecContext(ctx, `INSERT INTO user_preferences VALUES(?,?,?,?,?,?,?,?,?,?)
 ON CONFLICT(user_id) DO UPDATE SET notify_combos=excluded.notify_combos,notify_traffic=excluded.notify_traffic,
 notify_money=excluded.notify_money,notify_activity=excluded.notify_activity,notify_account=excluded.notify_account,
 show_referral_username=excluded.show_referral_username,show_around_tx=excluded.show_around_tx,
 show_activity=excluded.show_activity,include_node_prices=excluded.include_node_prices`, userID,
		p.Notifications.Combos, p.Notifications.Traffic, p.Notifications.Money, p.Notifications.Activity, p.Notifications.Account,
		p.ShowReferralUsername, p.ShowAroundTX, p.ShowActivity, p.IncludeNodePrices)
	return err
}

// TelegramNotificationAllowed applies current choices to legacy affiliate jobs.
func (s *Store) TelegramNotificationAllowed(ctx context.Context, telegramID int64, kind string) (bool, error) {
	user, err := s.UserByTelegramID(ctx, telegramID)
	if err != nil {
		return false, err
	}
	p, err := s.UserPreferences(ctx, user.ID)
	if err != nil {
		return false, err
	}
	allowed, err := p.Allows(kind)
	if err == nil && !allowed && s.logger != nil {
		s.logger.Info("private notification suppressed", "kind", kind, "user_id", user.ID)
	}
	return allowed, err
}
