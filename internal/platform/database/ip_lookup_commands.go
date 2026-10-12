package database

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"time"

	"github.com/txyyddss/Remna-User-Panel/internal/iplookup"
	"github.com/txyyddss/Remna-User-Panel/internal/model"
	"github.com/txyyddss/Remna-User-Panel/internal/platform/ids"
	"github.com/txyyddss/Remna-User-Panel/internal/providerops"
)

// CreateIPLookupCheck atomically replays or charges, joins a report and queues its receipt.
func (s *Store) CreateIPLookupCheck(ctx context.Context, user, key string, q iplookup.Quote, now time.Time) (model.OperationReceipt, error) {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return model.OperationReceipt{}, err
	}
	defer func() { _ = tx.Rollback() }()
	checkID, err := ids.New()
	if err != nil {
		return model.OperationReceipt{}, err
	}
	fingerprint := sha256.Sum256([]byte(q.IP + "\x00" + fmtBool(q.Refresh)))
	op, replay, err := createProviderOperationTx(ctx, tx, providerops.CreateInput{ActorUserID: user, OwnerUserID: user, Kind: iplookup.OperationKind, IdempotencyKey: key, RequestFingerprint: hex.EncodeToString(fingerprint[:]), Items: []providerops.ItemInput{{Key: "check", TargetType: "ip_lookup_check", TargetID: checkID}}}, now)
	if err != nil {
		return model.OperationReceipt{}, err
	}
	if replay {
		return op.Receipt, tx.Commit()
	}
	var running int
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM ip_lookup_reports WHERE ip=? AND status='processing'`, q.IP).Scan(&running); err != nil {
		return model.OperationReceipt{}, err
	}
	if running > 0 {
		return model.OperationReceipt{}, &iplookup.CodeError{Code: "IP_LOOKUP_BUSY"}
	}
	fresh, c, err := ipQuoteTx(ctx, tx, user, q.IP, q.Refresh, now)
	if err != nil {
		return model.OperationReceipt{}, err
	}
	becameCached := fresh.CacheReportID != "" && (!q.Refresh || fresh.CacheReportID != q.CacheReportID)
	if q.ExpiresAt <= now.Unix() || fresh.ConfigHash != q.ConfigHash || (!becameCached && (fresh.Charge.Minor != q.Charge.Minor || fresh.UseQuota != q.UseQuota || (q.QuotaCost != 0 && fresh.QuotaCost != q.QuotaCost) || (fresh.UseQuota && (fresh.PurchaseID != q.PurchaseID || fresh.Remaining != q.Remaining)))) {
		return model.OperationReceipt{}, &iplookup.CodeError{Code: "IP_LOOKUP_QUOTE_CHANGED"}
	}
	reportID, cached, err := selectIPRunTx(ctx, tx, q, fresh, c, now)
	if err != nil {
		return model.OperationReceipt{}, err
	}
	charge := fresh.Charge.MinorInt64()
	if cached && q.Refresh {
		charge = 0
		fresh.UseQuota = false
		fresh.QuotaCost = 0
	}
	var allowance any
	if fresh.UseQuota {
		allowance = fresh.PurchaseID
		result, err := tx.ExecContext(ctx, `UPDATE ip_lookup_allowances SET used=used+? WHERE purchase_id=? AND used+?<=total`, fresh.QuotaCost, allowance, fresh.QuotaCost)
		if err != nil {
			return model.OperationReceipt{}, err
		}
		if n, err := result.RowsAffected(); err != nil {
			return model.OperationReceipt{}, err
		} else if n != 1 {
			return model.OperationReceipt{}, ErrConflict
		}
	} else if charge > 0 {
		balance, err := changeBalanceTx(ctx, tx, user, -charge, now)
		if err != nil {
			return model.OperationReceipt{}, err
		}
		kind := "ip_lookup"
		if q.Refresh {
			kind = "ip_lookup_refresh"
		}
		if _, err := insertLedgerTx(ctx, tx, user, -charge, balance, kind, op.Receipt.ID, "IP reputation check", now); err != nil {
			return model.OperationReceipt{}, err
		}
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO ip_lookup_checks(id,operation_id,user_id,report_id,allowance_id,charge_minor,cached,quota_consumed) VALUES(?,?,?,?,?,?,?,?)`, checkID, op.Receipt.ID, user, reportID, allowance, charge, boolInt(cached), fresh.QuotaCost)
	if err != nil {
		return model.OperationReceipt{}, err
	}
	cacheMatch := "none"
	if cached {
		var evaluatedIP string
		if err := tx.QueryRowContext(ctx, `SELECT ip FROM ip_lookup_reports WHERE id=?`, reportID).Scan(&evaluatedIP); err != nil {
			return model.OperationReceipt{}, err
		}
		cacheMatch = "exact"
		if evaluatedIP != q.IP {
			cacheMatch = "subnet"
		}
	}
	metadata, err := json.Marshal(map[string]any{"reportId": reportID, "usedQuota": fresh.UseQuota, "quotaConsumed": fresh.QuotaCost, "requestedIP": q.IP, "cacheMatch": cacheMatch})
	if err != nil {
		return model.OperationReceipt{}, err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE provider_operations SET result_json=? WHERE id=?`, string(metadata), op.Receipt.ID); err != nil {
		return model.OperationReceipt{}, err
	}
	if cached {
		var status string
		if err := tx.QueryRowContext(ctx, `SELECT status FROM ip_lookup_reports WHERE id=?`, reportID).Scan(&status); err != nil {
			return model.OperationReceipt{}, err
		}
		if err := finishIPCheckTx(ctx, tx, op.Receipt.ID, reportID, status, now); err != nil {
			return model.OperationReceipt{}, err
		}
		op, err = scanProviderOperation(tx.QueryRowContext(ctx, providerOperationSelect+` WHERE id=?`, op.Receipt.ID))
		if err != nil {
			return model.OperationReceipt{}, err
		}
	}
	return op.Receipt, tx.Commit()
}

func fmtBool(value bool) string {
	if value {
		return "refresh"
	}
	return "lookup"
}

func selectIPRunTx(ctx context.Context, tx *sql.Tx, q, fresh iplookup.Quote, c iplookup.Config, now time.Time) (string, bool, error) {
	// A refresh completed since this quote already fulfills the quoted refresh.
	if fresh.CacheReportID != "" && (!q.Refresh || fresh.CacheReportID != q.CacheReportID) {
		return fresh.CacheReportID, true, nil
	}
	var id string
	err := tx.QueryRowContext(ctx, `SELECT id FROM ip_lookup_reports WHERE ip=? AND status='processing'`, q.IP).Scan(&id)
	if err == nil {
		if q.Refresh {
			if _, err := tx.ExecContext(ctx, `UPDATE ip_lookup_reports SET live_state_json=json_set(live_state_json,'$.status','queued') WHERE id=? AND COALESCE(json_extract(live_state_json,'$.status'),'')=''`, id); err != nil {
				return "", false, err
			}
		}
		return id, false, nil
	}
	if err != sql.ErrNoRows {
		return "", false, err
	}
	id, err = ids.New()
	if err != nil {
		return "", false, err
	}
	report := iplookup.NewReport(id, q.IP, c)
	encoded, err := iplookup.EncodeReport(report)
	if err != nil {
		return "", false, err
	}
	liveState := `{}`
	if q.Refresh {
		liveState = `{"status":"queued","attempted":false,"snapshotId":""}`
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO ip_lookup_reports(id,ip,status,report_json,created_at,live_state_json,cache_key) VALUES(?,?,'processing',?,?,?,?)`, id, q.IP, string(encoded), stamp(now), liveState, iplookup.CacheKey(q.IP))
	return id, false, err
}
