package database

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/iplookup"
)

func ipRepairRevision(raw string) string {
	digest := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(digest[:])
}

// UpdateIPLookupCachedReport repairs only the exact partial document read by
// this worker. Completion ordering and original identity remain unchanged.
func (s *Store) retryAttempt_UpdateIPLookupCachedReport(ctx context.Context, report iplookup.Report) (string, error) {
	if report.RepairRevision == "" || report.Status != "partial" && report.Status != "succeeded" {
		return "", iplookup.ErrRepairConflict
	}
	raw, err := iplookup.EncodeReport(report)
	if err != nil {
		return "", err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return "", err
	}
	defer tx.Rollback()
	var ip, originalRaw, status string
	err = tx.QueryRowContext(ctx, `SELECT ip,report_json,status FROM ip_lookup_reports WHERE id=$1 FOR UPDATE`,
		report.ID).Scan(&ip, &originalRaw, &status)
	if errors.Is(err, sql.ErrNoRows) {
		return "", iplookup.ErrRepairConflict
	}
	if err != nil {
		return "", err
	}
	if status != "partial" || ipRepairRevision(originalRaw) != report.RepairRevision {
		return "", iplookup.ErrRepairConflict
	}
	original, err := iplookup.DecodeReport([]byte(originalRaw))
	if err != nil {
		return "", err
	}
	if report.IP != ip || original.IP != ip || original.ID != report.ID ||
		report.PolicyVersion != original.PolicyVersion || report.ParserVersion != original.ParserVersion {
		return "", iplookup.ErrRepairConflict
	}
	if _, err = tx.ExecContext(ctx, `UPDATE ip_lookup_reports SET status=$1,report_json=$2 WHERE id=$3`,
		report.Status, string(raw), report.ID); err != nil {
		return "", err
	}
	if err = tx.Commit(); err != nil {
		return "", err
	}
	return ipRepairRevision(string(raw)), nil
}

// UpdateIPLiveSnapshot preserves the original address, snapshot and fetch time;
// only a worker holding the current document revision may publish repaired data.
func (s *Store) retryAttempt_UpdateIPLiveSnapshot(ctx context.Context, id string, details iplookup.LiveDetails) (string, error) {
	if details.RepairRevision == "" || details.SnapshotID != id {
		return "", iplookup.ErrRepairConflict
	}
	details = iplookup.CompactLiveDetails(details)
	raw, err := json.Marshal(details)
	if err != nil {
		return "", err
	}
	if len(raw) > 4*1024*1024 {
		return "", &iplookup.CodeError{Code: "IP_DETAILS_INVALID_RESPONSE"}
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return "", err
	}
	defer tx.Rollback()
	var ip, originalRaw, status string
	err = tx.QueryRowContext(ctx, `SELECT ip,details_json,status FROM ip_lookup_live_snapshots WHERE id=$1 FOR UPDATE`,
		id).Scan(&ip, &originalRaw, &status)
	if errors.Is(err, sql.ErrNoRows) {
		return "", iplookup.ErrRepairConflict
	}
	if err != nil {
		return "", err
	}
	if status != "partial" || ipRepairRevision(originalRaw) != details.RepairRevision {
		return "", iplookup.ErrRepairConflict
	}
	var original iplookup.LiveDetails
	if err = json.Unmarshal([]byte(originalRaw), &original); err != nil {
		return "", err
	}
	original = iplookup.CompactLiveDetails(original)
	if details.IP != ip || details.SourceIP != ip || original.IP != ip || original.SourceIP != ip ||
		details.FetchedAt != original.FetchedAt {
		return "", iplookup.ErrRepairConflict
	}
	status = "succeeded"
	if iplookup.LivePartial(details) {
		status = "partial"
	}
	if _, err = tx.ExecContext(ctx, `UPDATE ip_lookup_live_snapshots SET details_json=$1,status=$2 WHERE id=$3`,
		string(raw), status, id); err != nil {
		return "", err
	}
	if err = tx.Commit(); err != nil {
		return "", err
	}
	return ipRepairRevision(string(raw)), nil
}
