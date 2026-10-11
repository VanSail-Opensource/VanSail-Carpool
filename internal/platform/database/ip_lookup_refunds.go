package database

import (
	"context"
	"database/sql"
	"time"

	"github.com/txyyddss/Remna-User-Panel/internal/iplookup"
)

func refundIPCheckTx(ctx context.Context, tx *sql.Tx, operationID string, now time.Time) error {
	var user string
	var charge int64
	var allowance sql.NullString
	var refunded int
	if err := tx.QueryRowContext(ctx, `SELECT user_id,charge_minor,allowance_id,refunded FROM ip_lookup_checks WHERE operation_id=?`, operationID).Scan(&user, &charge, &allowance, &refunded); err != nil {
		return err
	}
	if refunded == 1 {
		return nil
	}
	if allowance.Valid {
		if _, err := tx.ExecContext(ctx, `UPDATE ip_lookup_allowances SET used=used-1 WHERE purchase_id=? AND used>0`, allowance.String); err != nil {
			return err
		}
	}
	if charge > 0 {
		balance, err := changeBalanceTx(ctx, tx, user, charge, now)
		if err != nil {
			return err
		}
		if _, err := insertLedgerTx(ctx, tx, user, charge, balance, "ip_lookup_refund", operationID, "IP lookup not attempted", now); err != nil {
			return err
		}
	}
	_, err := tx.ExecContext(ctx, `UPDATE ip_lookup_checks SET refunded=1 WHERE operation_id=?`, operationID)
	return err
}

// reconcileStaleIPLookupsTx releases stuck shared runs before generic receipt maintenance.
func reconcileStaleIPLookupsTx(ctx context.Context, tx *sql.Tx, now time.Time) error {
	rows, err := tx.QueryContext(ctx, `SELECT DISTINCT r.id,r.report_json FROM maintenance_operation_candidates candidate JOIN ip_lookup_checks c ON c.operation_id=candidate.id JOIN ip_lookup_reports r ON r.id=c.report_id WHERE r.status='processing'`)
	if err != nil {
		return err
	}
	type stalled struct{ id, raw string }
	var runs []stalled
	for rows.Next() {
		var run stalled
		if err := rows.Scan(&run.id, &run.raw); err != nil {
			_ = rows.Close()
			return err
		}
		runs = append(runs, run)
	}
	err = rows.Err()
	_ = rows.Close()
	if err != nil {
		return err
	}
	for _, run := range runs {
		report, err := iplookup.DecodeReport([]byte(run.raw))
		if err != nil {
			return err
		}
		liveStage, err := ipLiveStageTx(ctx, tx, run.id)
		if err != nil {
			return err
		}
		if liveStage.Status == "processing" {
			snapshot, status, err := liveSnapshotForRunTx(ctx, tx, run.id)
			if err != nil {
				return err
			}
			liveStage.SnapshotID, liveStage.Status = snapshot, status
			if snapshot == "" {
				liveStage.Status = "error"
			}
		}
		if liveStage.Status == "queued" || (liveStage.Status == "" && liveStage.Attempted) {
			liveStage.Status = "error"
		}
		if err := saveLiveStageTx(ctx, tx, run.id, liveStage); err != nil {
			return err
		}
		attempted := iplookup.ReportAttempted(report) || liveStage.Attempted
		if report.Checkpoint != nil {
			for i := range report.Checkpoint.Stages {
				stage := &report.Checkpoint.Stages[i]
				if stage.Status == "queued" || stage.Status == "processing" {
					stage.Status = "error"
					report.Checkpoint.Complete = false
				}
			}
		}
		report = iplookup.Aggregate(report, now)
		if !attempted {
			report.Status, report.Verdict = "failed", "inconclusive"
		}
		if attempted && report.Status == "failed" {
			report.Status = "partial"
		}
		raw, err := iplookup.EncodeReport(report)
		if err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `UPDATE ip_lookup_reports SET status=?,report_json=?,completed_at=? WHERE id=?`, report.Status, string(raw), stamp(now), run.id); err != nil {
			return err
		}
		if err := scheduleIPLookupReportRetryTx(ctx, tx, run.id, report, now); err != nil {
			return err
		}
		checkRows, err := tx.QueryContext(ctx, `SELECT operation_id FROM ip_lookup_checks WHERE report_id=?`, run.id)
		if err != nil {
			return err
		}
		var checks []string
		for checkRows.Next() {
			var id string
			if err := checkRows.Scan(&id); err != nil {
				_ = checkRows.Close()
				return err
			}
			checks = append(checks, id)
		}
		err = checkRows.Err()
		_ = checkRows.Close()
		if err != nil {
			return err
		}
		for _, id := range checks {
			if !attempted {
				if err := refundIPCheckTx(ctx, tx, id, now); err != nil {
					return err
				}
			}
			status := report.Status
			if status == "succeeded" && (liveStage.Status == "partial" || liveStage.Status == "error") {
				status = "partial"
			}
			if err := finishIPCheckTx(ctx, tx, id, run.id, status, now); err != nil {
				return err
			}
		}
	}
	// Domain-settled IP receipts must not enter generic failure/compensation handling.
	_, err = tx.ExecContext(ctx, `DELETE FROM maintenance_operation_candidates WHERE id IN (SELECT operation_id FROM ip_lookup_checks)`)
	return err
}
