package database

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/iplookup"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/ids"
)

// CachedIPDetails reads exact evidence without side effects.
func (s *Store) retryAttempt_CachedIPDetails(ctx context.Context, ip string) (*iplookup.LiveDetails, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()
	combined, match, err := ipCachedReportTx(ctx, tx, ip)
	if err != nil {
		return nil, err
	}
	if combined != nil && combined.LiveDetails != nil {
		details := *combined.LiveDetails
		details.CacheMatch = match
		details.SourceIP = combined.IP
		details.IP = ip
		return &details, nil
	}
	id, err := newestIPCacheTableTx(ctx, tx, "ip_lookup_live_snapshots", "ip=$1", ip)
	match = "exact"
	if err != nil {
		return nil, err
	}
	if err != nil || id == "" {
		return nil, err
	}
	var raw string
	if err := tx.QueryRowContext(ctx, `SELECT details_json FROM ip_lookup_live_snapshots WHERE id=$1`, id).Scan(&raw); err != nil {
		return nil, err
	}
	r, err := decodeIPDetails(raw, id, ip, match)
	return &r, err
}

func decodeIPDetails(raw, id, requested, match string) (iplookup.LiveDetails, error) {
	var r iplookup.LiveDetails
	if err := json.Unmarshal([]byte(raw), &r); err != nil {
		return r, err
	}
	r = iplookup.CompactLiveDetails(r)
	r.SnapshotID, r.IP, r.CacheMatch = id, requested, match
	return r, nil
}

// SaveIPDetails stores one normalized snapshot; a durable run key is replay safe.
func (s *Store) retryAttempt_SaveIPDetails(ctx context.Context, run string, r iplookup.LiveDetails, now time.Time) (string, error) {
	r = iplookup.CompactLiveDetails(r)
	if r.IP != r.SourceIP {
		return "", ErrConflict
	}
	raw, err := json.Marshal(r)
	if err != nil {
		return "", err
	}
	if len(raw) > 4*1024*1024 {
		return "", &iplookup.CodeError{Code: "IP_DETAILS_INVALID_RESPONSE"}
	}
	id, err := ids.New()
	if err != nil {
		return "", err
	}
	var runKey any
	if run != "" {
		runKey = run
	}
	status := "succeeded"
	if iplookup.LivePartial(r) {
		status = "partial"
	}
	var retryDelay time.Duration
	retryEnabled := false
	if status == "partial" {
		retryDelay, retryEnabled = s.ipLiveRetryDelay(ctx)
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return "", err
	}
	defer func() { _ = tx.Rollback() }()
	if run != "" {
		status, _, err := lockIPLookupRunTx(ctx, tx, run)
		if err != nil {
			return "", err
		}
		if status != "processing" {
			// Retained evidence is replayable; a completed run cannot acquire a new
			// snapshot from an expired provider execution.
			id, _, err := liveSnapshotForRunTx(ctx, tx, run)
			if err == nil && id == "" {
				err = ErrConflict
			}
			return id, err
		}
	}
	if run == "" {
		var fetched string
		err := tx.QueryRowContext(ctx, `SELECT (CASE ((details_json)::jsonb #> '{fetchedAt}')::text WHEN 'true' THEN '1' WHEN 'false' THEN '0' ELSE (details_json)::jsonb #>> '{fetchedAt}' END) FROM ip_lookup_live_snapshots WHERE ip=$1 ORDER BY `+purchaseTimestampOrderSQL("completed_at")+` DESC,record_order DESC LIMIT 1`, r.IP).Scan(&fetched)
		if err != nil && err != sql.ErrNoRows {
			return "", err
		}
		previous, previousErr := time.Parse(time.RFC3339Nano, fetched)
		started, startedErr := time.Parse(time.RFC3339Nano, r.FetchedAt)
		// A slow free read must not replace evidence from a later paid refresh.
		if previousErr == nil && startedErr == nil && previous.After(started) {
			return "", nil
		}
	}
	result, err := tx.ExecContext(ctx, `INSERT INTO ip_lookup_live_snapshots(id,ip,status,details_json,completed_at,run_key) VALUES($1,$2,$3,$4,$5,$6) ON CONFLICT(run_key) DO NOTHING`, id, r.IP, status, string(raw), stamp(now), runKey)
	if err != nil {
		return "", err
	}
	inserted, err := result.RowsAffected()
	if err != nil {
		return "", err
	}
	if inserted == 1 && retryEnabled {
		if err := scheduleIPLiveRetryTx(ctx, tx, id, retryDelay, now); err != nil {
			return "", err
		}
	}
	if run != "" {
		if err := tx.QueryRowContext(ctx, `SELECT id FROM ip_lookup_live_snapshots WHERE run_key=$1`, run).Scan(&id); err != nil {
			return "", err
		}
	}
	if err := tx.Commit(); err != nil {
		return "", err
	}
	return id, nil
}

func liveSnapshotForRunTx(ctx context.Context, tx *sql.Tx, run string) (string, string, error) {
	var id, status string
	err := tx.QueryRowContext(ctx, `SELECT id,status FROM ip_lookup_live_snapshots WHERE run_key=$1`, run).Scan(&id, &status)
	if err == sql.ErrNoRows {
		return "", "", nil
	}
	return id, status, err
}
