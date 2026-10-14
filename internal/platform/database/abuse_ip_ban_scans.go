package database

import (
	"context"
	"database/sql"
	"strings"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/abuse"
)

func (s *Store) AbuseIPBanScan(ctx context.Context, recordID string) (string, error) {
	var scanID string
	err := s.db.QueryRowContext(ctx, `SELECT scan_job_id FROM abuse_ip_ban_scans WHERE record_id=?`, recordID).Scan(&scanID)
	if err == sql.ErrNoRows {
		return "", ErrNotFound
	}
	return scanID, err
}

func (s *Store) SaveAbuseIPBanScan(ctx context.Context, recordID, scanID string, now time.Time) error {
	if strings.TrimSpace(recordID) == "" || strings.TrimSpace(scanID) == "" {
		return abuse.ErrInvalid
	}
	_, err := s.db.ExecContext(ctx, `INSERT OR IGNORE INTO abuse_ip_ban_scans(record_id,scan_job_id,created_at) VALUES(?,?,?)`, recordID, scanID, stamp(now))
	return err
}
