package database

import (
	"context"
	"database/sql"
	"errors"
	"math"
	"math/big"
	"strconv"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
	wire "github.com/VanSail-Opensource/X-Node/pkg/protocol"
)

// RecordXNodeTraffic commits one contiguous immutable journal sequence before ACK.
func (s *Store) RecordXNodeTraffic(ctx context.Context, nodeID, agent string, sequence uint64, digest string, report wire.TrafficReport) error {
	if sequence == 0 || sequence > math.MaxInt64 || report.ID != agent+":"+strconv.FormatUint(sequence, 10) {
		return xnode.ErrInvalid
	}
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var bound string
	var multiplier int64
	if err = tx.QueryRowContext(ctx, `SELECT COALESCE(agent_id,''),multiplier_fp FROM xnode_nodes WHERE id=?`, nodeID).Scan(&bound, &multiplier); err != nil {
		return err
	}
	if bound != agent {
		return xnode.ErrConflict
	}
	var previous uint64
	var previousDigest string
	err = tx.QueryRowContext(ctx, `SELECT last_sequence,last_digest FROM xnode_report_cursors WHERE agent_id=?`, agent).Scan(&previous, &previousDigest)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	if sequence <= previous {
		if sequence == previous && digest != previousDigest {
			return xnode.ErrConflict
		}
		return tx.Commit()
	}
	if sequence != previous+1 {
		return xnode.ErrConflict
	}
	var priced int64
	err = tx.QueryRowContext(ctx, `SELECT multiplier_fp FROM xnode_node_rates WHERE agent_id=? AND from_sequence<=? ORDER BY from_sequence DESC LIMIT 1`, agent, sequence).Scan(&priced)
	if err == nil {
		multiplier = priced
	} else if !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	sampled := map[int]bool{}
	for _, sample := range report.Samples {
		sampled[sample.UserID] = true
		if err = recordXNodeSample(ctx, tx, nodeID, multiplier, sample); err != nil {
			return err
		}
	}
	for user, bytes := range report.Traffic {
		if sampled[user] || bytes == [2]uint64{} {
			continue
		}
		var service sql.NullString
		var metered bool
		if err = tx.QueryRowContext(ctx, `SELECT service_name,EXISTS(SELECT 1 FROM xnode_metered_probes p WHERE p.proxy_user_id=i.proxy_user_id) FROM xnode_identities i WHERE proxy_user_id=?`, user).Scan(&service, &metered); err != nil {
			return err
		}
		if !service.Valid || metered {
			return xnode.ErrConflict
		}
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO xnode_report_cursors(agent_id,node_id,last_sequence,last_digest,updated_at) VALUES(?,?,?,?,?) ON CONFLICT(agent_id) DO UPDATE SET last_sequence=excluded.last_sequence,last_digest=excluded.last_digest,updated_at=excluded.updated_at`, agent, nodeID, sequence, digest, stamp(time.Now().UTC()))
	if err != nil {
		return err
	}
	return tx.Commit()
}

func recordXNodeSample(ctx context.Context, tx *sql.Tx, node string, multiplier int64, sample wire.TrafficSample) error {
	if sample.UserID <= 0 || !wire.ValidName(sample.AccountingPeriod) || sample.From.IsZero() || sample.Through.Before(sample.From) || sample.UploadBytes > math.MaxInt64 || sample.DownloadBytes > math.MaxInt64 || sample.UploadBytes > math.MaxInt64-sample.DownloadBytes {
		return xnode.ErrInvalid
	}
	var owner int64
	var status string
	if err := tx.QueryRowContext(ctx, `SELECT proxy_user_id,status FROM xnode_periods WHERE id=?`, sample.AccountingPeriod).Scan(&owner, &status); err != nil {
		return err
	}
	if status == "closed" || status == "review" {
		return xnode.ErrConflict
	}
	if owner != int64(sample.UserID) {
		if err := attributeProbeSample(ctx, tx, node, owner, &sample); err != nil {
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
