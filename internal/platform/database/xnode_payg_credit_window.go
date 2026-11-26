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
	priorCredit, priorPurchased := a.CreditBytes, a.PurchasedBytes
	if !a.HasCredit(through) {
		a.CreditBytes = 0
	}
	covered := min(bytes, a.CreditBytes)
	a.CreditBytes -= covered
	if bytes > 0 {
		result, err := tx.ExecContext(ctx, `INSERT INTO xnode_payg_windows(proxy_user_id,through,traffic_bytes,unbilled_bytes) VALUES($1,$2,$3,$4)
 ON CONFLICT(proxy_user_id,through) DO UPDATE SET traffic_bytes=xnode_payg_windows.traffic_bytes+excluded.traffic_bytes,
 unbilled_bytes=xnode_payg_windows.unbilled_bytes+excluded.unbilled_bytes
 WHERE xnode_payg_windows.traffic_bytes<=$5`, a.ProxyUserID, paygWindowStamp(through), bytes, bytes-covered, math.MaxInt64-bytes)
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
	// Settlement writes the post-purchase remainder itself. Otherwise only
	// persist an actual consumption or expiry, not an identical row version.
	if a.PurchasedBytes != priorPurchased || a.CreditBytes == priorCredit {
		return nil
	}
	_, err := tx.ExecContext(ctx, `UPDATE xnode_payg_accounts SET credit_bytes=$1 WHERE proxy_user_id=$2`, a.CreditBytes, a.ProxyUserID)
	return err
}

// The window is (through-72h, through]. A settled raw byte is never charged a
// second time; aging-out unbilled probes do not become lifetime billing debt.
func settlePAYGWindowTx(ctx context.Context, tx *sql.Tx, a *xnode.PAYGAccount, through time.Time, debt bool, now time.Time) error {
	from, to := paygWindowStamp(through.Add(-xnode.PAYGWindow)), paygWindowStamp(through)
	traffic, pending, cutoff, err := loadPAYGWindow(ctx, tx, a.ProxyUserID, from, to)
	if err != nil {
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
		if _, err := tx.ExecContext(ctx, `UPDATE xnode_payg_accounts SET credit_bytes=$1 WHERE proxy_user_id=$2`, a.CreditBytes, a.ProxyUserID); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `UPDATE xnode_payg_windows SET unbilled_bytes=0 WHERE proxy_user_id=$1 AND through>$2 AND through<=$3 AND unbilled_bytes<>0`, a.ProxyUserID, from, to); err != nil {
			return err
		}
	}
	return prunePAYGWindow(ctx, tx, a.ProxyUserID, from, cutoff)
}

func loadPAYGWindow(ctx context.Context, tx *sql.Tx, id int64, from, to string) (traffic, pending int64, cutoff sql.NullString, err error) {
	// The first cumulative prefix strictly above the excess is the oldest
	// sample needed to prove the threshold. The inner LIMIT favors a PK scan
	// over sorting the whole window for recurring small prunes. If the prefix
	// is not enough, read the exact suffix as a fallback. All reads share one
	// snapshot because retention does not take the account's settlement lock.
	err = tx.QueryRowContext(ctx, `SELECT sums.traffic,sums.pending,CASE WHEN sums.traffic >= $4::bigint THEN COALESCE((
 SELECT through FROM (
  SELECT through,SUM(traffic_bytes) OVER (ORDER BY through ROWS UNBOUNDED PRECEDING) AS prefix
  FROM (SELECT through,traffic_bytes FROM xnode_payg_windows WHERE proxy_user_id=$1 AND through>$2 AND through<=$3 ORDER BY through LIMIT 128) oldest
 ) prefixes WHERE prefix>sums.traffic-$4 ORDER BY through LIMIT 1
 ),(
 SELECT through FROM (
  SELECT through,SUM(traffic_bytes) OVER (ORDER BY through DESC ROWS UNBOUNDED PRECEDING) AS suffix
  FROM xnode_payg_windows WHERE proxy_user_id=$1 AND through>$2 AND through<=$3
 ) suffixes WHERE suffix>=$4 ORDER BY through DESC LIMIT 1
 )) END FROM (
 SELECT COALESCE(SUM(traffic_bytes),0) AS traffic,COALESCE(SUM(unbilled_bytes),0) AS pending
 FROM xnode_payg_windows WHERE proxy_user_id=$1 AND through>$2 AND through<=$3
 ) sums`, id, from, to, xnode.PAYGStartBytes).Scan(&traffic, &pending, &cutoff)
	return traffic, pending, cutoff, err
}

func prunePAYGWindow(ctx context.Context, tx *sql.Tx, id int64, from string, cutoff sql.NullString) error {
	if cutoff.Valid {
		// Expired rows and older settled rows share one index range. Pending
		// evidence is kept until it is settled or actually ages out.
		_, err := tx.ExecContext(ctx, `DELETE FROM xnode_payg_windows WHERE proxy_user_id=$1 AND through<$2 AND (through<=$3 OR unbilled_bytes=0)`, id, cutoff.String, from)
		return err
	}
	_, err := tx.ExecContext(ctx, `DELETE FROM xnode_payg_windows WHERE proxy_user_id=$1 AND through<=$2`, id, from)
	return err
}
