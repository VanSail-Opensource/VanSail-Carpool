package database

import (
	"context"
	"database/sql"
	"encoding/json"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
	"time"
)

func enqueuePAYGUserTx(ctx context.Context, tx *sql.Tx, user string, now time.Time) error {
	raw, err := json.Marshal(map[string]string{"userId": user})
	if err != nil {
		return err
	}
	var enabled int
	if err = tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM squad_product_overrides WHERE billing_mode='payg') OR EXISTS(SELECT 1 FROM xnode_payg_accounts WHERE user_id=?)`, user).Scan(&enabled); err != nil || enabled == 0 {
		return err
	}
	return insertOutboxTx(ctx, tx, xnode.PAYGJob, string(raw), now, now)
}

// Bounded repair only provisions missing tuples; settled financial state is
// never regenerated. Repeated scheduler passes progress past completed members.
func (s *Store) retryAttempt_EnqueueXNodePAYGRepairs(ctx context.Context, now time.Time) error {
	if !IsPostgres(s.db) {
		s.writeMu.Lock()
		defer s.writeMu.Unlock()
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	rows, err := tx.QueryContext(ctx, `SELECT DISTINCT u.id FROM users u JOIN squad_product_overrides o ON o.billing_mode='payg' JOIN xnode_groups g ON g.id=o.group_id AND g.enabled=1 JOIN xnode_group_bindings b ON b.group_id=g.id JOIN xnode_nodes n ON n.id=b.node_id AND n.enabled=1 WHERE u.onboarding_state='complete' AND NOT EXISTS(SELECT 1 FROM xnode_payg_accounts a WHERE a.user_id=u.id AND a.group_id=g.id AND a.node_id=b.node_id) ORDER BY u.id LIMIT 64`)
	if err != nil {
		return err
	}
	users := []string{}
	for rows.Next() {
		var user string
		if err = rows.Scan(&user); err != nil {
			rows.Close()
			return err
		}
		users = append(users, user)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	for _, user := range users {
		if err = enqueuePAYGUserTx(ctx, tx, user, now); err != nil {
			return err
		}
	}
	for batch := 0; batch < 16; batch++ {
		result, e := tx.ExecContext(ctx, `DELETE FROM xnode_payg_minutes WHERE (user_id,group_id,minute) IN (SELECT user_id,group_id,minute FROM xnode_payg_minutes WHERE minute<? ORDER BY minute LIMIT 4096)`, stamp(now.UTC().Truncate(time.Minute).Add(-72*time.Hour)))
		if e != nil {
			return e
		}
		count, e := result.RowsAffected()
		if e != nil {
			return e
		}
		if count < 4096 {
			break
		}
	}
	if _, err = tx.ExecContext(ctx, `DELETE FROM xnode_payg_windows WHERE (proxy_user_id,through) IN (SELECT proxy_user_id,through FROM xnode_payg_windows WHERE through<=? ORDER BY through LIMIT 65536)`, paygWindowStamp(now.Add(-xnode.PAYGWindow))); err != nil {
		return err
	}
	return tx.Commit()
}
