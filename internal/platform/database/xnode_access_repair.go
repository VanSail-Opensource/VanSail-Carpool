package database

import (
	"context"
	"encoding/json"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
)

// EnqueueXNodeAccessRepairs coalesces missing member credentials and paid-access
// recovery into the existing outbox. Quota exhaustion never triggers recovery.
func (s *Store) retryAttempt_EnqueueXNodeAccessRepairs(ctx context.Context, now time.Time) error {
	if !IsPostgres(s.db) {
		s.writeMu.Lock()
		defer s.writeMu.Unlock()
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	rows, err := tx.QueryContext(ctx, `SELECT DISTINCT u.id FROM users u
 LEFT JOIN purchases p ON p.user_id=u.id AND p.status IN ('active','activating')
 AND julianday(p.valid_from)<=julianday(?) AND julianday(p.valid_until)>julianday(?)
 LEFT JOIN xnode_identities i ON i.user_id=u.id
 LEFT JOIN xnode_periods a ON a.id=i.current_period_id
 WHERE u.onboarding_state='complete' AND (i.proxy_user_id IS NULL OR
 i.credentials_cipher='' OR i.subscription_cipher='' OR COALESCE(LENGTH(i.subscription_hash),0)<>32 OR
 (p.id IS NOT NULL AND (a.id IS NULL OR a.purchase_id<>p.id OR a.status<>'open')
 AND i.access_disabled=0
 AND NOT EXISTS(SELECT 1 FROM admin_temporary_bans b WHERE b.user_id=u.id AND b.restored_at IS NULL)))
 ORDER BY u.id LIMIT 128`, stamp(now), stamp(now))
	if err != nil {
		return err
	}
	var users []string
	for rows.Next() {
		var id string
		if err = rows.Scan(&id); err != nil {
			rows.Close()
			return err
		}
		users = append(users, id)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	for _, id := range users {
		raw, err := json.Marshal(map[string]string{"userId": id})
		if err != nil {
			return err
		}
		if err = insertOutboxTx(ctx, tx, xnode.AccessRepairJob, string(raw), now, now); err != nil {
			return err
		}
	}
	return tx.Commit()
}
