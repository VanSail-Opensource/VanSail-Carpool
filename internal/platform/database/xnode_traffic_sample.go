package database

import (
	"context"
	"database/sql"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
	wire "github.com/VanSail-Opensource/X-Node/pkg/protocol"
	"math"
	"time"
)

func recordXNodeSample(ctx context.Context, tx *sql.Tx, node string, multiplier int64, sample wire.TrafficSample, buffers ...*nativeReportBuffer) error {
	if sample.UserID <= 0 || !wire.ValidName(sample.AccountingPeriod) || sample.From.IsZero() || sample.Through.Before(sample.From) || sample.UploadBytes > math.MaxInt64 || sample.DownloadBytes > math.MaxInt64 || sample.UploadBytes > math.MaxInt64-sample.DownloadBytes {
		return xnode.ErrInvalid
	}
	var buffer *nativeReportBuffer
	if len(buffers) > 0 {
		buffer = buffers[0]
	}
	history := buffer
	period, err := nativePeriodTx(ctx, tx, buffer, sample.AccountingPeriod)
	if err != nil {
		return err
	}
	// PAYG probe attribution keeps the existing immediate financial path. A
	// deferred native counter may never hide bytes from a later wallet charge.
	if buffer != nil && period.payg {
		buffer = nil
		period, err = nativePeriodTx(ctx, tx, nil, sample.AccountingPeriod)
		if err != nil {
			return err
		}
	}
	owner, status := period.owner, period.status
	if status == "closed" || status == "review" {
		return xnode.ErrTrafficPeriodSettled
	}
	if owner != int64(sample.UserID) {
		var migrated bool
		if err := tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM member_identity_aliases a JOIN xnode_period_nodes p ON p.period_id=$1 AND p.node_id=$2 WHERE a.old_proxy_id=$3 AND a.proxy_id=$4)`, sample.AccountingPeriod, node, sample.UserID, owner).Scan(&migrated); err != nil {
			return err
		}
		if migrated {
			sample.UserID = int(owner)
		} else if err := attributeProbeSample(ctx, tx, node, owner, &sample); err != nil {
			return err
		}
	}
	if sample.UploadBytes == 0 && sample.DownloadBytes == 0 {
		return nil
	}
	date := sample.Through.UTC().Format(time.DateOnly)
	key := nativeUsageKey{int64(sample.UserID), node, sample.AccountingPeriod, date}
	old, err := nativeUsageTx(ctx, tx, buffer, key)
	if err != nil {
		return err
	}
	next, weighted, err := nextNativeUsage(old, int64(sample.UploadBytes), int64(sample.DownloadBytes), multiplier)
	if err != nil {
		return err
	}
	if !period.payg {
		if err = recordComboUsageTx(ctx, tx, node, sample, period, history); err != nil {
			return err
		}
	}
	if buffer != nil {
		return buffer.stage(key, next, weighted)
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO xnode_daily_usage(proxy_user_id,node_id,period_id,date,upload_bytes,download_bytes,weighted_bytes,weight_remainder) VALUES($1,$2,$3,$4,$5,$6,$7,$8) ON CONFLICT(proxy_user_id,node_id,period_id,date) DO UPDATE SET upload_bytes=excluded.upload_bytes,download_bytes=excluded.download_bytes,weighted_bytes=excluded.weighted_bytes,weight_remainder=excluded.weight_remainder`, key.user, key.node, key.period, key.date, next.upload, next.download, next.weighted, next.remainder)
	if err != nil {
		return err
	}
	result, err := tx.ExecContext(ctx, `UPDATE xnode_periods SET used_bytes=used_bytes+$1 WHERE id=$2 AND used_bytes<=$3`, weighted, sample.AccountingPeriod, math.MaxInt64-weighted)
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
	if period.payg {
		var user string
		if err := tx.QueryRowContext(ctx, `SELECT user_id FROM xnode_payg_accounts WHERE period_id=$1 AND proxy_user_id=$2 AND node_id=$3`, sample.AccountingPeriod, sample.UserID, node).Scan(&user); err != nil {
			return err
		}
		return recordPAYGNodeSampleTx(ctx, tx, user, node, sample, history)
	}
	return nil
}
