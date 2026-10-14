package database

import (
	"context"
	"database/sql"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/abuse"
)

func (s *Store) DetectorStateV2(ctx context.Context, userID, reason string) (abuse.DetectorState, error) {
	item := abuse.DetectorState{UserID: userID, ReasonName: reason}
	var raw string
	var emitted int
	err := s.db.QueryRowContext(ctx, `SELECT last_bucket_at,streak_seconds,incident_emitted FROM abuse_detector_state WHERE user_id=? AND reason_name=?`, userID, reason).Scan(&raw, &item.StreakSeconds, &emitted)
	if err == sql.ErrNoRows {
		return item, nil
	}
	if err != nil {
		return item, err
	}
	item.LastSecond, err = parseStamp(raw)
	item.IncidentEmitted = emitted == 1
	return item, err
}

func (s *Store) CommitEvaluation(ctx context.Context, claim abuse.EventClaim, result abuse.EvaluationResult, policy abuse.Policy, now time.Time) error {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	for _, state := range result.States {
		_, err = tx.ExecContext(ctx, `INSERT INTO abuse_detector_state(user_id,reason_name,last_bucket_at,streak_seconds,incident_emitted) VALUES(?,?,?,?,?) ON CONFLICT(user_id,reason_name) DO UPDATE SET last_bucket_at=excluded.last_bucket_at,streak_seconds=excluded.streak_seconds,incident_emitted=excluded.incident_emitted`, state.UserID, state.ReasonName, stamp(state.LastSecond), state.StreakSeconds, boolInt(state.IncidentEmitted))
		if err != nil {
			return err
		}
	}
	for _, incident := range result.Incidents {
		if _, err = createIncidentTx(ctx, tx, incident, policy, now); err != nil {
			return err
		}
	}
	for _, rollup := range result.Rollups {
		_, err = tx.ExecContext(ctx, `INSERT INTO abuse_qps_rollups(window_at,observation_count,qps_sum,qps_min,qps_max,created_at,updated_at) VALUES(?,?,?,?,?,?,?) ON CONFLICT(window_at) DO UPDATE SET observation_count=abuse_qps_rollups.observation_count+excluded.observation_count,qps_sum=abuse_qps_rollups.qps_sum+excluded.qps_sum,qps_min=MIN(abuse_qps_rollups.qps_min,excluded.qps_min),qps_max=MAX(abuse_qps_rollups.qps_max,excluded.qps_max),updated_at=excluded.updated_at`, stamp(rollup.WindowAt), rollup.ObservationCount, rollup.Sum, rollup.Min, rollup.Max, stamp(now), stamp(now))
		if err != nil {
			return err
		}
	}
	if claim.Token != "" {
		if _, err = tx.ExecContext(ctx, `DELETE FROM abuse_pending_log_events WHERE claim_token=?`, claim.Token); err != nil {
			return err
		}
	}
	if err = deleteLegacySamplesTx(ctx, tx, claim.Legacy); err != nil {
		return err
	}
	return tx.Commit()
}
