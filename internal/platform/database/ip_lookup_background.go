package database

import (
	"context"
	"database/sql"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/iplookup"
)

// PendingIPLookupOperations recovers durable runs after process interruption.
func (s *Store) PendingIPLookupOperations(ctx context.Context) ([]string, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT (SELECT c.operation_id FROM ip_lookup_checks c JOIN provider_operations o ON o.id=c.operation_id WHERE c.report_id=r.id ORDER BY o.created_at,o.record_order LIMIT 1) FROM ip_lookup_reports r WHERE r.status='processing' AND EXISTS(SELECT 1 FROM ip_lookup_checks c WHERE c.report_id=r.id) ORDER BY r.created_at,r.record_order LIMIT 1`)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	ids := []string{}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

// HeartbeatIPLookup preserves a live worker lease without rewriting report data.
func (s *Store) HeartbeatIPLookup(ctx context.Context, id, owner string, now time.Time) error {
	if owner == "" {
		return ErrConflict
	}
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	result, err := s.db.ExecContext(ctx, `UPDATE ip_lookup_reports SET heartbeat_at=$1,lease_expires_at=$2 WHERE status='processing' AND lease_owner=$3 AND vansail_timestamp(lease_expires_at)>vansail_timestamp($1) AND id=(SELECT report_id FROM ip_lookup_checks WHERE operation_id=$4)`, stamp(now), stamp(now.Add(time.Minute)), owner, id)
	if err != nil {
		return err
	}
	count, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if count != 1 {
		var status string
		if err := s.db.QueryRowContext(ctx, `SELECT r.status FROM ip_lookup_checks c LEFT JOIN ip_lookup_reports r ON r.id=c.report_id WHERE c.operation_id=$1`, id).Scan(&status); err == sql.ErrNoRows {
			return nil
		} else if err == nil && status != "processing" {
			return nil
		}
		return ErrConflict
	}
	return nil
}

// ClaimIPLookupLease admits only the oldest run, after every active run or repair.
func (s *Store) ClaimIPLookupLease(ctx context.Context, id, owner string, now time.Time) (bool, error) {
	if owner == "" {
		return false, ErrConflict
	}
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	result, err := s.db.ExecContext(ctx, `UPDATE ip_lookup_reports SET lease_owner=$1,lease_expires_at=$2,heartbeat_at=$3
		WHERE status='processing' AND id=(SELECT report_id FROM ip_lookup_checks WHERE operation_id=$4)
		AND id=(SELECT r.id FROM ip_lookup_reports r WHERE r.status='processing' AND EXISTS(SELECT 1 FROM ip_lookup_checks c WHERE c.report_id=r.id) ORDER BY r.created_at,r.record_order LIMIT 1)
		AND (lease_owner=$5 OR lease_expires_at IS NULL OR vansail_timestamp(lease_expires_at)<=vansail_timestamp($6))
		AND NOT EXISTS(SELECT 1 FROM ip_lookup_reports other WHERE other.status='processing' AND other.id<>ip_lookup_reports.id AND vansail_timestamp(other.lease_expires_at)>vansail_timestamp($7))
		AND NOT EXISTS(SELECT 1 FROM outbox_jobs WHERE kind IN ('ip_lookup_report_retry','ip_lookup_live_retry') AND status='processing')`, owner, stamp(now.Add(time.Minute)), stamp(now), id, owner, stamp(now), stamp(now))
	if err != nil {
		return false, err
	}
	count, err := result.RowsAffected()
	return count == 1, err
}

// IPLookupSharedProgress projects publicly shareable evidence for an exact IP.
func (s *Store) IPLookupSharedProgress(ctx context.Context, ip string) (*iplookup.SharedProgress, error) {
	var raw, updated string
	err := s.db.QueryRowContext(ctx, `SELECT report_json,COALESCE(heartbeat_at,created_at) FROM ip_lookup_reports WHERE ip=$1 AND status='processing' ORDER BY record_order DESC LIMIT 1`, ip).Scan(&raw, &updated)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	r, err := iplookup.DecodeReport([]byte(raw))
	if err != nil {
		return nil, err
	}
	return &iplookup.SharedProgress{IP: ip, Status: r.Status, Progress: iplookup.CheckProgress(r), UpdatedAt: updated}, nil
}
