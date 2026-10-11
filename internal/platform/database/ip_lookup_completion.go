package database

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"

	"github.com/txyyddss/Remna-User-Panel/internal/iplookup"
)

// IPLookupRun loads the shared provider run through a durable operation reference.
func (s *Store) IPLookupRun(ctx context.Context, operationID string) (iplookup.Report, iplookup.Config, error) {
	var report iplookup.Report
	var config iplookup.Config
	var raw, configJSON string
	err := s.db.QueryRowContext(ctx, `SELECT r.report_json,r.config_json FROM ip_lookup_checks c JOIN ip_lookup_reports r ON r.id=c.report_id WHERE c.operation_id=?`, operationID).Scan(&raw, &configJSON)
	if err == sql.ErrNoRows {
		return report, config, ErrNotFound
	}
	if err != nil {
		return report, config, err
	}
	report, err = iplookup.DecodeReport([]byte(raw))
	if err != nil {
		return report, config, err
	}
	config, err = iplookup.DecodeConfig(configJSON)
	return report, config, err
}

// SaveIPLookupProgress persists each attempt marker and completed normalized stage.
func (s *Store) SaveIPLookupProgress(ctx context.Context, report iplookup.Report) error {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	raw, err := iplookup.EncodeReport(report)
	if err != nil {
		return err
	}
	result, err := s.db.ExecContext(ctx, `UPDATE ip_lookup_reports SET report_json=? WHERE id=? AND status='processing'`, string(raw), report.ID)
	if err != nil {
		return err
	}
	if n, err := result.RowsAffected(); err != nil {
		return err
	} else if n != 1 {
		return ErrConflict
	}
	return nil
}

// FinishIPLookupRun freezes the report and completes/refunds every joined check atomically.
func (s *Store) FinishIPLookupRun(ctx context.Context, report iplookup.Report, now time.Time) error {
	if report.Status == "failed" && iplookup.ReportAttempted(report) {
		report.Status, report.Verdict = "partial", "inconclusive"
	}
	if report.Status != "succeeded" && report.Status != "partial" && report.Status != "failed" {
		return ErrConflict
	}
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	liveStage, err := ipLiveStageTx(ctx, tx, report.ID)
	if err != nil {
		return err
	}
	if liveStage.Status == "queued" || liveStage.Status == "processing" {
		return &iplookup.CodeError{Code: "IP_LOOKUP_BUSY"}
	}
	if report.Status == "failed" && liveStage.Attempted {
		report.Status, report.Verdict = "partial", "inconclusive"
	}
	raw, err := iplookup.EncodeReport(report)
	if err != nil {
		return err
	}
	result, err := tx.ExecContext(ctx, `UPDATE ip_lookup_reports SET status=?,report_json=?,completed_at=? WHERE id=? AND status='processing'`, report.Status, string(raw), stamp(now), report.ID)
	if err != nil {
		return err
	}
	if affected, err := result.RowsAffected(); err != nil {
		return err
	} else if affected == 0 {
		var stored string
		if err := tx.QueryRowContext(ctx, `SELECT report_json FROM ip_lookup_reports WHERE id=?`, report.ID).Scan(&stored); err != nil {
			return err
		}
		report, err = iplookup.DecodeReport([]byte(stored))
		if err != nil {
			return err
		}
	} else if err := scheduleIPLookupReportRetryTx(ctx, tx, report.ID, report, now); err != nil {
		return err
	}
	rows, err := tx.QueryContext(ctx, `SELECT operation_id FROM ip_lookup_checks WHERE report_id=?`, report.ID)
	if err != nil {
		return err
	}
	var operations []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			_ = rows.Close()
			return err
		}
		operations = append(operations, id)
	}
	err = rows.Err()
	_ = rows.Close()
	if err != nil {
		return err
	}
	for _, id := range operations {
		if report.Status == "failed" {
			if err := refundIPCheckTx(ctx, tx, id, now); err != nil {
				return err
			}
		}
		status := report.Status
		if status == "succeeded" && (liveStage.Status == "partial" || liveStage.Status == "error") {
			status = "partial"
		}
		if err := finishIPCheckTx(ctx, tx, id, report.ID, status, now); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func finishIPCheckTx(ctx context.Context, tx *sql.Tx, operationID, reportID, status string, now time.Time) error {
	var usedQuota int
	var metadata string
	if err := tx.QueryRowContext(ctx, `SELECT COALESCE(json_extract(o.result_json,'$.usedQuota'),c.allowance_id IS NOT NULL),o.result_json FROM ip_lookup_checks c JOIN provider_operations o ON o.id=c.operation_id WHERE c.operation_id=?`, operationID).Scan(&usedQuota, &metadata); err != nil {
		return err
	}
	errorCode := ""
	if status == "failed" {
		errorCode = "IP_LOOKUP_FAILED"
	}
	result := map[string]any{}
	if err := json.Unmarshal([]byte(metadata), &result); err != nil {
		return err
	}
	result["reportId"], result["usedQuota"] = reportID, usedQuota == 1
	resultJSON, err := json.Marshal(result)
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `UPDATE provider_operations SET status=?,error_code=?,result_json=?,attempt_started_at=COALESCE(attempt_started_at,?),completed_at=?,updated_at=? WHERE id=? AND status IN ('queued','processing')`, status, errorCode, string(resultJSON), stamp(now), stamp(now), stamp(now), operationID)
	if err != nil {
		return err
	}
	itemStatus := status
	// The single item is report delivery; coverage remains partial on the report/receipt.
	if itemStatus == "partial" {
		itemStatus = "succeeded"
	}
	_, err = tx.ExecContext(ctx, `UPDATE provider_operation_items SET status=?,error_code=?,result_json=?,completed_at=?,updated_at=? WHERE operation_id=? AND status IN ('queued','processing')`, itemStatus, errorCode, string(resultJSON), stamp(now), stamp(now), operationID)
	return err
}
