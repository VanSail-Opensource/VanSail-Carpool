package database

import (
	"context"
	"database/sql"
	"errors"
	"math"
	"sort"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/pricing"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
	wire "github.com/VanSail-Opensource/X-Node/pkg/protocol"
)

// Balances precede periods in every PAYG transaction, including reports from
// different nodes spending the same wallet. The journal cursor remains atomic.
func lockPAYGOwnersTx(ctx context.Context, tx *sql.Tx, node string, ids map[int64]bool) error {
	if len(ids) == 0 {
		return nil
	}
	rows, err := tx.QueryContext(ctx, `SELECT proxy_user_id,user_id FROM xnode_payg_accounts WHERE node_id=? ORDER BY user_id`, node)
	if err != nil {
		return err
	}
	users := map[string]bool{}
	for rows.Next() {
		var id int64
		var user string
		if err = rows.Scan(&id, &user); err != nil {
			rows.Close()
			return err
		}
		if ids[id] {
			users[user] = true
		}
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	ordered := []string{}
	for user := range users {
		ordered = append(ordered, user)
	}
	sort.Strings(ordered)
	for _, user := range ordered {
		if _, err = balanceTx(ctx, tx, user); err != nil {
			return err
		}
	}
	return nil
}

func recordPAYGSampleTx(ctx context.Context, tx *sql.Tx, node string, sample wire.TrafficSample) (bool, error) {
	a, err := scanPAYG(tx.QueryRowContext(ctx, paygSelect+` WHERE a.proxy_user_id=?`, sample.UserID))
	if errors.Is(err, xnode.ErrNotFound) {
		return false, nil
	}
	if err != nil {
		return true, err
	}
	if node != a.NodeID || sample.AccountingPeriod != a.PeriodID {
		return true, xnode.ErrTrafficOwnerMismatch
	}
	if sample.From.IsZero() || sample.Through.Before(sample.From) || sample.Through.After(time.Now().Add(5*time.Minute)) || sample.UploadBytes > math.MaxInt64 || sample.DownloadBytes > math.MaxInt64-sample.UploadBytes {
		return true, xnode.ErrInvalid
	}
	var status string
	if err = tx.QueryRowContext(ctx, `SELECT status FROM xnode_periods WHERE id=?`, a.PeriodID).Scan(&status); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return true, xnode.ErrTrafficPAYGPeriodMissing
		}
		return true, err
	}
	if status != "open" {
		return true, xnode.ErrTrafficPeriodSettled
	}
	bytes := int64(sample.UploadBytes + sample.DownloadBytes)
	if a.UsedBytes > math.MaxInt64-bytes {
		return true, xnode.ErrInvalid
	}
	a.UsedBytes += bytes
	if err = recordPAYGWindowTx(ctx, tx, &a, sample.Through, bytes, time.Now().UTC()); err != nil {
		return true, err
	}
	if _, err = tx.ExecContext(ctx, `UPDATE xnode_periods SET used_bytes=? WHERE id=? AND status='open'`, a.UsedBytes, a.PeriodID); err != nil {
		return true, err
	}
	return true, recordPAYGMinuteTx(ctx, tx, a.UserID, a.GroupID, sample.Through, bytes, 0)
}

func (s *Store) retryAttempt_ObserveXNodePAYGConnections(ctx context.Context, node string, ids []int64, now time.Time) error {
	ctx = pricing.WithHealth(ctx, s.pricingHealth)
	if len(ids) == 0 {
		return nil
	}
	if !IsPostgres(s.db) {
		s.writeMu.Lock()
		defer s.writeMu.Unlock()
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	selected := map[int64]bool{}
	for _, id := range ids {
		selected[id] = true
	}
	if err = lockPAYGOwnersTx(ctx, tx, node, selected); err != nil {
		return err
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	for _, id := range ids {
		a, e := scanPAYG(tx.QueryRowContext(ctx, paygSelect+` WHERE a.proxy_user_id=? AND a.node_id=?`, id, node))
		if errors.Is(e, xnode.ErrNotFound) {
			continue
		}
		if e != nil {
			return e
		}
		var locked string
		if e = tx.QueryRowContext(ctx, `SELECT id FROM xnode_periods WHERE id=? /*pg:FOR UPDATE*/`, a.PeriodID).Scan(&locked); e != nil {
			return e
		}
		a, e = scanPAYG(tx.QueryRowContext(ctx, paygSelect+` WHERE a.proxy_user_id=?`, id))
		if e != nil {
			return e
		}
		if a.Disabled || a.HasCredit(now) {
			continue
		}
		// Commit the future block offer before more service is delivered. Historical
		// traffic reconciliation continues using this accepted account price.
		price, priceErr := resolvePricingTx(ctx, tx, "payg", a.GroupID, a.UnitPriceMinor, now)
		if priceErr != nil {
			var code pricing.Code
			if errors.As(priceErr, &code) {
				continue
			}
			return priceErr
		}
		personal, e := personalPricingTx(ctx, tx, a.UserID, "payg", a.GroupID, price, 1, "", false, now)
		if e != nil {
			return e
		}
		if personal <= 0 || a.BalanceMinor < personal {
			continue
		}
		a.UnitPriceMinor = price
		if _, e = tx.ExecContext(ctx, `UPDATE xnode_payg_accounts SET unit_price_minor=? WHERE proxy_user_id=?`, price, a.ProxyUserID); e != nil {
			return e
		}
		if _, e = tx.ExecContext(ctx, "SAVEPOINT pricing_payg_offer"); e != nil {
			return e
		}
		if e = settlePAYGWindowTx(ctx, tx, &a, now, false, now); errors.Is(e, ErrInsufficientBalance) {
			if _, e = tx.ExecContext(ctx, "ROLLBACK TO SAVEPOINT pricing_payg_offer"); e != nil {
				return e
			}
			if _, e = tx.ExecContext(ctx, "RELEASE SAVEPOINT pricing_payg_offer"); e != nil {
				return e
			}
			continue
		} else if e != nil {
			return e
		}
		if _, e = tx.ExecContext(ctx, "RELEASE SAVEPOINT pricing_payg_offer"); e != nil {
			return e
		}
	}
	return tx.Commit()
}
