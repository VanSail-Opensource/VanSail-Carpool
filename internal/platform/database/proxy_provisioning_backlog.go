package database

import (
	"context"
	"fmt"
	"time"
)

// EnqueueUnlinkedPaidUsers repairs paid members left without a provider link.
func (s *Store) EnqueueUnlinkedPaidUsers(ctx context.Context, now time.Time) error {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin unlinked purchase scan: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	rows, err := tx.QueryContext(ctx, `SELECT users.id FROM users WHERE users.onboarding_state='complete'
		AND users.proxy_user_id IS NULL
		AND EXISTS(SELECT 1 FROM purchases WHERE purchases.user_id=users.id
			AND purchases.status IN ('active','activating','queued') AND purchases.valid_until>?)`, stamp(now))
	if err != nil {
		return fmt.Errorf("scan unlinked paid users: %w", err)
	}
	ids := make([]string, 0)
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			_ = rows.Close()
			return err
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return err
	}
	_ = rows.Close()
	for _, id := range ids {
		if err := insertOutboxTx(ctx, tx, "xnode_sync_user", `{"userId":"`+id+`"}`, now, now); err != nil {
			return err
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit unlinked purchase scan: %w", err)
	}
	return nil
}
