package database

import (
	"context"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
)

// RetiredDetectorReleaseUser reads only acknowledged historical restrictions.
// Detector configuration, samples and credentials have no runtime read surface.
func (s *Store) RetiredDetectorReleaseUser(ctx context.Context, id string) (model.User, bool, error) {
	var pending bool
	err := s.db.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM abuse_temp_bans ban JOIN abuse_records record ON record.id=ban.record_id WHERE ban.user_id=? AND ban.restored_at IS NULL AND record.punishment_completed_at IS NOT NULL)`, id).Scan(&pending)
	if err != nil || !pending {
		return model.User{}, pending, err
	}
	user, err := s.UserByID(ctx, id)
	return user, true, err
}

func (s *Store) retryAttempt_CompleteRetiredDetectorRelease(ctx context.Context, id string, now time.Time) error {
	if !IsPostgres(s.db) {
		s.writeMu.Lock()
		defer s.writeMu.Unlock()
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	result, err := tx.ExecContext(ctx, `UPDATE abuse_temp_bans SET restored_at=? WHERE user_id=? AND restored_at IS NULL AND EXISTS(SELECT 1 FROM abuse_records record WHERE record.id=abuse_temp_bans.record_id AND record.punishment_completed_at IS NOT NULL)`, stamp(now), id)
	if err != nil {
		return err
	}
	count, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if count > 0 {
		_, err = tx.ExecContext(ctx, `INSERT INTO audit_events(id,actor_user_id,action,target_type,target_id,detail,created_at) VALUES(?,NULL,'migration.detector_restriction_released','user',?,'{}',?)`, "detector-retirement-release:"+id, id, stamp(now))
		if err != nil {
			return err
		}
	}
	return tx.Commit()
}
