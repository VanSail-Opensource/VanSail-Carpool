package database

import (
	"context"
	"database/sql"
	"time"

	"github.com/txyyddss/Remna-User-Panel/internal/iplookup"
	"github.com/txyyddss/Remna-User-Panel/internal/model"
)

func ipConfigTx(ctx context.Context, tx *sql.Tx) (iplookup.Config, error) {
	var raw string
	if err := tx.QueryRowContext(ctx, `SELECT value FROM settings WHERE key=?`, iplookup.SettingKey).Scan(&raw); err != nil && err != sql.ErrNoRows {
		return iplookup.Config{}, err
	}
	return iplookup.DecodeConfig(raw)
}

func ipAllowanceTx(ctx context.Context, tx *sql.Tx, user string, now time.Time) (*iplookup.Allowance, error) {
	var a iplookup.Allowance
	err := tx.QueryRowContext(ctx, `SELECT p.id,a.total,a.total-a.used,p.valid_until FROM purchases p JOIN ip_lookup_allowances a ON a.purchase_id=p.id
		WHERE p.user_id=? AND p.status='active' AND julianday(p.valid_from)<=julianday(?) AND julianday(p.valid_until)>julianday(?) ORDER BY p.valid_from DESC,p.id LIMIT 1`, user, stamp(now), stamp(now)).Scan(&a.PurchaseID, &a.Total, &a.Remaining, &a.ValidUntil)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &a, nil
}

// IPLookupState reads safe prices and the active purchase allowance.
func (s *Store) IPLookupState(ctx context.Context, user string, now time.Time) (iplookup.State, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return iplookup.State{}, err
	}
	defer func() { _ = tx.Rollback() }()
	c, err := ipConfigTx(ctx, tx)
	if err != nil {
		return iplookup.State{}, err
	}
	state := iplookup.State{Enabled: c.Enabled}
	for index, raw := range []string{c.LookupFeeTXB, c.RefreshFeeTXB} {
		if raw == "" {
			continue
		}
		amount, err := model.ParseTXBMajor(raw)
		if err != nil {
			return state, err
		}
		money := model.TXBMoney(amount)
		if index == 0 {
			state.LookupFee = &money
		} else {
			state.RefreshFee = &money
		}
	}
	state.Allowance, err = ipAllowanceTx(ctx, tx, user, now)
	return state, err
}

func ipQuoteTx(ctx context.Context, tx *sql.Tx, user, ip string, refresh bool, now time.Time) (iplookup.Quote, iplookup.Config, error) {
	q := iplookup.Quote{IP: ip, Refresh: refresh}
	c, err := ipConfigTx(ctx, tx)
	if err != nil {
		return q, c, err
	}
	if !c.Enabled {
		return q, c, &iplookup.CodeError{Code: "IP_LOOKUP_DISABLED"}
	}
	q.ConfigHash = iplookup.ConfigHash(c)
	cached, _, err := ipCachedReportTx(ctx, tx, ip, !refresh)
	if err != nil {
		return q, c, err
	}
	if cached != nil {
		q.CacheReportID = cached.ID
	}
	if refresh && q.CacheReportID == "" {
		return q, c, &iplookup.CodeError{Code: "IP_LOOKUP_REFRESH_UNAVAILABLE"}
	}
	a, err := ipAllowanceTx(ctx, tx, user, now)
	if err != nil {
		return q, c, err
	}
	fee := c.LookupFeeTXB
	if refresh {
		fee = c.RefreshFeeTXB
	}
	amount, err := model.ParseTXBMajor(fee)
	if err != nil {
		return q, c, err
	}
	if a != nil {
		q.PurchaseID = a.PurchaseID
		q.Remaining = a.Remaining
	}
	q.UseQuota = !refresh && a != nil && a.Remaining > 0
	if q.UseQuota {
		amount = 0
	}
	if !refresh && q.CacheReportID != "" {
		amount = 0
		q.UseQuota = false
	}
	q.Charge = model.TXBMoney(amount)
	return q, c, nil
}

// IPLookupQuote does not reserve funds or run providers.
func (s *Store) IPLookupQuote(ctx context.Context, user, ip string, refresh bool, now time.Time) (iplookup.Quote, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return iplookup.Quote{}, err
	}
	defer func() { _ = tx.Rollback() }()
	q, _, err := ipQuoteTx(ctx, tx, user, ip, refresh, now)
	return q, err
}

// IPLookupCheck returns an owner-scoped receipt with the immutable shared report.
func (s *Store) IPLookupCheck(ctx context.Context, user, id string) (iplookup.Check, error) {
	var result iplookup.Check
	var raw string
	var cached, refunded int
	var usedQuota int
	var charge int64
	err := s.db.QueryRowContext(ctx, `SELECT r.report_json,c.cached,c.refunded,COALESCE(json_extract(o.result_json,'$.usedQuota'),c.allowance_id IS NOT NULL),c.charge_minor,COALESCE(json_extract(o.result_json,'$.requestedIP'),r.ip),COALESCE(json_extract(o.result_json,'$.cacheMatch'),CASE c.cached WHEN 1 THEN 'exact' ELSE 'none' END) FROM ip_lookup_checks c JOIN ip_lookup_reports r ON r.id=c.report_id JOIN provider_operations o ON o.id=c.operation_id WHERE c.operation_id=? AND c.user_id=?`, id, user).Scan(&raw, &cached, &refunded, &usedQuota, &charge, &result.RequestedIP, &result.CacheMatch)
	if err == sql.ErrNoRows {
		return result, ErrNotFound
	}
	if err != nil {
		return result, err
	}
	result.Operation, err = s.ProviderOperationForOwner(ctx, id, user)
	if err != nil {
		return result, err
	}
	report, err := iplookup.DecodeReport([]byte(raw))
	if err != nil {
		return result, err
	}
	result.Progress = iplookup.CheckProgress(report)
	if report.Status != "processing" && report.Status != "failed" {
		public := iplookup.PublicReport(report)
		result.Report = &public
	}
	result.Cached, result.Refunded, result.UsedQuota = cached == 1, refunded == 1, usedQuota == 1
	if result.Refunded {
		charge = 0
	}
	result.Charge = model.TXBMoney(charge)
	return result, nil
}
