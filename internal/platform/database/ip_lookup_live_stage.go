package database

import (
	"context"
	"database/sql"
	"encoding/json"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/iplookup"
)

func ipLiveStageTx(ctx context.Context, tx *sql.Tx, id string) (iplookup.LiveStage, error) {
	var raw string
	var stage iplookup.LiveStage
	err := tx.QueryRowContext(ctx, `SELECT live_state_json FROM ip_lookup_reports WHERE id=$1`, id).Scan(&raw)
	if err == nil {
		err = json.Unmarshal([]byte(raw), &stage)
	}
	return stage, err
}

// ClaimIPLookupLiveStage recovers a persisted snapshot before considering replay.
func (s *Store) retryAttempt_ClaimIPLookupLiveStage(ctx context.Context, id string) (iplookup.LiveStage, bool, error) {

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return iplookup.LiveStage{}, false, err
	}
	defer func() { _ = tx.Rollback() }()
	if err := lockIPLookupProcessingRunTx(ctx, tx, id); err != nil {
		return iplookup.LiveStage{}, false, err
	}
	stage, err := ipLiveStageTx(ctx, tx, id)
	if err != nil {
		return stage, false, err
	}
	claimed := stage.Status == "queued"
	if stage.Status == "processing" {
		snapshot, status, err := liveSnapshotForRunTx(ctx, tx, id)
		if err != nil {
			return stage, false, err
		}
		stage.SnapshotID, stage.Status = snapshot, status
		if snapshot == "" {
			stage.Status = "error"
		}
	} else if claimed {
		stage.Status = "processing"
	}
	if err := saveLiveStageTx(ctx, tx, id, stage); err != nil {
		return stage, false, err
	}
	return stage, claimed, tx.Commit()
}

func saveLiveStageTx(ctx context.Context, tx *sql.Tx, id string, stage iplookup.LiveStage) error {
	raw, err := json.Marshal(stage)
	if err != nil {
		return err
	}
	result, err := tx.ExecContext(ctx, `UPDATE ip_lookup_reports SET live_state_json=$1 WHERE id=$2 AND status='processing'`, string(raw), id)
	if err != nil {
		return err
	}
	n, err := result.RowsAffected()
	if err == nil && n != 1 {
		return ErrConflict
	}
	return err
}

// MarkIPLookupLiveAttempt commits before the first actual queued external request.
func (s *Store) MarkIPLookupLiveAttempt(ctx context.Context, id string) error {
	result, err := s.db.ExecContext(ctx, `UPDATE ip_lookup_reports SET live_state_json=jsonb_set((live_state_json)::jsonb,'{attempted}',('true')::jsonb,true)::text WHERE id=$1 AND status='processing' AND (CASE ((live_state_json)::jsonb #> '{status}')::text WHEN 'true' THEN '1' WHEN 'false' THEN '0' ELSE (live_state_json)::jsonb #>> '{status}' END)='processing'
		AND ((COALESCE(lease_owner,'')='' AND $2='') OR (lease_owner=$2 AND `+purchaseTimestampOrderSQL("lease_expires_at")+`>`+databaseClockTimestampOrderSQL+`))`, id, iplookup.RunLeaseOwner(ctx))
	if err != nil {
		return err
	}
	n, err := result.RowsAffected()
	if err == nil && n != 1 {
		return ErrConflict
	}
	return err
}

// FinishIPLookupLiveStage keeps the durable attempted marker independent of readers.
func (s *Store) retryAttempt_FinishIPLookupLiveStage(ctx context.Context, id string, stage iplookup.LiveStage) error {

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	if err := lockIPLookupProcessingRunTx(ctx, tx, id); err != nil {
		return err
	}
	previous, err := ipLiveStageTx(ctx, tx, id)
	if err != nil {
		return err
	}
	stage.Attempted = previous.Attempted
	if err := saveLiveStageTx(ctx, tx, id, stage); err != nil {
		return err
	}
	return tx.Commit()
}
