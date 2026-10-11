package database

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/netip"
	"strings"
	"time"

	"github.com/txyyddss/Remna-User-Panel/internal/iplookup"
	"github.com/txyyddss/Remna-User-Panel/internal/platform/ids"
)

// CachedIPDetails reads exact evidence first, then IPv4 /24, without side effects.
func (s *Store) CachedIPDetails(ctx context.Context, ip string) (*iplookup.LiveDetails, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()
	combined, match, err := ipCachedReportTx(ctx, tx, ip, true)
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
	id, err := newestIPCacheTableTx(ctx, tx, "ip_lookup_live_snapshots", "ip=?", ip)
	match = "exact"
	if err != nil {
		return nil, err
	}
	if address, parseErr := netip.ParseAddr(ip); id == "" && parseErr == nil && address.Is4() {
		prefix := ip[:strings.LastIndexByte(ip, '.')+1]
		id, err = newestIPCacheTableTx(ctx, tx, "ip_lookup_live_snapshots", "ip>=? AND ip<?", prefix, strings.TrimSuffix(prefix, ".")+"/")
		match = "subnet"
	}
	if err != nil || id == "" {
		return nil, err
	}
	var raw string
	if err := tx.QueryRowContext(ctx, `SELECT details_json FROM ip_lookup_live_snapshots WHERE id=?`, id).Scan(&raw); err != nil {
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
func (s *Store) SaveIPDetails(ctx context.Context, run string, r iplookup.LiveDetails, now time.Time) (string, error) {
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
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return "", err
	}
	defer func() { _ = tx.Rollback() }()
	if run == "" {
		var fetched string
		err := tx.QueryRowContext(ctx, `SELECT json_extract(details_json,'$.fetchedAt') FROM ip_lookup_live_snapshots WHERE ip=? ORDER BY julianday(completed_at) DESC,rowid DESC LIMIT 1`, r.IP).Scan(&fetched)
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
	result, err := tx.ExecContext(ctx, `INSERT INTO ip_lookup_live_snapshots(id,ip,status,details_json,completed_at,run_key) VALUES(?,?,?,?,?,?) ON CONFLICT(run_key) DO NOTHING`, id, r.IP, status, string(raw), stamp(now), runKey)
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
		if err := tx.QueryRowContext(ctx, `SELECT id FROM ip_lookup_live_snapshots WHERE run_key=?`, run).Scan(&id); err != nil {
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
	err := tx.QueryRowContext(ctx, `SELECT id,status FROM ip_lookup_live_snapshots WHERE run_key=?`, run).Scan(&id, &status)
	if err == sql.ErrNoRows {
		return "", "", nil
	}
	return id, status, err
}
