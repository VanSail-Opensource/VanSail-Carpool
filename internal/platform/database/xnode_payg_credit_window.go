package database

import (
	"context"
	"database/sql"
	"math"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
)

// Fixed precision makes indexed comparisons exact at whole-second boundaries.
func paygWindowStamp(at time.Time) string {
	return at.UTC().Format("2006-01-02T15:04:05.000000000Z")
}

// PAYG windows retain only sampled raw counters required for rolling admission
// and unbilled usage. Lifetime usage and financial charges live independently.
func recordPAYGWindowTx(ctx context.Context, tx *sql.Tx, a *xnode.PAYGAccount, through time.Time, bytes int64, now time.Time) error {
	if !a.HasCredit(through) {
		a.CreditBytes = 0
	}
	covered := min(bytes, a.CreditBytes)
	a.CreditBytes -= covered
	if bytes > 0 {
		result, err := tx.ExecContext(ctx, `INSERT INTO xnode_payg_windows(proxy_user_id,through,traffic_bytes,unbilled_bytes) VALUES(?,?,?,?)
 ON CONFLICT(proxy_user_id,through) DO UPDATE SET traffic_bytes=xnode_payg_windows.traffic_bytes+excluded.traffic_bytes,
 unbilled_bytes=xnode_payg_windows.unbilled_bytes+excluded.unbilled_bytes
 WHERE xnode_payg_windows.traffic_bytes<=?`, a.ProxyUserID, paygWindowStamp(through), bytes, bytes-covered, math.MaxInt64-bytes)
		if err != nil {
			return err
		}
		count, err := result.RowsAffected()
		if err != nil {
			return err
		}
		if count != 1 {
			return xnode.ErrInvalid
		}
	}
	if err := settlePAYGWindowTx(ctx, tx, a, through, true, now); err != nil {
		return err
	}
	_, err := tx.ExecContext(ctx, `UPDATE xnode_payg_accounts SET credit_bytes=? WHERE proxy_user_id=?`, a.CreditBytes, a.ProxyUserID)
	return err
}

// The window is (through-72h, through]. A settled raw byte is never charged a
// second time; aging-out unbilled probes do not become lifetime billing debt.
func settlePAYGWindowTx(ctx context.Context, tx *sql.Tx, a *xnode.PAYGAccount, through time.Time, debt bool, now time.Time) error {
	from, to := paygWindowStamp(through.Add(-xnode.PAYGWindow)), paygWindowStamp(through)
	var traffic, pending int64
	if err := tx.QueryRowContext(ctx, `SELECT COALESCE(SUM(traffic_bytes),0),COALESCE(SUM(unbilled_bytes),0) FROM xnode_payg_windows WHERE proxy_user_id=? AND through>? AND through<=?`, a.ProxyUserID, from, to).Scan(&traffic, &pending); err != nil {
		return err
	}
	if traffic >= xnode.PAYGStartBytes && pending > 0 {
		if !a.HasCredit(through) {
			a.CreditBytes = 0
		}
		blocks := pending / xnode.PAYGGB
		if pending%xnode.PAYGGB != 0 {
			blocks++
		}
		if err := buyPAYGTx(ctx, tx, a, blocks, debt, now); err != nil {
			return err
		}
		a.CreditBytes -= pending
		if _, err := tx.ExecContext(ctx, `UPDATE xnode_payg_accounts SET credit_bytes=? WHERE proxy_user_id=?`, a.CreditBytes, a.ProxyUserID); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `UPDATE xnode_payg_windows SET unbilled_bytes=0 WHERE proxy_user_id=? AND through>? AND through<=?`, a.ProxyUserID, from, to); err != nil {
			return err
		}
	}
	// Once the newest samples already prove the threshold, older settled
	// samples can never affect a future rolling decision: they expire first.
	// Keep pending evidence, and avoid scanning 72 hours of high-volume reports.
	if traffic >= xnode.PAYGStartBytes {
		if _, err := tx.ExecContext(ctx, `DELETE FROM xnode_payg_windows WHERE proxy_user_id=? AND unbilled_bytes=0 AND through<(
 SELECT MAX(through) FROM (SELECT through,SUM(traffic_bytes) OVER (ORDER BY through DESC) AS total
 FROM xnode_payg_windows WHERE proxy_user_id=? AND through>? AND through<=?) recent WHERE total>=?)`, a.ProxyUserID, a.ProxyUserID, from, to, xnode.PAYGStartBytes); err != nil {
			return err
		}
	}
	_, err := tx.ExecContext(ctx, `DELETE FROM xnode_payg_windows WHERE proxy_user_id=? AND through<=?`, a.ProxyUserID, from)
	return err
}
