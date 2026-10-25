package database

import (
	"context"
	"database/sql"
	"errors"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
	wire "github.com/VanSail-Opensource/X-Node/pkg/protocol"
	"math"
	"math/big"
	"time"
)

func recordXNodeSample(ctx context.Context, tx *sql.Tx, node string, multiplier int64, sample wire.TrafficSample) error {
	if sample.UserID <= 0 || !wire.ValidName(sample.AccountingPeriod) || sample.From.IsZero() || sample.Through.Before(sample.From) || sample.UploadBytes > math.MaxInt64 || sample.DownloadBytes > math.MaxInt64 || sample.UploadBytes > math.MaxInt64-sample.DownloadBytes {
		return xnode.ErrInvalid
	}
	var owner int64
	var status string
	if err := tx.QueryRowContext(ctx, `SELECT proxy_user_id,status FROM xnode_periods WHERE id=?`, sample.AccountingPeriod).Scan(&owner, &status); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return xnode.ErrTrafficPeriodMissing
		}
		return err
	}
	if status == "closed" || status == "review" {
		return xnode.ErrTrafficPeriodSettled
	}
	if owner != int64(sample.UserID) {
		var migrated bool
		if err := tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM member_identity_aliases a JOIN xnode_period_nodes p ON p.period_id=? AND p.node_id=? WHERE a.old_proxy_id=? AND a.proxy_id=?)`, sample.AccountingPeriod, node, sample.UserID, owner).Scan(&migrated); err != nil {
			return err
		}
		if migrated {
			sample.UserID = int(owner)
		} else if err := attributeProbeSample(ctx, tx, node, owner, &sample); err != nil {
			return err
		}
	}
	date := sample.Through.UTC().Format(time.DateOnly)
	var oldUpload, oldDownload, oldWeighted, remainder int64
	err := tx.QueryRowContext(ctx, `SELECT upload_bytes,download_bytes,weighted_bytes,weight_remainder FROM xnode_daily_usage WHERE proxy_user_id=? AND node_id=? AND period_id=? AND date=?`, sample.UserID, node, sample.AccountingPeriod, date).Scan(&oldUpload, &oldDownload, &oldWeighted, &remainder)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	upload, download := int64(sample.UploadBytes), int64(sample.DownloadBytes)
	if oldUpload > math.MaxInt64-upload || oldDownload > math.MaxInt64-download {
		return xnode.ErrInvalid
	}
	numerator := new(big.Int).Mul(big.NewInt(upload+download), big.NewInt(multiplier))
	numerator.Add(numerator, big.NewInt(remainder))
	weighted, left := new(big.Int), new(big.Int)
	weighted.QuoRem(numerator, big.NewInt(xnode.MultiplierScale), left)
	if !weighted.IsInt64() || oldWeighted > math.MaxInt64-weighted.Int64() {
		return xnode.ErrInvalid
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO xnode_daily_usage(proxy_user_id,node_id,period_id,date,upload_bytes,download_bytes,weighted_bytes,weight_remainder) VALUES(?,?,?,?,?,?,?,?) ON CONFLICT(proxy_user_id,node_id,period_id,date) DO UPDATE SET upload_bytes=excluded.upload_bytes,download_bytes=excluded.download_bytes,weighted_bytes=excluded.weighted_bytes,weight_remainder=excluded.weight_remainder`, sample.UserID, node, sample.AccountingPeriod, date, oldUpload+upload, oldDownload+download, oldWeighted+weighted.Int64(), left.Int64())
	if err != nil {
		return err
	}
	result, err := tx.ExecContext(ctx, `UPDATE xnode_periods SET used_bytes=used_bytes+? WHERE id=? AND used_bytes<=?`, weighted.Int64(), sample.AccountingPeriod, math.MaxInt64-weighted.Int64())
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
	return nil
}
