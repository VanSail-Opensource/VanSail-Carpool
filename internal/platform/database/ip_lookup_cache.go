package database

import (
	"context"
	"database/sql"
	"net/netip"
	"strings"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/iplookup"
)

// ipCachedReportTx prefers evidence for the address itself, then a completed IPv4 /24.
func ipCachedReportTx(ctx context.Context, tx *sql.Tx, ip string, subnet bool) (*iplookup.Report, string, error) {
	id, err := newestIPCacheTx(ctx, tx, "ip=$1", ip)
	match := "exact"
	if err != nil {
		return nil, "none", err
	}
	if id == "" && subnet {
		address, parseErr := netip.ParseAddr(ip)
		if parseErr != nil || !address.Is4() {
			return nil, "none", nil
		}
		prefix := ip[:strings.LastIndexByte(ip, '.')+1]
		// Canonical IPv4 text in [prefix, prefix-with-slash) has this exact /24.
		upper := strings.TrimSuffix(prefix, ".") + "/"
		id, err = newestIPCacheTx(ctx, tx, "ip>=$1 AND ip<$2", prefix, upper)
		match = "subnet"
	}
	if err != nil {
		return nil, "none", err
	}
	if id == "" {
		return nil, "none", nil
	}
	var raw string
	if err := tx.QueryRowContext(ctx, `SELECT report_json FROM ip_lookup_reports WHERE id=$1`, id).Scan(&raw); err != nil {
		return nil, "none", err
	}
	report, err := iplookup.DecodeReport([]byte(raw))
	if err != nil {
		return nil, "none", err
	}
	public := iplookup.PublicReport(report)
	if public.LiveDetails != nil {
		public.LiveDetails.CacheMatch = match
		public.LiveDetails.SourceIP = report.IP
	}
	return &public, match, nil
}

// newestIPCacheTx compares original nanoseconds within a PostgreSQL timestamp bucket.
// Only metadata is streamed; record_order breaks equal-time ties.
func newestIPCacheTx(ctx context.Context, tx *sql.Tx, predicate string, args ...any) (string, error) {
	return newestIPCacheTableTx(ctx, tx, "ip_lookup_reports", predicate, args...)
}

// Callers select a fixed internal table; predicates and tables never come from HTTP.
func newestIPCacheTableTx(ctx context.Context, tx *sql.Tx, table, predicate string, args ...any) (string, error) {
	rows, err := tx.QueryContext(ctx, `SELECT id,completed_at,record_order,vansail_timestamp(completed_at) FROM `+table+` WHERE `+predicate+` AND status IN ('succeeded','partial') ORDER BY vansail_timestamp(completed_at) DESC,record_order DESC`, args...)
	if err != nil {
		return "", err
	}
	defer func() { _ = rows.Close() }()
	var bestID string
	var bestTime time.Time
	var bestRowID int64
	var bestBucket sql.NullTime
	for rows.Next() {
		var id string
		var timestamp sql.NullString
		var rowID int64
		var bucket sql.NullTime
		if err := rows.Scan(&id, &timestamp, &rowID, &bucket); err != nil {
			return "", err
		}
		if !bestTime.IsZero() && bucket != bestBucket {
			break
		}
		completed, parseErr := time.Parse(time.RFC3339Nano, timestamp.String)
		if parseErr != nil {
			completed = time.Time{}
		}
		if bestID == "" || completed.After(bestTime) || (completed.Equal(bestTime) && rowID > bestRowID) {
			bestID, bestTime, bestRowID, bestBucket = id, completed, rowID, bucket
		}
	}
	return bestID, rows.Err()
}

// IPLookupQuoteResponse previews a shared cache without creating an operation or charging.
func (s *Store) retryAttempt_IPLookupQuoteResponse(ctx context.Context, user, ip string, refresh bool, now time.Time) (iplookup.QuoteResponse, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return iplookup.QuoteResponse{}, err
	}
	defer func() { _ = tx.Rollback() }()
	quote, _, err := ipQuoteTx(ctx, tx, user, ip, refresh, now)
	if err != nil {
		return iplookup.QuoteResponse{}, err
	}
	report, match, err := ipCachedReportTx(ctx, tx, ip, true)
	return iplookup.QuoteResponse{Quote: quote, CachedReport: report, CacheMatch: match}, err
}
