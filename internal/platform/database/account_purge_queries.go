package database

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
)

// AccountPurge reads only the current durable cleanup generation.
func (s *Store) AccountPurge(ctx context.Context, id string, generation int64) (*model.AccountPurge, error) {
	target := &model.AccountPurge{UserID: id, Generation: generation, ThroughGeneration: generation - 1}
	var cutoff string
	err := s.db.QueryRowContext(ctx, `SELECT last_purged_at FROM users WHERE id=$1 AND role='user' AND account_status='purging' AND account_generation=$2`, id, generation).Scan(&cutoff)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	valid, err := s.AccountCleanupAdmissionValid(ctx, id, generation, time.Now().UTC())
	if err != nil {
		return nil, err
	}
	if !valid {
		return target, ErrCleanupAdmissionRequired
	}
	if target.Cutoff, err = parseStamp(cutoff); err != nil {
		return nil, err
	}
	var attempted int
	var status string
	err = s.db.QueryRowContext(ctx, `SELECT COALESCE(remote_user_id,''),create_attempted,status FROM emby_accounts WHERE user_id=$1`, id).Scan(&target.RemoteEmbyID, &attempted, &status)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}
	target.EmbyUncertain = target.RemoteEmbyID == "" && attempted == 1 && status != "failed"
	err = s.db.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM outbox_jobs WHERE kind<>'account_purge' AND status='processing' AND `+accountJobTargets+`)`, id, id, id, id, id).Scan(&target.Busy)
	return target, err
}
